package handlers

import tele "gopkg.in/telebot.v3"

type botMux interface {
	Handle(endpoint interface{}, h tele.HandlerFunc, m ...tele.MiddlewareFunc)
	Use(middleware ...tele.MiddlewareFunc)
}
