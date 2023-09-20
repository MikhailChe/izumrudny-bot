package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"mikhailche/botcomod/lib/tracer.v2"
)

func AutoRespondCallback(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(ctx context.Context, c telebot.Context) error {
		err := hf(ctx, c)
		ctx, span := tracer.Open(ctx, tracer.Named("AutoRespondCallbackMiddleware"))
		defer span.Close()
		if c.Callback() != nil {
			_ = c.Respond(ctx, &telebot.CallbackResponse{})
		}
		return err
	}
}
