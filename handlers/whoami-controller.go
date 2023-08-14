package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"mikhailche/botcomod/repositories"
	"mikhailche/botcomod/tracer"
	"strconv"

	tele "gopkg.in/telebot.v3"
)

type iUserRepository interface {
	GetById(ctx context.Context, userID int64) (*repositories.User, error)
	IsResident(ctx context.Context, userID int64) (bool, error)
}

func WhoAmIController(mux botMux, userRepository iUserRepository) {
	mux.Handle("/whoami", func(ctx tele.Context) error {
		defer tracer.Trace("/whoami")()
		userID := ctx.Sender().ID
		if len(ctx.Args()) > 0 && len(ctx.Args()[0]) > 0 {
			parsedUserID, err := strconv.Atoi(ctx.Args()[0])
			if err == nil {
				userID = int64(parsedUserID)
			}
		}
		user, err := userRepository.GetById(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("не могу достать пользователя: %w", err)
		}
		userRepository.IsResident(context.Background(), userID)
		userAsJson, _ := json.MarshalIndent(*user, "", "  ")
		eventsAsJson, _ := json.MarshalIndent(user.Events, "", "  ")
		return ctx.EditOrReply(fmt.Sprintf("%#v\n\n%v\n\n%v", *user, string(userAsJson), string(eventsAsJson)))
	})
}
