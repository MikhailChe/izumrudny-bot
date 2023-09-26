package middleware

import (
	"context"
	"fmt"
	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
	"mikhailche/botcomod/lib/devbotsender"
	"mikhailche/botcomod/lib/tracer.v2"
)

func RecoverMiddleware(log *zap.Logger) func(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			defer func() {
				ctx, span := tracer.Open(ctx, tracer.Named("RecoverMiddleware::defer"))
				defer span.Close()
				if r := recover(); r != nil {
					log.WithOptions(zap.AddCallerSkip(3)).Error("Паника", zap.Any("panicObj", r))
					_ = devbotsender.SendToDeveloper(ctx, c, log, fmt.Sprintf("Паника\n\n%v\n\n%#v", r, r))
				}
			}()
			return hf(ctx, c)
		}
	}
}
