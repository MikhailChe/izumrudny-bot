package ydb

import (
	"context"
	"fmt"
	"github.com/ydb-platform/ydb-go-sdk/v3"
	yc "github.com/ydb-platform/ydb-go-yc"
	"go.uber.org/zap"
	"mikhailche/botcomod/lib/tracer.v2"
	"os"
)

func ydbOpen(ctx context.Context, log *zap.Logger) (*ydb.Driver, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("ydbOpen"))
	defer span.Close()
	defer log.Info("Закончил открывать новое YDB соединение")
	log.Info("Открываю новое YDB соединение")
	var credOption = yc.WithMetadataCredentials()
	if ydbSaKey := os.Getenv("YDB_SA_KEY"); len(ydbSaKey) > 0 {
		log.Info("Нашел YDB ключ в переменных окружения")
		credOption = ydb.WithAccessTokenCredentials(ydbSaKey)
	}
	ydbd, err := ydb.Open(ctx,
		"grpcs://ydb.serverless.yandexcloud.net:2135/ru-central1/b1gekmt72ibh4jkb7edu/etnq0a1lnsfuvp5eulft",
		yc.WithInternalCA(),
		credOption,

		// ydbZap.WithTraces(log, trace.DatabaseSQLEvents|trace.RatelimiterEvents|trace.TableEvents, ydbZap.WithLogQuery()),
	)
	if err != nil {
		return nil, fmt.Errorf("ydbOpen: %w", err)
	}
	return ydbd, nil
}

func NewYDBDriver(ctx context.Context, log *zap.Logger) (*ydb.Driver, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("NewYDBDriver"))
	defer span.Close()
	defer log.Info("Закончил создавать новое YDB соединение")
	log.Info("Создаю новое YDB соединение")
	ydbd, err := ydbOpen(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("newYDBDrive: %w", err)
	}
	return ydbd, nil
}
