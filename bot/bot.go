package bot

import (
	"context"
	"fmt"
	"math/rand"
	"mikhailche/botcomod/lib/devbotsender"
	"mikhailche/botcomod/lib/tracer.v2"
	"os"
	"time"

	"mikhailche/botcomod/handlers"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/http"
	"mikhailche/botcomod/repository"
	"mikhailche/botcomod/services"

	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

type TBot struct {
	Bot *telebot.Bot
}

func NewBot(
	ctx context.Context,
	log *zap.Logger,
	userRepository *repository.UserRepository,
	houses func() repository.THouses,
	groupChats *services.GroupChatService,
	updateLogRepository *repository.UpdateLogger,
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	globalMiddlewares []telebot.MiddlewareFunc,
) (*TBot, error) {
	var b TBot
	rand.Seed(time.Now().UnixMicro())
	b.Init(ctx, log, userRepository, houses, groupChats, updateLogRepository, userGroupsByUserId, globalMiddlewares)
	return &b, nil
}

func (b *TBot) Init(
	ctx context.Context,
	log *zap.Logger,
	userRepository *repository.UserRepository,
	houses func() repository.THouses,
	groupChats *services.GroupChatService,
	updateLogRepository *repository.UpdateLogger,
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	globalMiddlewares []telebot.MiddlewareFunc,
) {
	ctx, span := tracer.Open(ctx, tracer.Named("botInit"))
	defer span.Close()
	var err error
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	pref := telebot.Settings{
		Token:       telegramToken,
		Synchronous: true,
		Verbose:     false,
		Offline:     true,
		OnError: func(err error, c telebot.Context) {
			if c != nil {
				log.Error("Ошибка внутри бота",
					zap.Any("update", c.Update()), zap.Error(err),
					zap.Reflect("errorStruct", err), zap.String("errorType", fmt.Sprintf("%T", err)))
			} else {
				log.Error("Ошибка внутри бота", zap.Error(err))
			}
			if _, err := c.Bot().Send(ctx,
				&telebot.User{ID: devbotsender.DeveloperID},
				fmt.Sprintf("Ошибка обработчика: %v", err.Error()),
			); err != nil {
				log.Error("Не смог логировать в телегу", zap.Error(err))
			}
		},
		Client: http.TracedHttpClient(ctx, log.Named("telegram-http-log"), telegramToken),
	}

	_, telebotNewBotSpan := tracer.Open(ctx, tracer.Named("NewBot"))
	bot, err := telebot.NewBot(pref)
	telebotNewBotSpan.Close()
	if err != nil {
		log.Fatal("Cannot start bot", zap.Error(err))
		return
	}
	bot.Me.Username = "IzumrudnyBot" // It is not initialized in offline mode, but is needed for processing command in chat groups
	b.Bot = bot

	bot.Use(globalMiddlewares...)

	adminAuthMiddleware := func(hf telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			ctx, span := tracer.Open(ctx, tracer.Named("AdminCommandControllerAuth middleware"))
			defer span.Close()
			if userRepository.IsAdmin(ctx, c.Sender().ID) {
				return hf(ctx, c)
			}
			return nil
		}
	}
	groupChatAdminAuthMiddleware := adminAuthMiddleware

	log.Info("Adding admin command controller")
	handlers.AdminCommandController(bot.Group(), adminAuthMiddleware, userRepository, groupChats, houses)

	log.Info("Adding replay update controller")
	handlers.ReplayUpdateController(bot.Group(), adminAuthMiddleware, updateLogRepository, bot)

	handlers.StaticDataController(bot.Group())
	log.Info("Adding phones controller")
	handlers.PhonesController(bot.Group(), &markup.HelpMainMenuBtn, &markup.HelpfulPhonesBtn)

	log.Info("Adding ChatGroupAdmin controller")
	handlers.ChatGroupAdminController(bot.Group())

	log.Info("Adding Whois controller")
	handlers.WhoisHandler(
		bot.Group(),
		groupChatAdminAuthMiddleware,
		func(ctx context.Context, userID int64) (*repository.User, error) {
			return userRepository.GetUser(ctx, userRepository.ByID(userID))
		},
		func(ctx context.Context, username string) (*repository.User, error) {
			return userRepository.GetUser(ctx, userRepository.ByUsername(username))
		},
		userGroupsByUserId,
		log.Named("whoisHandler"),
	)

	chatsHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("chatsHandler"))
		defer span.Close()
		var rows []telebot.Row
		var linkGroup []telebot.Btn
		dumpMe := func() {
			if len(linkGroup) > 0 {
				rows = append(rows, markup.Row(linkGroup...))
			}
			linkGroup = nil
		}
		inviteLinks := groupChats.GroupChats()
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
		return c.EditOrSend(ctx,
			"Вот список известных мне чатов.\n"+
				"Для вступления в большинство из них требуется подтверждение от администратора чата (🔐).",
			markup.InlineMarkup(rows...),
		)
	}
	bot.Handle(&markup.DistrictChatsBtn, chatsHandler)
	bot.Handle("/chats", chatsHandler)

	registrationService := newTelegramRegistrar(log, userRepository, houses, markup.HelpMainMenuBtn)
	registrationService.Register(bot)

	var authMiddleware telebot.MiddlewareFunc = func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(ctx context.Context, c telebot.Context) error {
			ctx, span := tracer.Open(ctx, tracer.Named("AuthMiddleware"))
			if c.Chat().Type == telebot.ChatPrivate && userRepository.IsResident(ctx, c.Sender().ID) {
				span.Close()
				return next(ctx, c)
			}
			defer span.Close()
			var rows []telebot.Row
			if c.Chat().Type == telebot.ChatPrivate {
				user := repository.CurrentUserFromContext(ctx)
				if user != nil && user.HavePendingRegistration() {
					rows = append(rows, markup.Row(markup.ContinueRegisterBtn))
				} else {
					rows = append(rows, markup.Row(markup.RegisterBtn))
				}
			}
			rows = append(rows, markup.Row(markup.HelpMainMenuBtn))
			return c.EditOrSend(ctx, `Этот раздел только для резидентов изумрудного бора. И доступен только в личном общении с ботом.
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

	getResidentsMarkup := func(ctx context.Context, c telebot.Context) *telebot.ReplyMarkup {
		_, span := tracer.Open(ctx, tracer.Named("getResidentsMarkup"))
		defer span.Close()
		var rows []telebot.Row
		rows = append(rows,
			// residentsMenuMarkup.Row(intercomCodeBtn),
			markup.Row(markup.VideoCamerasBtn),
			markup.Row(markup.PMWithResidentsBtn),
			markup.Row(markup.PMWithCarOwnersBtn),
			markup.Row(carsService.EntryPoint()),
			markup.Row(markup.HelpMainMenuBtn),
		)
		return markup.InlineMarkup(rows...)
	}

	registrationCheckApproveCode := func(c telebot.Context, ctx context.Context, user *repository.User, approveCode string) error {
		if user.Registration == nil {
			return c.EditOrReply(ctx, "Ошибка регистрации: вы не начинали регистрацию, поэтому не можете её завершить", getResidentsMarkup(ctx, c))
		}
		if approveCode == user.Registration.Events.Start.ApproveCode {
			userRepository.ConfirmRegistration(
				ctx,
				c.Sender().ID,
				repository.ConfirmRegistrationEvent{UpdateID: int64(c.Update().ID), WithCode: approveCode},
			)
			return c.EditOrReply(ctx, "Спасибо. Регистрация завершена.", getResidentsMarkup(ctx, c))
		} else {
			userRepository.FailRegistration(
				ctx,
				c.Sender().ID,
				repository.FailRegistrationEvent{UpdateID: int64(c.Update().ID), WithCode: approveCode},
			)
			return c.EditOrReply(ctx,
				"Неверный код. Попробуем заново? Процесс такой же: выбираете дом и квартиру и ждёте правильный код на почту.",
				markup.HelpMenuMarkup(ctx),
			)
		}
	}

	/*
				handleContinueRegistration := func(c telebot.Context, ctx context.Context, user *User) error {
					ctx, span := tracer.Open(ctx, tracer.Named("handleContinueRegistration"))
		defer span.Close()
					if err != nil {
						return fmt.Errorf("продолжение регистрации: %w", err)
					}
					data := ctx.Args()
					if len(data) == 0 || len(data) == 1 && data[0] == "" {
						var allCodes []string = append(append([]string(nil), user.Registration.Events.Start.ApproveCode), user.Registration.Events.Start.InvalidCodes...)
						rand.Shuffle(len(allCodes), reflect.Swapper(allCodes))
						conRegMarkup := ctx.Bot().NewMarkup()
						var rows []telebot.Row
						var buttons []telebot.Btn
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
						return ctx.EditOrReply(ctx, "Для завершения регистрации выберите правильный код, который вы нашли у себя в почтовом ящике.\n"+
							"Если Ваш дом ещё не сдан, то вы можете пользоваться частью сервисов и завершить регистрацию после заселения.", conRegMarkup)
					}
					return registrationCheckApproveCode(c, ctx, user, data[0])
				}
	*/
	handleMaybeRegistration := func(c telebot.Context, ctx context.Context, token string) error {
		var approveToken repository.UserRegistrationApproveToken
		err := DecodeSignedMessage(token, &approveToken)
		if err != nil {
			return err
		}
		if approveToken.UserID != c.Sender().ID {
			return c.EditOrReply(ctx, "Этот код регистрации для другого пользователя. Перепутали телефон?", markup.HelpMenuMarkup(ctx))
		}

		user, err := userRepository.GetUser(ctx, userRepository.ByID(c.Sender().ID))
		if err != nil {
			return err
		}
		return registrationCheckApproveCode(c, ctx, user, approveToken.ApproveCode)
	}

	bot.Handle("/start", func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("/start"))
		defer span.Close()
		if len(c.Args()) == 1 && len(c.Args()[0]) > 4 {
			if err := handleMaybeRegistration(c, ctx, c.Args()[0]); err == nil {
				return nil
			} else {
				log.Error("Ошибочная /start регистрация", zap.Error(err))
			}
		}
		return c.EditOrReply(ctx, "Привет! "+handlers.BotDescription+"\nИспользуйте команду /help для вызова меню")
	})

	bot.Handle("/clear", handlers.ClearAllDataController(userRepository))

	authGroup := bot.Group()
	authGroup.Use(authMiddleware)

	residentsHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("residentsHandler"))
		defer span.Close()
		return c.EditOrSend(ctx, "Немного полезностей для резидентов", getResidentsMarkup(ctx, c))
	}
	authGroup.Handle(&markup.ResidentsBtn, residentsHandler)

	intercomHandlers := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("intercomHandlers"))
		defer span.Close()
		return c.EditOrSend(ctx,
			"Здесь будет актуальный код для прохода через домофон. Если вы знаете теукщий код - напишите его мне.",
			getResidentsMarkup(ctx, c),
		)
	}
	authGroup.Handle(&markup.IntercomCodeBtn, intercomHandlers)

	videoCamerasHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("videoCamerasHandler"))
		defer span.Close()
		return c.EditOrSend(ctx, `
<a href="https://vs.domru.ru">Площадка 108А</a>
Логин: <code>ertel-wk-557</code>
Пароль: <code>uu4rg2x3</code>

<a href="https://video.ugmk-telecom.ru">Площадка 108Б</a>
Логин: <code>108b</code>
Пароль: <code>izumrud20</code>

Для просмотра можно воспользоваться приложением Форпост.
`,
			telebot.ModeHTML,
			markup.InlineMarkup(markup.Row(markup.BackToResidentsBtn)))
	}
	authGroup.Handle(&markup.VideoCamerasBtn, videoCamerasHandler)

	residentsChatter, err := NewResidentsChatter(ctx, userRepository, houses, markup.BackToResidentsBtn)
	if err != nil {
		log.Fatal("Ошибка инициализации чатов", zap.Error(err))
	}
	residentsChatter.RegisterBotsHandlers(ctx, authGroup)
	pmWithResidentsHandler := residentsChatter.HandleChatWithResident
	authGroup.Handle("/connect", pmWithResidentsHandler)
	authGroup.Handle(&markup.PMWithResidentsBtn, pmWithResidentsHandler)

	carownerChatter, err := NewCarOwnerChatter(markup.BackToResidentsBtn, userRepository)
	if err != nil {
		log.Fatal("Ошибка инициализации чатов", zap.Error(err))
	}
	carownerChatter.RegisterBotsHandlers(ctx, authGroup)
	authGroup.Handle("/beep", func(ctx context.Context, c telebot.Context) error {
		return c.EditOrReply(ctx, "Пробуем связаться с владельцем авто", markup.InlineMarkup(markup.Row(markup.PMWithCarOwnersBtn)))
	})
	authGroup.Handle(&markup.PMWithCarOwnersBtn, carownerChatter.HandleInputCarPlate)

	forwardDeveloperHandler := devbotsender.ForwardToDeveloper(log.Named("forwardToDeveloper"))

	obsceneFilter := services.NewObsceneFilter(log.Named("obsceneFilter"))

	bot.Handle(telebot.OnText, func(ctx context.Context, c telebot.Context) error {
		if c.Chat().Type == telebot.ChatPrivate {
			return forwardDeveloperHandler(ctx, c)
		}
		log.Info("Handling anti spam")
		return manageAntiSpam(log, groupChats, obsceneFilter)(ctx, c)
	})
	bot.Handle(telebot.OnMedia, func(ctx context.Context, c telebot.Context) error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if c.Chat().Type == telebot.ChatPrivate {
			user, err := userRepository.GetUser(ctx, userRepository.ByID(c.Sender().ID))
			if err != nil {
				return fmt.Errorf("telebot.OnMedia: %w", err)
			}
			if user.Registration != nil {
				return registrationService.HandleMediaCreated(ctx, user, c)
			}
			return forwardDeveloperHandler(ctx, c)
		}
		return nil
	})
}
