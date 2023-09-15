package handlers

import (
	"context"
	markup "mikhailche/botcomod/lib/bot-markup"

	"github.com/mikhailche/telebot"
)

func ChatGroupAdminController(mux botMux) {
	mux.Handle(&markup.ChatGroupAdminBtn, func(ctx context.Context, c telebot.Context) error {
		return c.EditOrReply(
			`Тут пока ничего нет. 
			Если вы увидели этот раздел, значит бот знает, что вы админ одного из чатов. 
			Скоро тут будут полезности.`,
			markup.InlineMarkup(markup.Row(markup.HelpMainMenuBtn)),
		)
	})
}
