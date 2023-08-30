package handlers

import (
	markup "mikhailche/botcomod/lib/bot-markup"

	"gopkg.in/telebot.v3"
)

func ChatGroupAdminController(mux botMux) {
	mux.Handle(&markup.ChatGroupAdminBtn, func(ctx telebot.Context) error {
		return ctx.EditOrReply(
			`Тут пока ничего нет. 
			Если вы увидели этот раздел, значит бот знает, что вы админ одного из чатов. 
			Скоро тут будут полезности.`,
			markup.InlineMarkup(markup.Row(markup.HelpMainMenuBtn)),
		)
	})
}
