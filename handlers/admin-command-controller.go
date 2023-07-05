package handlers

import (
	"fmt"

	. "mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

func AdminCommandController(mux botMux, adminAuth tele.MiddlewareFunc) {
	mux.Use(adminAuth)
	mux.Handle("/chatidlink", func(ctx tele.Context) error {
		defer Trace("/chatidlink")()
		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(markup.URL("Общаться", fmt.Sprintf("tg://user?id=%s", ctx.Args()[0]))))
		return ctx.Reply("Ссылка на чат", markup)
	})
}
