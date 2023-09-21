package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mikhailche/telebot"
)

type UpdateProcessor interface {
	ProcessUpdateCtx(context.Context, telebot.Update) error
}

type UpdateLogGetter interface {
	GetByUpdateId(context.Context, uint64) (*telebot.Update, error)
}

func ReplayUpdateController(mux botMux, adminAuth telebot.MiddlewareFunc, getter UpdateLogGetter, processor UpdateProcessor) {
	mux.Handle("/replayupdate", func(ctx context.Context, c telebot.Context) error {
		if len(c.Args()) == 0 {
			return c.EditOrReply(ctx, "Укажи ID обновления аргументом к команде")
		}
		updateID, err := strconv.Atoi(c.Args()[0])
		if err != nil {
			return c.EditOrReply(ctx, fmt.Sprintf("ID должен быть числовой: %v", err))
		}
		update, err := getter.GetByUpdateId(ctx, uint64(updateID))
		if err != nil {
			return c.EditOrReply(ctx, fmt.Sprintf("Не удалось получить обновление из базы: %v", err))
		}
		processor.ProcessUpdateCtx(ctx, *update)
		return c.EditOrReply(ctx, "Наверное, всё удалось, но я точно не знаю")
	}, adminAuth)
}
