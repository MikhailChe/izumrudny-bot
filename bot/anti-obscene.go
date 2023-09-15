package bot

import (
	"context"
	"mikhailche/botcomod/services"

	tele "github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

type obsceneDetector interface {
	DetectObsceneLanguage(string) bool
}

func manageAntiSpam(log *zap.Logger, groupChatService *services.GroupChatService, antiObsceneFilter obsceneDetector) func(context.Context, tele.Context) error {
	return func(ctx context.Context, c tele.Context) error {
		if !groupChatService.IsAntiObsceneEnabled(c.Chat().ID) {
			return nil
		}
		text := c.Text()
		if len(text) == 0 {
			return nil
		}
		if antiObsceneFilter.DetectObsceneLanguage(text) {
			log.Warn("Обсценная лексика Удоли")
			return c.Delete()
		}
		return nil
	}
}
