package handlers

import (
	"fmt"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

func CarsController(mux botMux, adminAuth tele.MiddlewareFunc) {
	mux.Use(adminAuth)
	mux.Handle("/chatidlink", func(ctx tele.Context) error {
		defer tracer.Trace("/chatidlink")()
		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(markup.URL("Общаться", fmt.Sprintf("tg://user?id=%s", ctx.Args()[0]))))
		return ctx.Reply("Ссылка на чат", markup)
	})
}
