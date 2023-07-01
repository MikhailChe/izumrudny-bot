package main

import (
	"context"
	"encoding/json"

	. "mikhailche/botcomod/tracer"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type LambdaResponse struct {
	StatusCode int         `json:"statusCode"`
	Body       interface{} `json:"body"`
}

type LambdaRequest struct {
	Body       string         `json:"body"`
	Headers    map[string]any `json:"headers"`
	HTTPMethod string         `json:"httpMethod"`
}

func Handler(ctx context.Context, body []byte) (*LambdaResponse, error) {
	defer Trace("Handler")()
	app := APP()
	defer func() {
		if r := recover(); r != nil {
			app.log.WithOptions(zap.AddCallerSkip(3)).Error("Паника в верхнем уровне", zap.Any("panicObj", r))
		}
	}()
	var request LambdaRequest
	if err := json.Unmarshal(body, &request); err != nil {
		app.log.Error("Не получилось распарсить запрос", zap.Error(err))
	}
	var updateMap map[string]any
	_ = json.Unmarshal([]byte(request.Body), &updateMap)
	app.updateLogger.logUpdate(ctx, updateMap, request.Body)

	var update tele.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		app.log.Error("Запросец", zap.Error(err))
	}
	if app.bot == nil {
		panic("nil app.bot")
	}
	if app.bot.bot == nil {
		panic("nil app.bot.bot")
	}
	app.bot.bot.ProcessUpdate(update)
	return &LambdaResponse{
		StatusCode: 200,
		Body:       "OK",
	}, nil
}

// TODO: добавить сервис показания счётчиков
// TODO: информация про тихий час
// TODO: информация про правила поведения с собаками
