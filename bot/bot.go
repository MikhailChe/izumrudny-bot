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
	"mikhailche/botcomod/repository"
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	tele "github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

type TBot struct {
	Bot *tele.Bot
}

func NewBot(log *zap.Logger,
	userRepository *repository.UserRepository,
	houses func() repository.THouses,
	groupChats *services.GroupChatService,
	updateLogRepository *repository.UpdateLogger,
	telegramChatUpserter func(ctx context.Context, chat tele.Chat) error,
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	chatToUserUpserter func(ctx context.Context, chat, user int64) error,
) (*TBot, error) {
	var b TBot
	rand.Seed(time.Now().UnixMicro())
	b.Init(log, userRepository, houses, groupChats, updateLogRepository, telegramChatUpserter, userGroupsByUserId, chatToUserUpserter)
	return &b, nil
}

func (b *TBot) Init(
	log *zap.Logger,
	userRepository *repository.UserRepository,
	houses func() repository.THouses,
	groupChats *services.GroupChatService,
	updateLogRepository *repository.UpdateLogger,
	telegramChatUpserter func(ctx context.Context, chat tele.Chat) error,
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	chatToUserUpserter func(ctx context.Context, chat, user int64) error,
) {
	defer tracer.Trace("botInit")()
	var err error
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	pref := tele.Settings{
		Token:       telegramToken,
		Synchronous: true,
		Verbose:     false,
		Offline:     false,
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
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("TraceMiddleware")()
			return hf(ctx, c)
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("RecoverMiddleware")()
			defer func() {
				defer tracer.Trace("RecoverMiddleware::defer")()
				if r := recover(); r != nil {
					log.WithOptions(zap.AddCallerSkip(3)).Error("Паника", zap.Any("panicObj", r))
					sendToDeveloper(c, log, fmt.Sprintf("Паника\n\n%v\n\n%#v", r, r))
				}
			}()
			return hf(ctx, c)
		}
	})

	log.Info("Adding UpsertGroupChat middleware")
	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("UpsertGroupChat middleware")()
			log.Info("Running UpsertGroupChat middleware", zap.String("type", string(c.Chat().Type)))
			if c.Chat().Type != tele.ChatPrivate {
				ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
				log.Info("Trying to update chat by telegram ID",
					zap.Int64("telegram_chat_id", c.Chat().ID),
					zap.String("telegram_chat_title", c.Chat().Title),
					zap.String("telegram_chat_type", string(c.Chat().Type)),
				)
				if err := groupChats.UpdateChatByTelegramId(ctx, c.Chat().ID, c.Chat().Title, string(c.Chat().Type)); err != nil {
					log.Error("Cannot update chat by telegram ID", zap.Error(err))
				}
			}
			return hf(ctx, c)
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("UpsertUsername middleware")()
			err := hf(ctx, c)
			userRepository.UpsertUsername(context.Background(), c.Sender().ID, c.Sender().Username)
			if err := telegramChatUpserter(context.Background(), *c.Chat()); err != nil {
				log.Error("telegramChatUpserter middleware failed", zap.Error(err))
			}
			if err := chatToUserUpserter(context.Background(), c.Chat().ID, c.Sender().ID); err != nil {
				log.Error("telegramChatUpserter middleware failed", zap.Error(err))
			}
			return err
		}
	})

	bot.Use(func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx context.Context, c tele.Context) error {
			c.Respond(&tele.CallbackResponse{})
			return hf(ctx, c)
		}
	})

	adminAuthMiddleware := func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("AdminCommandControllerAuth middleware")()
			if userRepository.IsAdmin(context.Background(), c.Sender().ID) {
				return hf(ctx, c)
			}
			return nil
		}
	}
	groupChatAdminAuthMiddleware := adminAuthMiddleware

	log.Info("Adding admin command controller")
	handlers.AdminCommandController(bot.Group(), adminAuthMiddleware, userRepository, groupChats)

	log.Info("Adding replay update controller")
	handlers.ReplayUpdateController(bot.Group(), adminAuthMiddleware, updateLogRepository, bot)

	handlers.StaticDataController(bot.Group(), groupChats)
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

	chatsHandler := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("chatsHandler")()
		var rows []tele.Row
		var linkGroup []tele.Btn
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
		return c.EditOrSend(
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
		return func(ctx context.Context, c tele.Context) error {
			defer tracer.Trace("AuthMiddleware")()
			if userRepository.IsResident(context.Background(), c.Sender().ID) {
				return next(ctx, c)
			}
			var rows []tele.Row
			rows = append(rows, markup.Row(*registrationService.EntryPoint()))
			rows = append(rows, markup.Row(markup.HelpMainMenuBtn))
			return c.EditOrSend(`Этот раздел только для резидентов изумрудного бора. 
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
		user, err := userRepository.GetUser(context.Background(), userRepository.ByID(ctx.Sender().ID))
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

	registrationCheckApproveCode := func(c tele.Context, ctx context.Context, user *repository.User, approveCode string) error {
		if user.Registration == nil {
			return c.EditOrReply("Ошибка регистрации: вы не начинали регистрацию, поэтому не можете её завершить", getResidentsMarkup(c))
		}
		if approveCode == user.Registration.Events.Start.ApproveCode {
			userRepository.ConfirmRegistration(
				ctx,
				c.Sender().ID,
				repository.ConfirmRegistrationEvent{UpdateID: int64(c.Update().ID), WithCode: approveCode},
			)
			return c.EditOrReply("Спасибо. Регистрация завершена.", getResidentsMarkup(c))
		} else {
			userRepository.FailRegistration(
				ctx,
				c.Sender().ID,
				repository.FailRegistrationEvent{UpdateID: int64(c.Update().ID), WithCode: approveCode},
			)
			return c.EditOrReply(
				"Неверный код. Попробуем заново? Процесс такой же: выбираете дом и квартиру и ждёте правильный код на почту.",
				markup.HelpMenuMarkup(),
			)
		}
	}

	/*
		handleContinueRegistration := func(c tele.Context, ctx context.Context, user *User) error {
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
			return registrationCheckApproveCode(c, ctx, user, data[0])
		}
	*/
	handleMaybeRegistration := func(c tele.Context, ctx context.Context, token string) error {
		var approveToken repository.UserRegistrationApproveToken
		err := DecodeSignedMessage(token, &approveToken)
		if err != nil {
			return err
		}
		if approveToken.UserID != c.Sender().ID {
			return c.EditOrReply("Этот код регистрации для другого пользователя. Перепутали телефон?", markup.HelpMenuMarkup())
		}

		user, err := userRepository.GetUser(ctx, userRepository.ByID(c.Sender().ID))
		if err != nil {
			return err
		}
		return registrationCheckApproveCode(c, ctx, user, approveToken.ApproveCode)
	}

	bot.Handle("/start", func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("/start")()
		if len(c.Args()) == 1 && len(c.Args()[0]) > 4 {
			if err := handleMaybeRegistration(c, context.Background(), c.Args()[0]); err == nil {
				return nil
			} else {
				log.Error("Ошибочная /start регистрация", zap.Error(err))
			}
		}
		return c.EditOrReply("Привет! " + handlers.BotDescription + "\nИспользуйте команду /help для вызова меню")
	})

	authGroup := bot.Group()
	authGroup.Use(authMiddleware)

	residentsHandler := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("residentsHandler")()
		return c.EditOrSend("Немного полезностей для резидентов", getResidentsMarkup(c))
	}
	authGroup.Handle(&markup.ResidentsBtn, residentsHandler)

	intercomHandlers := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("intercomHandlers")()
		return c.EditOrSend(
			"Здесь будет актуальный код для прохода через домофон. Если вы знаете теукщий код - напишите его мне.",
			getResidentsMarkup(c),
		)
	}
	authGroup.Handle(&markup.IntercomCodeBtn, intercomHandlers)

	videoCamerasHandler := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("videoCamerasHandler")()
		return c.EditOrSend(`
<a href="https://vs.domru.ru">Площадка 108А</a>
Логин: <code>ertel-wk-557</code>
Пароль: <code>uu4rg2x3</code>

<a href="https://video.ugmk-telecom.ru">Площадка 108Б</a>
Логин: <code>108b</code>
Пароль: <code>izumrud20</code>

Для просмотра можно воспользоваться приложением Форпост.
`,
			tele.ModeHTML,
			getResidentsMarkup(c))
	}
	authGroup.Handle(&markup.VideoCamerasBtn, videoCamerasHandler)

	residentsChatter, err := NewResidentsChatter(userRepository, houses, markup.HelpMainMenuBtn)
	if err != nil {
		log.Fatal("Ошибка инициализации чатов", zap.Error(err))
	}

	residentsChatter.RegisterBotsHandlers(authGroup)
	pmWithResidentsHandler := func(ctx context.Context, c tele.Context) error {
		defer tracer.Trace("pmWithResidentsHandler")()
		return residentsChatter.HandleChatWithResident(ctx, c)
	}
	authGroup.Handle("/connect", pmWithResidentsHandler)
	authGroup.Handle(&markup.PMWithResidentsBtn, pmWithResidentsHandler)

	forwardDeveloperHandler := forwardToDeveloper(log.Named("forwardToDeveloper"))

	obsceneFilter := services.NewObsceneFilter(log.Named("obsceneFilter"))

	bot.Handle(tele.OnText, func(ctx context.Context, c tele.Context) error {
		if c.Chat().Type == tele.ChatPrivate {
			return forwardDeveloperHandler(ctx, c)
		}
		log.Info("Handling anti spam")
		return manageAntiSpam(log, groupChats, obsceneFilter)(ctx, c)
	})
	bot.Handle(tele.OnMedia, func(ctx context.Context, c tele.Context) error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		user, err := userRepository.GetUser(ctx, userRepository.ByID(int64(c.Sender().ID)))
		if err != nil {
			return fmt.Errorf("tele.OnMedia: %w", err)
		}
		if user.Registration != nil {
			return registrationService.HandleMediaCreated(user, c)
		}
		return forwardDeveloperHandler(ctx, c)
	})
}
