package handlers

import (
	"context"
	"fmt"
	"github.com/mikhailche/telebot"
	"mikhailche/botcomod/repository"
)

func ClearAllDataController(userRepo *repository.UserRepository) telebot.HandlerFunc {
	return func(ctx context.Context, c telebot.Context) error {
		if err := userRepo.ClearEvents(ctx, c.Sender().ID); err != nil {
			return c.EditOrReply(fmt.Sprintf("Не получилось удалить данные пользователя %d. %v", c.Sender().ID, err))
		} else {
			return c.EditOrReply(fmt.Sprintf("Получилось удалить данные пользователя %d.", c.Sender().ID))
		}
	}
}
