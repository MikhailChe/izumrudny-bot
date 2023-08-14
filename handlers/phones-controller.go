package handlers

import (
	bm "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

func PhonesController(mux botMux, helpMainMenuBtn *tele.Btn, helpfulPhonesBtn *tele.Btn) {
	phonesHandler := func(ctx tele.Context) error {
		defer tracer.Trace("phonesHandler")()
		markup := bm.Markup()
		markup.Inline(
			markup.Row(*helpMainMenuBtn),
		)
		return ctx.EditOrSend(
			"👮 Охрана  <b>+7-982-690-0793</b>\n"+
				"🚨 Аварийно-диспетчерская служба <b>+7-343-317-0798</b>\n"+
				"🧑‍💼👔 Управляющая компания <b>+7-343-283-0555</b>\n\n"+
				"Если здесь не хватает какого-то важного номера телефона - напишите мне об этом",
			tele.ModeHTML,
			markup)
	}
	mux.Handle(helpfulPhonesBtn, phonesHandler)
	mux.Handle("/phones", phonesHandler)
}