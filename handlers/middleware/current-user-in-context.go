package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/repository"
)

func CurrentUserInContext(users *repository.UserRepository) telebot.MiddlewareFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			mwctx, span := tracer.Open(ctx)
			sess := YdbSessionFromContext(mwctx)
			if sess != nil {
				user, err := users.GetUser(mwctx, users.ByID(c.Sender().ID))
				if err == nil {
					ctx = repository.PutCurrentUserToContext(ctx, user)
				}
			}
			span.Close()
			return hf(ctx, c)
		}
	}
}
