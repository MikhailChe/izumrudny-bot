package bot

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"mikhailche/botcomod/handlers"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/http"
	"mikhailche/botcomod/repositories"
	"mikhailche/botcomod/tracer"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type TBot struct {
	Bot *tele.Bot
}

func NewBot(log *zap.Logger,
	userRepository botUserRepository,
	houses func() repositories.THouses,
	groupChats func() repositories.TGroupChats,
	updateLogRepository *repositories.UpdateLogger,
) (*TBot, error) {
	var b TBot
	rand.Seed(time.Now().UnixMicro())
	b.Init(log, userRepository, houses, groupChats, updateLogRepository)
	return &b, nil
}

type botUserRepository interface {
	IsAdmin(ctx context.Context, userID int64) bool
	UpsertUsername(ctx context.Context, userID int64, username string)
	IsResident(ctx context.Context, userID int64) bool
	GetById(ctx context.Context, userID int64) (*repositories.User, error)
	StartRegistration(ctx context.Context, userID int64, UpdateID int64, houseNumber string, apartment string) (approveCode string, err error)
	ConfirmRegistration(ctx context.Context, userID int64, event repositories.ConfirmRegistrationEvent) error
	FailRegistration(ctx context.Context, userID int64, event repositories.FailRegistrationEvent) error

	RegisterCarLicensePlate(ctx context.Context, userID int64, event repositories.RegisterCarLicensePlateEvent) error

	FindByAppartment(ctx context.Context, house string, appartment string) (*repositories.User, error)
}

