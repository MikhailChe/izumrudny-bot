package handlers

import (
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

func HelpMenuController(mux botMux) {
	helpHandler := func(ctx tele.Context) error {
		defer tracer.Trace("helpHandler")()
		return ctx.EditOrSend(
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			markup.HelpMenuMarkup(),
		)
	}
	mux.Handle("/help", helpHandler)
	mux.Handle(&markup.HelpMainMenuBtn, helpHandler)
}
