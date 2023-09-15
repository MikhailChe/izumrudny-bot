package main

import (
	"context"
	"encoding/json"

	"mikhailche/botcomod/app"
	"mikhailche/botcomod/tracer"

	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
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

	var update telebot.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		app.Log.Error("Запросец", zap.Error(err))
	}
	if app.Bot == nil {
		panic("nil app.bot")
	}
	if app.Bot.Bot == nil {
		panic("nil app.bot.bot")
	}
	if err := app.Bot.Bot.ProcessUpdateCtx(ctx, update); err != nil {
		app.Log.Error("Error processing update", zap.Error(err))
	}
	return &LambdaResponse{
		StatusCode: 200,
		Body:       "OK",
	}, nil
}

// TODO: добавить сервис показания счётчиков
// TODO: информация про тихий час
// TODO: информация про правила поведения с собаками
// TODO: добавить managed ссылки на чаты
