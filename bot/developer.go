package bot

import (
	"context"
	"fmt"

	"mikhailche/botcomod/tracer"

	tele "github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

const developerID = 257582730

func forwardToDeveloper(log *zap.Logger) func(context.Context, tele.Context) error {
	return func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("forwardToDeveloper")()
		if c.Chat().Type != tele.ChatPrivate {
			return nil
		}
		if err := doForwardToDeveloper(ctx, c); err != nil {
			log.Error("Не могу передать разработчику")
			return c.Reply("Не получилось передать сообщение разработчику. Давайте попробуем позже.")
		}
		return c.Reply("Спасибо. Передал разработчику.")
	}
}

func doForwardToDeveloper(ctx context.Context, c tele.Context) error {
	defer tracer.Trace("doForwardToDeveloper")()
	var chat = c.Chat()
	if _, err := c.Bot().Send(
		&tele.Chat{ID: developerID},
		fmt.Sprintf("Сообщение от клиента [%v]: %v %v @%v", chat.ID, chat.FirstName, chat.LastName, chat.Username),
	); err != nil {
		return fmt.Errorf("форвардинг разработчику: %w", err)
	}
	return c.ForwardTo(&tele.User{ID: developerID})
}

func sendToDeveloper(ctx tele.Context, log *zap.Logger, message string, opts ...any) error {
	defer tracer.Trace("sendToDeveloper")()
	log.Named("сообщения для разработчиков").Info(message, zap.Any("opts", opts))
	if _, err := ctx.Bot().Send(&tele.Chat{ID: developerID}, message, opts...); err != nil {
		return fmt.Errorf("сообщение разработчику %v: %w", message, err)
	}
	return nil
}
