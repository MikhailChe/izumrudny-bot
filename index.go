package main

import (
	"context"
	"encoding/json"
	"mikhailche/botcomod/lib/tracer.v2"

	"mikhailche/botcomod/app"

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
	ctx, span := tracer.Open(ctx, tracer.Named("Handler"))
	appInstance := app.APP(ctx)
	defer func() {
		span.Close()
		if chromeTrace, err := span.PrintTrace(); err == nil {
			appInstance.Log.Debug(string(chromeTrace))
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			appInstance.Log.WithOptions(zap.AddCallerSkip(3)).Error("Паника в верхнем уровне", zap.Any("panicObj", r))
		}
	}()
	var request LambdaRequest
	if err := json.Unmarshal(body, &request); err != nil {
		appInstance.Log.Error("Не получилось распарсить запрос", zap.Error(err))
	}
	var updateMap map[string]any
	_ = json.Unmarshal([]byte(request.Body), &updateMap)
	appInstance.UpdateLogger.LogUpdate(ctx, updateMap, request.Body)

	var update telebot.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		appInstance.Log.Error("Запросец", zap.Error(err))
	}
	if appInstance.Bot == nil {
		panic("nil appInstance.bot")
	}
	if appInstance.Bot.Bot == nil {
		panic("nil appInstance.bot.bot")
	}
	appInstance.Log.Debug("Запускаем процессинг обновления")
	if err := appInstance.Bot.Bot.ProcessUpdateCtx(ctx, update); err != nil {
		appInstance.Log.Error("Error processing update", zap.Error(err), zap.Any("update", update))
	}
	appInstance.Log.Debug("Завершили процессинг обновления")

	return &LambdaResponse{
		StatusCode: 200,
		Body:       "OK",
	}, nil
}

// TODO: добавить сервис показания счётчиков
// TODO: информация про тихий час
// TODO: информация про правила поведения с собаками
// TODO: добавить managed ссылки на чаты
