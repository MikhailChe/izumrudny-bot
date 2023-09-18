package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/repository"
)

func UpsertUsernameMiddleware(
	log *zap.Logger,
	userRepository *repository.UserRepository,
	telegramChatUpserter func(ctx context.Context, chat telebot.Chat) error,
	chatToUserUpserter func(ctx context.Context, chat, user int64) error,
) func(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			ctx, span := tracer.Open(ctx, tracer.Named("UpsertUsername middleware"))
			defer span.Close()
			err := hf(ctx, c)
			userRepository.UpsertUsername(context.Background(), c.Sender().ID, c.Sender().Username)
			if err := telegramChatUpserter(context.Background(), *c.Chat()); err != nil {
				log.Error("telegramChatUpserter middleware failed", zap.Error(err))
			}
			if err := chatToUserUpserter(context.Background(), c.Chat().ID, c.Sender().ID); err != nil {
				log.Error("chatToUserUpserter middleware failed", zap.Error(err))
			}
			return err
		}
	}
}
