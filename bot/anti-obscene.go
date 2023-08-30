package bot

import (
	"mikhailche/botcomod/services"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type obsceneDetector interface {
	DetectObsceneLanguage(string) bool
}

func manageAntiSpam(log *zap.Logger, groupChatService *services.GroupChatService, antiObsceneFilter obsceneDetector) func(ctx tele.Context) error {
	return func(ctx tele.Context) error {
		if !groupChatService.IsAntiObsceneEnabled(ctx.Chat().ID) {
			return nil
		}
		text := ctx.Text()
		if len(text) == 0 {
			return nil
		}
		if antiObsceneFilter.DetectObsceneLanguage(text) {
			log.Warn("Обсценная лексика Удоли")
			return ctx.Delete()
		}
		return nil
	}
}
