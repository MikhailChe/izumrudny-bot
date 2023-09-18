package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"go.uber.org/zap"
	"mikhailche/botcomod/lib/tracer.v2"
)

type ydbSessionInCtxType int

var ydbSessionInCtx ydbSessionInCtxType

func YdbSessionFromContext(ctx context.Context) table.Session {
	if sess, ok := ctx.Value(ydbSessionInCtx).(table.Session); ok {
		return sess
	}
	return nil
}

type ydbDriver interface {
	Table() table.Client
}

func WithYdbTxInContext(db ydbDriver, log *zap.Logger) telebot.MiddlewareFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			ctx, span := tracer.Open(ctx, tracer.Named("GlobalHandlerTx"))
			defer span.Close()
			log.Debug("Running Ydb transaction context handler")
			defer log.Debug("Finished Ydb transaction context handler")
			return db.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
				ctx, span := tracer.Open(ctx, tracer.Named("GlobalHandlerTx::Do"))
				defer span.Close()
				log.Debug("Inside Ydb transaction")
				defer log.Debug("Outside Ydb transaction")
				ctx = context.WithValue(ctx, ydbSessionInCtx, s)
				return hf(ctx, c)
			}, table.WithIdempotent())
		}
	}
}
