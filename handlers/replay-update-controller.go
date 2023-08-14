package handlers

import (
	"context"
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v3"
)

type UpdateProcessor interface {
	ProcessUpdate(tele.Update)
}

type UpdateLogGetter interface {
	GetByUpdateId(context.Context, uint64) (*tele.Update, error)
}

func ReplayUpdateController(mux botMux, adminAuth tele.MiddlewareFunc, getter UpdateLogGetter, processor UpdateProcessor) {
	mux.Handle("/replayupdate", func(ctx tele.Context) error {
		if len(ctx.Args()) == 0 {
			return ctx.EditOrReply("Укажи ID обновления аргументом к команде")
		}
		updateID, err := strconv.Atoi(ctx.Args()[0])
		if err != nil {
			return ctx.EditOrReply(fmt.Sprintf("ID должен быть числовой: %v", err))
		}
		update, err := getter.GetByUpdateId(context.Background(), uint64(updateID))
		if err != nil {
			return ctx.EditOrReply(fmt.Sprintf("Не удалось получить обновление из базы: %v", err))
		}
		processor.ProcessUpdate(*update)
		return ctx.EditOrReply("Наверное, всё удалось, но я точно не знаю")
	}, adminAuth)
}
