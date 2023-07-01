package main

import (
	"fmt"

	. "mikhailche/botcomod/tracer"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

const developerID = 257582730

func forwardToDeveloper(log *zap.Logger) func(ctx tele.Context) error {
	return func(ctx tele.Context) error {
		defer Trace("forwardToDeveloper")()
		if ctx.Chat().Type != tele.ChatPrivate {
			return nil
		}
		if err := doForwardToDeveloper(ctx); err != nil {
			log.Error("Не могу передать разработчику")
			return ctx.Reply("Не получилось передать сообщение разработчику. Давайте попробуем позже.")
		}
		return ctx.Reply("Спасибо. Передал разработчику.")
	}
}

func doForwardToDeveloper(ctx tele.Context) error {
	defer Trace("doForwardToDeveloper")()
	var chat = ctx.Chat()
	if _, err := ctx.Bot().Send(&tele.Chat{ID: developerID}, fmt.Sprintf("Сообщение от клиента [%v]: %v %v @%v", chat.ID, chat.FirstName, chat.LastName, chat.Username)); err != nil {
		return fmt.Errorf("форвардинг разработчику: %w", err)
	}
	return ctx.ForwardTo(&tele.User{ID: developerID})
}

func sendToDeveloper(ctx tele.Context, log *zap.Logger, message string, opts ...any) error {
	defer Trace("sendToDeveloper")()
	log.Named("сообщения для разработчиков").Info(message, zap.Any("opts", opts))
	if _, err := ctx.Bot().Send(&tele.Chat{ID: developerID}, message, opts...); err != nil {
		return fmt.Errorf("сообщение разработчику %v: %w", message, err)
	}
	return nil
}
