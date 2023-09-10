package handlers

import (
	"context"
	"fmt"
	"mikhailche/botcomod/repository"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

// HANDLER TO DETECT A USER
func with[A any](
	ctx context.Context,
	userIdent A,
	fn func(context.Context, A) (*repository.User, error),
	byUser func(context.Context, repository.User) (string, error),
) (string, error) {
	user, err := fn(ctx, userIdent)
	if err != nil {
		return "", err
	}
	return byUser(ctx, *user)
}

func WhoisHandler(
	mux botMux,
	groupChatAdminMiddleware telebot.MiddlewareFunc,
	userByID func(context.Context, int64) (*repository.User, error),
	userByUsername func(context.Context, string) (*repository.User, error),
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	log *zap.Logger,
) {
	mux.Use(groupChatAdminMiddleware)

	byUser := func(ctx context.Context, user repository.User) (string, error) {
		groupIDs, err := userGroupsByUserId(ctx, user.ID)
		if err != nil {
			log.Error("Ошибка получения списка чатов пользователя", zap.Error(err))
		}
		return fmt.Sprintf(
			"Пользователь %#v\nУчаствует в группах %v",
			user, groupIDs,
		), nil

	}

	whois := func(ctx telebot.Context) error {
		args := ctx.Args()
		if len(args) == 0 {
			return ctx.EditOrReply("Введите имя пользователя или его идентификатор")
		}
		stdctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		var message string
		var err error
		userID, err := strconv.Atoi(args[0])
		if err != nil {
			message, err = with(stdctx, args[0], userByUsername, byUser)
		} else {
			message, err = with(stdctx, int64(userID), userByID, byUser)
		}
		if err != nil {
			ctx.EditOrReply("Ошибка получения информации о пользователе")
			log.Error("Ошибка получения информации о пользователе", zap.Error(err))
			return nil
		}
		return ctx.EditOrReply(message)
	}
	mux.Handle("/whois", whois)
}
