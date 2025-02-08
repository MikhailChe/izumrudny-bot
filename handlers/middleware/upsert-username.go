package middleware

import (
	"context"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/repository"

	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

func UpsertUsernameMiddleware(
	log *zap.Logger,
	userRepository *repository.UserRepository,
	telegramChatUpserter func(ctx context.Context, chat telebot.Chat) error,
	chatToUserUpserter func(ctx context.Context, chat, user int64) error,
) func(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			func() {
				ctx, span := tracer.Open(ctx, tracer.Named("UpsertUsername middleware"))
				defer span.Close()
				userRepository.UpsertUsername(ctx, c.Sender().ID, c.Sender().Username)
				if err := telegramChatUpserter(ctx, *c.Chat()); err != nil {
					log.Error("telegramChatUpserter middleware failed", zap.Error(err))
				}
				if err := chatToUserUpserter(ctx, c.Chat().ID, c.Sender().ID); err != nil {
					log.Error("chatToUserUpserter middleware failed", zap.Error(err))
				}
			}()
			err := hf(ctx, c)
			return err
		}
	}
}
