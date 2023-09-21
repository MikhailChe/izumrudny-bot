package handlers

import (
	"context"
	bm "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/tracer.v2"

	"github.com/mikhailche/telebot"
)

func PhonesController(mux botMux, helpMainMenuBtn *telebot.Btn, helpfulPhonesBtn *telebot.Btn) {
	phonesHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("phonesHandler"))
		defer span.Close()
		markup := bm.Markup()
		markup.Inline(
			markup.Row(*helpMainMenuBtn),
		)
		return c.EditOrSend(ctx,
			"👮 Охрана  <b>+7-982-690-0793</b>\n"+
				"🚨 Аварийно-диспетчерская служба <b>+7-343-317-0798</b>\n"+
				"🧑‍💼👔 Управляющая компания <b>+7-343-283-0555</b>\n\n"+
				"Если здесь не хватает какого-то важного номера телефона - напишите мне об этом",
			telebot.ModeHTML,
			markup)
	}
	mux.Handle(helpfulPhonesBtn, phonesHandler)
	mux.Handle("/phones", phonesHandler)
}
