package app

import (
	"context"
	"sync"

	"mikhailche/botcomod/bot"
	"mikhailche/botcomod/logger"
	"mikhailche/botcomod/repositories"
	ydbd "mikhailche/botcomod/repositories/ydb"
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"go.uber.org/zap"
)

type app struct {
	db           *ydb.Driver
	Bot          *bot.TBot
	Log          *zap.Logger
	UpdateLogger *UpdateLogger
}

var _the_app *app
var _the_app_mutex sync.Mutex

func APP() *app {
	tracer.Trace("APP")()
	_the_app_mutex.Lock()
	defer _the_app_mutex.Unlock()
	if _the_app == nil {
		_the_app = newApp()
	}
	return _the_app
}

func newApp() *app {
	defer tracer.Trace("newApp")()
	ctx := context.Background()

	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	log.Info("Инициализируем новое приложение. Вот и логгер уже готов.")
	ydb, err := ydbd.NewYDBDriver(ctx, log)
	if err != nil {
		log.Fatal("Ошибка инициализации YDB в приложении", zap.Error(err))
	}
	userRepository, err := repositories.NewUserRepository(ydb, log)
	if err != nil {
		log.Fatal("Ошибка инициализации пользовательского репозитория", zap.Error(err))
	}

	housesRepository := repositories.NewHouseRepository(ydb)
	houseService := services.NewHouseService(housesRepository)

	groupChatRepository := repositories.NewGroupChatRepository(ydb)
	groupChatService := services.NewGroupChatService(groupChatRepository)

	updateLogRepository := repositories.NewUpdateLogger(ydb, log)

	bot, err := bot.NewBot(log, userRepository, houseService.Houses, groupChatService.GroupChats, updateLogRepository)
	if err != nil {
		log.Fatal("Ошибка инициализации бота", zap.Error(err))
	}
	return &app{
		db:           ydb,
		Bot:          bot,
		Log:          log,
		UpdateLogger: newUpdateLogger(ydb, log.Named("updateLogger")),
	}
}
