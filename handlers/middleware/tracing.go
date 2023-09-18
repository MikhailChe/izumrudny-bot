package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"mikhailche/botcomod/lib/tracer.v2"
)

func TracingMiddleware(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("TraceMiddleware"))
		defer span.Close()
		return hf(ctx, c)
	}
}
