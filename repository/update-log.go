package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"mikhailche/botcomod/lib/errors"
	"mikhailche/botcomod/lib/tracer.v2"
	"time"

	"github.com/mikhailche/telebot"
	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"go.uber.org/zap"
)

type YDBUpdateLogEntry struct {
	ID     uint64
	Update string
}

func (u *YDBUpdateLogEntry) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("YDBUpdateLogEntry::Scan"))
	defer span.Close()
	return res.ScanNamed(
		named.Required("id", &u.ID),
		named.OptionalWithDefault("update", &u.Update),
	)
}

type UpdateLogger struct {
	db      *ydb.Driver
	log     *zap.Logger
	entries chan YDBUpdateLogEntry
}

func NewUpdateLogger(db *ydb.Driver, logger *zap.Logger) *UpdateLogger {
	upLogger := &UpdateLogger{db: db, log: logger, entries: make(chan YDBUpdateLogEntry, 8)}
	upLogger.runYDBWorker()
	return upLogger
}

func (l *UpdateLogger) LogUpdate(ctx context.Context, upd map[string]any, rawUpdate string) {
	ctx, span := tracer.Open(ctx, tracer.Named("logUpdate"))
	defer span.Close()

	l.log.Info("Обновление от телеги", zap.Any("update", upd))
	select {
	case l.entries <- YDBUpdateLogEntry{(uint64)(upd["update_id"].(float64)), rawUpdate}:
		break
	case <-time.After(time.Second):
		l.log.Error("Не получилось записать обновление в очередь за секунду")
		break
	}
}

type tRetryCount int8

const times tRetryCount = 1

func withRetry(ctx context.Context, f func(context.Context) error, retryCount tRetryCount, retryDelay time.Duration) error {
	ctx, span := tracer.Open(ctx, tracer.Named("withRetry"))
	defer span.Close()
	var allErrors []error
	for ; retryCount > 0; retryCount-- {
		err := f(ctx)
		if err == nil {
			return nil
		}
		allErrors = append(allErrors, err)
	}
	return errors.Join(allErrors...)
}

func (l *UpdateLogger) runYDBWorker() {
	globalCtx := context.Background()
	go func() {
		for entry := range l.entries {
			if err := withRetry(globalCtx, func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(globalCtx, 500*time.Millisecond)
				defer cancel()
				return l.ydbLogUpdateNow(ctx, entry.ID, entry.Update)
			}, 3*times, time.Second); err != nil {
				l.log.Error("Не удалось записать обновление", zap.Error(err))
			}
		}
	}()
}

func (l *UpdateLogger) ydbLogUpdateNow(ctx context.Context, ID uint64, update string) error {
	ctx, span := tracer.Open(ctx, tracer.Named("LogUpdate"))
	defer span.Close()
	return (*l.db).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("Do upsert updates-log"))
		defer span.Close()
		_, res, err := s.Execute(ctx,
			table.DefaultTxControl(),
			"DECLARE $timestamp AS Timestamp; "+
				"DECLARE $id AS Uint64; "+
				"DECLARE $update AS JsonDocument; "+
				"UPSERT INTO `updates-log` "+
				"(timestamp, id, update) "+
				"VALUES ($timestamp, $id, $update);",
			table.NewQueryParameters(
				table.ValueParam("$timestamp", types.TimestampValueFromTime(time.Now())),
				table.ValueParam("$id", types.Uint64Value(ID)),
				table.ValueParam("$update", types.JSONDocumentValue(update)),
			),
		)
		if res != nil {
			_ = res.Close()
		}
		if err != nil {
			return fmt.Errorf("upser update-log %d: %w", ID, err)
		}
		return nil
	}, table.WithIdempotent())
}

func (l *UpdateLogger) GetByUpdateId(ctx context.Context, updateID uint64) (*telebot.Update, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UpdateLogger::GetByUpdateId"))
	defer span.Close()
	var update YDBUpdateLogEntry
	err := (*l.db).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("Do select updates-log"))
		defer span.Close()
		_, res, err := s.Execute(ctx,
			table.DefaultTxControl(),
			"DECLARE $id AS Uint64; "+
				"SELECT id, update FROM `updates-log` WHERE id = $id",
			table.NewQueryParameters(
				table.ValueParam("$id", types.Uint64Value(updateID)),
			),
		)
		if err != nil {
			return fmt.Errorf("select updates-log [%d]: %w", updateID, err)
		}
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел result set для логов обновлений")
		}
		if !res.NextRow() {
			return fmt.Errorf("обновление не найдено")
		}
		if err := update.Scan(ctx, res); err != nil {
			return fmt.Errorf("скан события обновления %v: %w", res, err)
		}
		return res.Err()
	}, table.WithIdempotent())
	if err != nil {
		return nil, err
	}
	var teleUpd telebot.Update
	if err := json.Unmarshal([]byte(update.Update), &teleUpd); err != nil {
		return nil, err
	}
	return &teleUpd, nil
}
