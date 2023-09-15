package handlers

import tele "github.com/mikhailche/telebot"

type botMux interface {
	Handle(endpoint interface{}, h tele.HandlerFunc, m ...tele.MiddlewareFunc)
	Use(middleware ...tele.MiddlewareFunc)
}
