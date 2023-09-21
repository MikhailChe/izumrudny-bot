package devbotsender

import (
	"context"
	"fmt"
	"mikhailche/botcomod/lib/tracer.v2"

	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

const DeveloperID = 257582730

func ForwardToDeveloper(log *zap.Logger) func(context.Context, telebot.Context) error {
	return func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("ForwardToDeveloper"))
		defer span.Close()
		if c.Chat().Type != telebot.ChatPrivate {
			return nil
		}
		if err := doForwardToDeveloper(ctx, c); err != nil {
			log.Error("Не могу передать разработчику")
			return c.Reply("Не получилось передать сообщение разработчику. Давайте попробуем позже.")
		}
		return c.Reply("Спасибо. Передал разработчику.")
	}
}

func doForwardToDeveloper(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("doForwardToDeveloper"))
	defer span.Close()
	var sender = c.Sender()
	if _, err := c.Bot().Send(ctx,
		&telebot.Chat{ID: DeveloperID},
		fmt.Sprintf("Сообщение от клиента [%v]: %v %v @%v", sender.ID, sender.FirstName, sender.LastName, sender.Username),
	); err != nil {
		return fmt.Errorf("форвардинг разработчику: %w", err)
	}
	return c.ForwardTo(&telebot.User{ID: DeveloperID})
}

func SendToDeveloper(ctx context.Context, c telebot.Context, log *zap.Logger, message string, opts ...any) error {
	ctx, span := tracer.Open(ctx, tracer.Named("SendToDeveloper"))
	defer span.Close()
	log.Named("сообщения для разработчиков").Info(message, zap.Any("opts", opts))
	if _, err := c.Bot().Send(ctx, &telebot.Chat{ID: DeveloperID}, message, opts...); err != nil {
		return fmt.Errorf("сообщение разработчику %v: %w", message, err)
	}
	return nil
}
