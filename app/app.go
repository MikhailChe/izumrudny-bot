package app

import (
	"context"
	"mikhailche/botcomod/handlers/middleware"
	"mikhailche/botcomod/handlers/middleware/ydbctx"
	"mikhailche/botcomod/lib/tracer.v2"
	"sync"

	"github.com/mikhailche/telebot"

	"mikhailche/botcomod/bot"
	"mikhailche/botcomod/logger"
	"mikhailche/botcomod/repository"
	ydbrepodriver "mikhailche/botcomod/repository/ydb"
	"mikhailche/botcomod/services"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"go.uber.org/zap"
)

type App struct {
	db           *ydb.Driver
	Bot          *bot.TBot
	Log          *zap.Logger
	UpdateLogger *repository.UpdateLogger
}

var theApp *App
var theAppMutex sync.Mutex

func APP(ctx context.Context) *App {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	theAppMutex.Lock()
	defer theAppMutex.Unlock()
	if theApp == nil {
		theApp = newApp(ctx)
	}
	return theApp
}

func newApp(ctx context.Context) *App {
	ctx, span := tracer.Open(tracer.Background(ctx))
	defer span.Close()

	log, err := logger.New(ctx)
	if err != nil {
		panic(err)
	}
	log.Info("Инициализируем новое приложение. Вот и логгер уже готов.")
	ydbDriver, err := ydbrepodriver.NewYDBDriver(ctx, log)
	if err != nil {
		log.Fatal("Ошибка инициализации YDB в приложении", zap.Error(err))
	}
	userRepository, err := repository.NewUserRepository(ctx, ydbDriver, log)
	if err != nil {
		log.Fatal("Ошибка инициализации пользовательского репозитория", zap.Error(err))
	}

	housesRepository := repository.NewHouseRepository(ydbDriver, log)
	houseService := services.NewHouseService(ctx, housesRepository)

	groupChatRepository := repository.NewGroupChatRepository(ydbDriver, log.Named("groupChatRepository"))
	groupChatService := services.NewGroupChatService(ctx, groupChatRepository)

	telegramChatUpserter := repository.UpsertTelegramChat(ctx, ydbDriver)

	updateLogRepository := repository.NewUpdateLogger(ydbDriver, log.Named("updateLogger"))

	tBot, err := bot.NewBot(
		ctx,
		log,
		userRepository,
		houseService.Houses,
		groupChatService,
		updateLogRepository,
		repository.SelectTelegramChatsByUserID(ydbDriver),
		[]telebot.MiddlewareFunc{
			middleware.TracingMiddleware,
			ydbctx.WithYdbTxInContext(ydbDriver, log.Named("ydbSessionMiddleware")),
			middleware.UpsertUsernameMiddleware(
				log.Named("upsertUsernameMiddleware"),
				userRepository, telegramChatUpserter,
				repository.UpsertTelegramChatToUserMapping(ydbDriver),
			),
			middleware.AutoRespondCallback,
			middleware.CurrentUserInContext(userRepository),
			middleware.RecoverMiddleware(log.Named("recoverMiddleware")),
		},
	)
	if err != nil {
		log.Fatal("Ошибка инициализации бота", zap.Error(err))
	}
	return &App{
		db:           ydbDriver,
		Bot:          tBot,
		Log:          log,
		UpdateLogger: updateLogRepository,
	}
}
