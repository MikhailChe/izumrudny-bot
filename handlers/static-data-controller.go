package handlers

import (
	"context"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	tele "github.com/mikhailche/telebot"
)

func StaticDataController(mux botMux, groupChats *services.GroupChatService) {
	helpHandler := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("helpHandler")()
		return c.EditOrSend(
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			markup.DynamicHelpMenuMarkup(c, groupChats),
		)
	}
	mux.Handle("/help", helpHandler)
	mux.Handle(&markup.HelpMainMenuBtn, helpHandler)

	mux.Handle("/status", func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("/status")()
		// return c.EditOrSend("🟡 Проводятся технические работы на линии интернета оператора МТС")
		return c.EditOrSend("🟢 Пока нет известных проблем")
	})
}
