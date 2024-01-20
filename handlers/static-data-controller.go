package handlers

import (
	"context"
	"github.com/mikhailche/telebot"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/tracer.v2"
)

func StaticDataController(mux botMux) {
	helpHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("helpHandler"))
		defer span.Close()
		return c.EditOrSend(ctx,
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			markup.DynamicHelpMenuMarkup(ctx),
		)
	}
	mux.Handle("/help", helpHandler)
	mux.Handle(&markup.HelpMainMenuBtn, helpHandler)

	mux.Handle("/status", func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("/status"))
		defer span.Close()
		// return c.EditOrSend(ctx,"🟡 Проводятся технические работы на линии интернета оператора МТС")
		return c.EditOrSend(ctx, "🟢 Пока нет известных проблем")
	})
}
