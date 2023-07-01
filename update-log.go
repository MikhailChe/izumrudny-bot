package main

import (
	"context"
	"fmt"
	"time"

	. "mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"go.uber.org/zap"
)

type YDBUpdateLogEntry struct {
	ID     uint64
	Update string
}

type UpdateLogger struct {
	db      **ydb.Driver
	log     *zap.Logger
	entries chan YDBUpdateLogEntry
}

func newUpdateLogger(db **ydb.Driver, logger *zap.Logger) *UpdateLogger {
	upLogger := &UpdateLogger{db: db, log: logger, entries: make(chan YDBUpdateLogEntry, 8)}
	upLogger.runYDBWorker()
	return upLogger
}

func (l *UpdateLogger) logUpdate(ctx context.Context, upd map[string]any, rawUpdate string) {
	defer Trace("logUpdate")()

	l.log.Info("Обновление от телеги", zap.Any("update", upd))
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
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

func withRetry(f func() error, retryCount tRetryCount, retryDelay time.Duration) error {
	defer Trace("withRetry")()
	var allErrors []error
	for ; retryCount > 0; retryCount-- {
		err := f()
		if err == nil {
			return nil
		}
		allErrors = append(allErrors, err)
	}
	return Join(allErrors...)
}

func (l *UpdateLogger) runYDBWorker() {
	go func() {
		for entry := range l.entries {
			if err := withRetry(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				return l.ydbLogUpdateNow(ctx, entry.ID, entry.Update)
			}, 3*times, time.Second); err != nil {
				l.log.Error("Не удалось записать обновление", zap.Error(err))
			}
		}
	}()
}

func (l *UpdateLogger) ydbLogUpdateNow(ctx context.Context, ID uint64, update string) error {
	defer Trace("LogUpdate")()
	return (*l.db).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		defer Trace("Do upsert updates-log")()
		_, result, err := s.Execute(ctx,
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
		if result != nil {
			result.Close()
		}
		if err != nil {
			return fmt.Errorf("upser update-log %d: %w", ID, err)
		}
		return nil
	}, table.WithIdempotent())
}
