package handlers

import "github.com/mikhailche/telebot"

type botMux interface {
	Handle(endpoint interface{}, h telebot.HandlerFunc, m ...telebot.MiddlewareFunc)
	Use(middleware ...telebot.MiddlewareFunc)
}
