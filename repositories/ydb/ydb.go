package ydb

import (
	"context"
	"fmt"
	"os"
	"time"

	ydbZap "github.com/ydb-platform/ydb-go-sdk-zap"
	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/trace"
	yc "github.com/ydb-platform/ydb-go-yc"
	"go.uber.org/zap"

	. "mikhailche/botcomod/tracer"
)

func ydbOpen(ctx context.Context, log *zap.Logger) (*ydb.Driver, error) {
	defer Trace("ydbOpen")()
	log.Info("Открываю новое YDB соединение")
	var credOption ydb.Option = yc.WithMetadataCredentials()
	if ydb_sa_key := os.Getenv("YDB_SA_KEY"); len(ydb_sa_key) > 0 {
		log.Info("Нашел YDB ключ в переменных окружения")
		credOption = ydb.WithAccessTokenCredentials(ydb_sa_key)
	}
	ydbd, err := ydb.Open(ctx,
		"grpcs://ydb.serverless.yandexcloud.net:2135/ru-central1/b1gekmt72ibh4jkb7edu/etnq0a1lnsfuvp5eulft",
		yc.WithInternalCA(),
		credOption,
		ydbZap.WithTraces(log, trace.DatabaseSQLEvents|trace.RatelimiterEvents|trace.TableEvents, ydbZap.WithLogQuery()),
	)
	if err != nil {
		return nil, fmt.Errorf("ydbOpen: %w", err)
	}
	return ydbd, nil
}

func NewYDBDriverWithPing(ctx context.Context, log *zap.Logger) (**ydb.Driver, error) {
	defer Trace("NewYDBDriver")()
	log.Info("Создаю новое YDB соединение")
	ydbd, err := ydbOpen(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("newYDBDrive: %w", err)
	}
	pinger(&ydbd, log)
	return &ydbd, nil
}

func NewYDBDriver(ctx context.Context, log *zap.Logger) (*ydb.Driver, error) {
	defer Trace("NewYDBDriver")()
	log.Info("Создаю новое YDB соединение")
	ydbd, err := ydbOpen(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("newYDBDrive: %w", err)
	}
	return ydbd, nil
}

func pinger(db **ydb.Driver, log *zap.Logger) {
	ctx := context.Background()
	go func() {
		for range time.Tick(time.Second) {
			pingOnce(ctx, db, log)
		}
	}()
}

func pingOnce(ctx context.Context, dbDoubleRef **ydb.Driver, log *zap.Logger) {
	var err error
	log.Info("YDB ping")
	err = (*dbDoubleRef).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, res, err := s.Execute(ctx, table.DefaultTxControl(), "SELECT 1", table.NewQueryParameters())
		if err != nil {
			return fmt.Errorf("select 1: %w", err)
		}
		res.Close()
		return nil
	}, table.WithIdempotent())
	if err == nil {
		log.Info("YDB pong")
		return
	}
	log.Error("Ошибка пинга YDB. Убиваем приложение", zap.Error(err))
	panic(err)
}
