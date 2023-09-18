package middleware

import (
	"context"
	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/services"
	"time"
)

func UpsertGroupChatMiddleware(log *zap.Logger, groupChats *services.GroupChatService) func(hf telebot.HandlerFunc) telebot.HandlerFunc {
	return func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			ctx, span := tracer.Open(ctx, tracer.Named("UpsertGroupChat middleware"))
			defer span.Close()
			log.Info("Running UpsertGroupChat middleware", zap.String("type", string(c.Chat().Type)))
			if c.Chat().Type != telebot.ChatPrivate {
				ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
				log.Info("Trying to update chat by telegram ID",
					zap.Int64("telegram_chat_id", c.Chat().ID),
					zap.String("telegram_chat_title", c.Chat().Title),
					zap.String("telegram_chat_type", string(c.Chat().Type)),
				)
				if err := groupChats.UpdateChatByTelegramId(ctx, c.Chat().ID, c.Chat().Title, string(c.Chat().Type)); err != nil {
					log.Error("Cannot update chat by telegram ID", zap.Error(err))
				}
			}
			return hf(ctx, c)
		}
	}
}
