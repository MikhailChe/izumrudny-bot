package handlers

import (
	"context"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/services"

	"github.com/mikhailche/telebot"
)

func StaticDataController(mux botMux, groupChats *services.GroupChatService) {
	helpHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("helpHandler"))
		defer span.Close()
		return c.EditOrSend(
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			markup.DynamicHelpMenuMarkup(ctx, c, groupChats),
		)
	}
	mux.Handle("/help", helpHandler)
	mux.Handle(&markup.HelpMainMenuBtn, helpHandler)

	mux.Handle("/status", func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("/status"))
		defer span.Close()
		// return c.EditOrSend("🟡 Проводятся технические работы на линии интернета оператора МТС")
		return c.EditOrSend("🟢 Пока нет известных проблем")
	})
}