func (b *TBot) Init(
	log *zap.Logger,
	userRepository botUserRepository,
	houses func() repositories.THouses,
	groupChats func() repositories.TGroupChats,
	updateLogRepository *repositories.UpdateLogger,
) {
	defer tracer.Trace("botInit")()
	var err error
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	pref := tele.Settings{
		Token:       telegramToken,
		Synchronous: true,
		Verbose:     false,
		Offline:     true,
		OnError: func(err error, c tele.Context) {
			defer tracer.Trace("Telebot::OnError")()
			if c != nil {
				log.Error("Ошибка внутри бота",
					zap.Any("update", c.Update()), zap.Error(err),
					zap.Reflect("errorStruct", err), zap.String("errorType", fmt.Sprintf("%T", err)))
			} else {
				log.Error("Ошибка внутри бота", zap.Error(err))
			}
			if _, err := c.Bot().Send(
				&tele.User{ID: developerID},
				fmt.Sprintf("Ошибка обработчика: %v", err.Error()),
			); err != nil {
				log.Error("Не смог логировать в телегу", zap.Error(err))
			}
		},
		Client: http.TracedHttpClient(telegramToken),
	}

	finishTraceNewBot := tracer.Trace("NewBot")
	bot, err := tele.NewBot(pref)
	finishTraceNewBot()
	if err != nil {
		log.Fatal("Cannot start bot", zap.Error(err))
		return
	}
	bot.Me.Username = "IzumrudnyBot" // It is not initialized in offline mode, but is needed for processing command in chat groups
	b.Bot = bot

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("TraceMiddleware")()
			return hf(ctx)
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("RecoverMiddleware")()
			defer func() {
				defer tracer.Trace("RecoverMiddleware::defer")()
				if r := recover(); r != nil {
					log.WithOptions(zap.AddCallerSkip(3)).Error("Паника", zap.Any("panicObj", r))
					sendToDeveloper(ctx, log, fmt.Sprintf("Паника\n\n%v\n\n%#v", r, r))
				}
			}()
			return hf(ctx)
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("UpsertUsername middleware")()
			userRepository.UpsertUsername(context.Background(), ctx.Sender().ID, ctx.Sender().Username)
			return hf(ctx)
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			ctx.Respond(&tele.CallbackResponse{})
			return hf(ctx)
		}
	})

	adminAuthMiddleware := func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("AdminCommandControllerAuth middleware")()
			if userRepository.IsAdmin(context.Background(), ctx.Sender().ID) {
				return hf(ctx)
			}
			return nil
		}
	}

	log.Info("Adding admin command controller")
	handlers.AdminCommandController(bot.Group(), adminAuthMiddleware, bot, userRepository)

	log.Info("Adding replay update controller")
	handlers.ReplayUpdateController(bot.Group(), adminAuthMiddleware, updateLogRepository, bot)

	handlers.StaticDataController(bot.Group())
	log.Info("Adding phones controller")
	handlers.PhonesController(bot.Group(), &markup.HelpMainMenuBtn, &markup.HelpfulPhonesBtn)

	chatsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("chatsHandler")()
		var rows []tele.Row
		var linkGroup []tele.Btn
		dumpMe := func() {
			if len(linkGroup) > 0 {
				rows = append(rows, markup.Row(linkGroup...))
			}
			linkGroup = nil
		}
		inviteLinks := groupChats()
		for i, link := range inviteLinks {
			if i > 0 && (link.Group == "" || link.Group != inviteLinks[i-1].Group) {
				dumpMe()
			}
			// BEFORE POINTER
			// ------------------
			// AFTER POINTER
			if link.Link != "" {
				linkGroup = append(linkGroup, markup.URL(link.Name, link.Link))
			}
			if i >= len(inviteLinks)-1 {
				dumpMe()
			}
		}
		rows = append(rows, markup.Row(markup.HelpMainMenuBtn))
		return ctx.EditOrSend(
			"Вот список известных мне чатов.\n"+
				"Для вступления в большинство из них требуется подтверждение от администратора чата (🔐).",
			markup.InlineMarkup(rows...),
		)
	}
	bot.Handle(&markup.DistrictChatsBtn, chatsHandler)
	bot.Handle("/chats", chatsHandler)

	registrationService := newTelegramRegistrator(log, userRepository, houses, markup.HelpMainMenuBtn)
	registrationService.Register(bot)

	var authMiddleware tele.MiddlewareFunc = func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("AuthMiddleware")()
			if userRepository.IsResident(context.Background(), ctx.Sender().ID) {
				return next(ctx)
			}
			var rows []tele.Row
			rows = append(rows, markup.Row(*registrationService.EntryPoint()))
			rows = append(rows, markup.Row(markup.HelpMainMenuBtn))
			return ctx.EditOrSend(`Этот раздел только для резидентов изумрудного бора. 
Нажмите клавишу регистрации, чтобы получить доступ. Регистрация может занять от нескольких минут до нескольких дней.

После регистрации вы получите доступ к коду от домофона 🔑, ссылкам на видеокамеры, установленные в районе 📽.
А в некоторые соседские чаты вы сможете вступать без дополнительной проверки.

В будущем мы дадим возможность не раскрывая персональных данных общаться с любым резидентом по номеру квартиры или автомобиля на парковке.`,
				markup.InlineMarkup(rows...),
			)
		}
	}

	carsService := NewCarsHandler(userRepository, &markup.HelpMainMenuBtn)
	carsService.Register(bot)

	getResidentsMarkup := func(ctx tele.Context) *tele.ReplyMarkup {
		defer tracer.Trace("getResidentsMarkup")()
		user, err := userRepository.GetById(context.Background(), ctx.Sender().ID)
		if err != nil || user.Registration == nil {
			var rows []tele.Row
			rows = append(rows,
				// residentsMenuMarkup.Row(intercomCodeBtn),
				markup.Row(markup.VideoCamerasBtn),
				markup.Row(markup.PMWithResidentsBtn),
				markup.Row(markup.HelpMainMenuBtn),
			)
			if userRepository.IsAdmin(context.Background(), ctx.Sender().ID) {
				rows = append(rows, markup.Row(carsService.EntryPoint()))
			}
			return markup.InlineMarkup(rows...)
		}

		return markup.InlineMarkup(
			// residentsMenuMarkup.Row(intercomCodeBtn),
			markup.Row(markup.VideoCamerasBtn),
			markup.Row(markup.PMWithResidentsBtn),
			markup.Row(markup.ContinueRegisterBtn),
			markup.Row(markup.HelpMainMenuBtn),
		)
	}

	registrationCheckApproveCode := func(ctx tele.Context, stdctx context.Context, user *repositories.User, approveCode string) error {
		if user.Registration == nil {
			return ctx.EditOrReply("Ошибка регистрации: вы не начинали регистрацию, поэтому не можете её завершить", getResidentsMarkup(ctx))
		}
		if approveCode == user.Registration.Events.Start.ApproveCode {
			userRepository.ConfirmRegistration(
				stdctx,
				ctx.Sender().ID,
				repositories.ConfirmRegistrationEvent{UpdateID: int64(ctx.Update().ID), WithCode: approveCode},
			)
			return ctx.EditOrReply("Спасибо. Регистрация завершена.", getResidentsMarkup(ctx))
		} else {
			userRepository.FailRegistration(
				stdctx,
				ctx.Sender().ID,
				repositories.FailRegistrationEvent{UpdateID: int64(ctx.Update().ID), WithCode: approveCode},
			)
			return ctx.EditOrReply(
				"Неверный код. Попробуем заново? Процесс такой же: выбираете дом и квартиру и ждёте правильный код на почту.",
				markup.HelpMenuMarkup(),
			)
		}
	}

	/*
		handleContinueRegistration := func(ctx tele.Context, stdctx context.Context, user *User) error {
			defer tracer.Trace("handleContinueRegistration")()
			if err != nil {
				return fmt.Errorf("продолжение регистрации: %w", err)
			}
			data := ctx.Args()
			if len(data) == 0 || len(data) == 1 && data[0] == "" {
				var allCodes []string = append(append([]string(nil), user.Registration.Events.Start.ApproveCode), user.Registration.Events.Start.InvalidCodes...)
				rand.Shuffle(len(allCodes), reflect.Swapper(allCodes))
				conRegMarkup := ctx.Bot().NewMarkup()
				var rows []tele.Row
				var buttons []tele.Btn
				for _, code := range allCodes {
					buttons = append(buttons, conRegMarkup.Data(code, registerBtn.Unique, code))
					if len(buttons) >= 2 {
						rows = append(rows, conRegMarkup.Row(buttons...))
						buttons = nil
					}
				}
				if len(buttons) > 0 {
					rows = append(rows, conRegMarkup.Row(buttons...))
					buttons = nil
				}
				rows = append(rows, conRegMarkup.Row(helpMainMenuBtn))
				conRegMarkup.Inline(rows...)
				return ctx.EditOrReply("Для завершения регистрации выберите правильный код, который вы нашли у себя в почтовом ящике.\n"+
					"Если Ваш дом ещё не сдан, то вы можете пользоваться частью сервисов и завершить регистрацию после заселения.", conRegMarkup)
			}
			return registrationCheckApproveCode(ctx, stdctx, user, data[0])
		}
	*/
	handleMaybeRegistration := func(ctx tele.Context, stdctx context.Context, token string) error {
		var approveToken repositories.UserRegistrationApproveToken
		err := DecodeSignedMessage(token, &approveToken)
		if err != nil {
			return err
		}
		if approveToken.UserID != ctx.Sender().ID {
			return ctx.EditOrReply("Этот код регистрации для другого пользователя. Перепутали телефон?", markup.HelpMenuMarkup())
		}

		user, err := userRepository.GetById(stdctx, ctx.Sender().ID)
		if err != nil {
			return err
		}
		return registrationCheckApproveCode(ctx, stdctx, user, approveToken.ApproveCode)
	}

	bot.Handle("/start", func(ctx tele.Context) error {
		defer tracer.Trace("/start")()
		if len(ctx.Args()) == 1 && len(ctx.Args()[0]) > 4 {
			if err := handleMaybeRegistration(ctx, context.Background(), ctx.Args()[0]); err == nil {
				return nil
			} else {
				log.Error("Ошибочная /start регистрация", zap.Error(err))
			}
		}
		return ctx.EditOrReply("Привет! " + handlers.BotDescription + "\nИспользуйте команду /help для вызова меню")
	})

	authGroup := bot.Group()
	authGroup.Use(authMiddleware)

	residentsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("residentsHandler")()
		return ctx.EditOrSend("Немного полезностей для резидентов", getResidentsMarkup(ctx))
	}
	authGroup.Handle(&markup.ResidentsBtn, residentsHandler)

	intercomHandlers := func(ctx tele.Context) error {
		defer tracer.Trace("intercomHandlers")()
		return ctx.EditOrSend(
			"Здесь будет актуальный код для прохода через домофон. Если вы знаете теукщий код - напишите его мне.",
			getResidentsMarkup(ctx),
		)
	}
	authGroup.Handle(&markup.IntercomCodeBtn, intercomHandlers)

	videoCamerasHandler := func(ctx tele.Context) error {
		defer tracer.Trace("videoCamerasHandler")()
		return ctx.EditOrSend(`
<a href="https://vs.domru.ru">Площадка 108А</a>
Логин: <code>ertel-wk-557</code>
Пароль: <code>uu4rg2x3</code>

<a href="https://video.ugmk-telecom.ru">Площадка 108Б</a>
Логин: <code>108b</code>
Пароль: <code>izumrud20</code>

Для просмотра можно воспользоваться приложением Форпост.
`,
			tele.ModeHTML,
			getResidentsMarkup(ctx))
	}
	authGroup.Handle(&markup.VideoCamerasBtn, videoCamerasHandler)

	residentsChatter, err := NewResidentsChatter(userRepository, houses, markup.HelpMainMenuBtn)
	if err != nil {
		log.Fatal("Ошибка инициализации чатов", zap.Error(err))
	}

	residentsChatter.RegisterBotsHandlers(authGroup)
	pmWithResidentsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("pmWithResidentsHandler")()
		return residentsChatter.HandleChatWithResident(ctx)
	}
	authGroup.Handle("/connect", pmWithResidentsHandler)
	authGroup.Handle(&markup.PMWithResidentsBtn, pmWithResidentsHandler)

	bot.Handle(tele.OnText, forwardToDeveloper(log))
	bot.Handle(tele.OnMedia, func(ctx tele.Context) error {
		stdctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		user, err := userRepository.GetById(stdctx, int64(ctx.Sender().ID))
		if err != nil {
			return fmt.Errorf("tele.OnMedia: %w", err)
		}
		if user.Registration != nil {
			return registrationService.HandleMediaCreated(user, ctx)
		}
		return forwardToDeveloper(log)(ctx)
	})
}
