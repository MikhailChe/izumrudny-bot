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

		if c.Message().IsReply() {
			return handleReply(ctx, c, log)
		}

		if err := doForwardToDeveloper(ctx, c); err != nil {
			log.Error("Не могу передать разработчику", zap.Error(err))
			return c.Reply("Не получилось передать сообщение разработчику. Давайте попробуем позже.")
		}

		return c.Reply("Спасибо. Передал разработчику.")
	}
}

func handleReply(ctx context.Context, c telebot.Context, log *zap.Logger) error {
	if origin := c.Message().ReplyTo; origin != nil {
		forwardedFrom := origin.OriginalSender
		if forwardedFrom == nil {
			log.Error("Не получилось ответить на сообщение: некому отвечать")
			return fmt.Errorf("did not get forwarded origin")
		}
		if _, err := c.Bot().Send(ctx, forwardedFrom, c.Text()); err != nil {
			log.Error("Не получилось ответить на пересланное сообщение", zap.Error(err))
			return err
		}
		return nil
	}

	if err := c.Reply("Не получилось ответить на пересланное сообщение: не нашел оригинал"); err != nil {
		log.Error("Ошибка при ответе на пересланное сообщение", zap.Error(err))
		return err
	}

	return nil
}

func doForwardToDeveloper(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("doForwardToDeveloper"))
	defer span.Close()

	sender := c.Sender()
	message := fmt.Sprintf("Сообщение от клиента [%v]: %v %v @%v", sender.ID, sender.FirstName, sender.LastName, sender.Username)

	if _, err := c.Bot().Send(ctx, &telebot.Chat{ID: DeveloperID}, message); err != nil {
		return fmt.Errorf("форвардинг разработчику: %w", err)
	}

	if err := c.ForwardTo(&telebot.User{ID: DeveloperID}); err != nil {
		return fmt.Errorf("ошибка при пересылке разработчику: %w", err)
	}

	return nil
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
