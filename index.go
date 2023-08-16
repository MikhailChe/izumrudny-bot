package main

import (
	"context"
	"encoding/json"

	"mikhailche/botcomod/app"
	"mikhailche/botcomod/tracer"

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
	defer tracer.Trace("Handler")()
	app := app.APP()
	defer func() {
		if r := recover(); r != nil {
			app.Log.WithOptions(zap.AddCallerSkip(3)).Error("Паника в верхнем уровне", zap.Any("panicObj", r))
		}
	}()
	var request LambdaRequest
	if err := json.Unmarshal(body, &request); err != nil {
		app.Log.Error("Не получилось распарсить запрос", zap.Error(err))
	}
	var updateMap map[string]any
	_ = json.Unmarshal([]byte(request.Body), &updateMap)
	app.UpdateLogger.LogUpdate(ctx, updateMap, request.Body)

	var update tele.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		app.Log.Error("Запросец", zap.Error(err))
	}
	if app.Bot == nil {
		panic("nil app.bot")
	}
	if app.Bot.Bot == nil {
		panic("nil app.bot.bot")
	}
	app.Bot.Bot.ProcessUpdate(update)
	return &LambdaResponse{
		StatusCode: 200,
		Body:       "OK",
	}, nil
}

// TODO: добавить сервис показания счётчиков
// TODO: информация про тихий час
// TODO: информация про правила поведения с собаками
// TODO: добавить managed ссылки на чаты
