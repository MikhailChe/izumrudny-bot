package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
)

func AutoRespondCallback(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(ctx context.Context, c telebot.Context) error {
		err := hf(ctx, c)
		if c.Callback() != nil {
			_ = c.Respond(&telebot.CallbackResponse{})
		}
		return err
	}
}
