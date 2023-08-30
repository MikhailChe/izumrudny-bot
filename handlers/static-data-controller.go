package handlers

import (
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

func StaticDataController(mux botMux, groupChats *services.GroupChatService) {
	helpHandler := func(ctx tele.Context) error {
		defer tracer.Trace("helpHandler")()
		return ctx.EditOrSend(
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			markup.DynamicHelpMenuMarkup(ctx, groupChats),
		)
	}
	mux.Handle("/help", helpHandler)
	mux.Handle(&markup.HelpMainMenuBtn, helpHandler)

	mux.Handle("/status", func(ctx tele.Context) error {
		defer tracer.Trace("/status")()
		// return ctx.EditOrSend("🟡 Проводятся технические работы на линии интернета оператора МТС")
		return ctx.EditOrSend("🟢 Пока нет известных проблем")
	})
}
