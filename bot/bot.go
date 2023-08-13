package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"mikhailche/botcomod/handlers"
	"mikhailche/botcomod/lib/http"
	"mikhailche/botcomod/repositories"
	"mikhailche/botcomod/tracer"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

const botDescription = `Я бот микрорайона Изумрудный Бор. Я подскажу как позвонить в пункт охраны, УК, найти общий чатик и соседские чаты домов. 
Со мной вы не пропустите важные анонсы и многое другое.

Меня разрабатывают сами жители района на добровольных началах. Если есть предложения - напишите их мне, а я передам разработчикам.
Зарегистрированные резиденты в скором времени смогут искать друг друга по номеру авто или квартиры.`

type TBot struct {
	Bot *tele.Bot
}

func NewBot(log *zap.Logger, userRepository botUserRepository, houses func() repositories.THouses, groupChats func() repositories.TGroupChats) (*TBot, error) {
	var b TBot
	rand.Seed(time.Now().UnixMicro())
	b.Init(log, userRepository, houses, groupChats)
	return &b, nil
}

type botUserRepository interface {
	IsAdmin(ctx context.Context, userID int64) bool
	UpsertUsername(ctx context.Context, userID int64, username string)
	IsResident(ctx context.Context, userID int64) bool
	GetById(ctx context.Context, userID int64) (*User, error)
	StartRegistration(ctx context.Context, userID int64, UpdateID int64, houseNumber string, apartment string) (approveCode string, err error)
	ConfirmRegistration(ctx context.Context, userID int64, event confirmRegistrationEvent) error
	FailRegistration(ctx context.Context, userID int64, event failRegistrationEvent) error

	RegisterCarLicensePlate(ctx context.Context, userID int64, event registerCarLicensePlateEvent) error

	FindByAppartment(ctx context.Context, house string, appartment string) (*User, error)
}

func (b *TBot) Init(log *zap.Logger, userRepository botUserRepository, houses func() repositories.THouses, groupChats func() repositories.TGroupChats) {
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
				log.Error("Ошибка внутри бота", zap.Any("update", c.Update()), zap.Error(err), zap.Reflect("errorStruct", err), zap.String("errorType", fmt.Sprintf("%T", err)))
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

	log.Info("Adding admin command controller")
	handlers.AdminCommandController(bot.Group(), func(hf tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("AdminCommandControllerAuth middleware")()
			if userRepository.IsAdmin(context.Background(), ctx.Sender().ID) {
				return hf(ctx)
			}
			return nil
		}
	})

	var markup = bot.NewMarkup()
	helpMainMenuBtn := markup.Data("⬅️ Назад в главное меню", "help-main-menu")
	districtChatsBtn := markup.Data("💬🏘 Чаты района", "district-chats")

	helpfulPhonesBtn := markup.Data("☎️ Телефоны", "phone-numbers")

	residentsBtn := markup.Data("🏡 Для резидентов", "authorized-section")
	intercomCodeBtn := markup.Data("🔑 Код домофона", "intercom-code")
	videoCamerasBtn := markup.Data("📽 Камеры видеонаблюдения", "internal-video-cameras")
	pmWithResidentsBtn := markup.Data("💬 Чат с другими резидентами", "resident-pm")

	// registerBtn := markup.Data("📒 Начать регистрацию", "registration")

	helpMenuMarkup := func() *tele.ReplyMarkup {
		defer tracer.Trace("helpMenuMarkup")()
		var markup = bot.NewMarkup()
		markup.Inline(
			markup.Row(districtChatsBtn),
			markup.Row(helpfulPhonesBtn),
			markup.Row(residentsBtn),
			markup.Row(markup.Text("🟢 Без комунальных проблем")),
		)
		return markup
	}

	helpHandler := func(ctx tele.Context) error {
		defer tracer.Trace("helpHandler")()
		return ctx.EditOrSend(
			"Привет. Я помогу сориентироваться в Изумрудном Бору.\nВы всегда можете вызвать это меню командой /help",
			helpMenuMarkup(),
		)
	}
	bot.Handle("/help", helpHandler)
	bot.Handle(&helpMainMenuBtn, helpHandler)

	chatsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("chatsHandler")()
		var markup = bot.NewMarkup()
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
		rows = append(rows, markup.Row(helpMainMenuBtn))
		markup.Inline(rows...)
		return ctx.EditOrSend(
			"Вот список известных мне чатов.\n"+
				"Для вступления в большинство из них требуется подтверждение от администратора чата (🔐).",
			markup,
		)
	}
	bot.Handle(&districtChatsBtn, chatsHandler)
	bot.Handle("/chats", chatsHandler)

	handlers.PhonesController(bot, &helpMainMenuBtn, &helpfulPhonesBtn)

	registrationService := newTelegramRegistrator(log, userRepository, houses, helpMainMenuBtn)
	registrationService.Register(bot)

	var authMiddleware tele.MiddlewareFunc = func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			defer tracer.Trace("AuthMiddleware")()
			if userRepository.IsResident(context.Background(), ctx.Sender().ID) {
				return next(ctx)
			}
			markup := bot.NewMarkup()
			var rows []tele.Row
			rows = append(rows, markup.Row(*registrationService.EntryPoint()))
			rows = append(rows, markup.Row(helpMainMenuBtn))
			markup.Inline(rows...)
			return ctx.EditOrSend(`Этот раздел только для резидентов изумрудного бора. 
Нажмите клавишу регистрации, чтобы получить доступ. Регистрация может занять от нескольких минут до нескольких дней.

После регистрации вы получите доступ к коду от домофона 🔑, ссылкам на видеокамеры, установленные в районе 📽.
А в некоторые соседские чаты вы сможете вступать без дополнительной проверки.

В будущем мы дадим возможность не раскрывая персональных данных общаться с любым резидентом по номеру квартиры или автомобиля на парковке.`,
				markup,
			)
		}
	}

	carsService := NewCarsHandler(userRepository, &helpMainMenuBtn)
	carsService.Register(bot)

	getResidentsMarkup := func(ctx tele.Context) *tele.ReplyMarkup {
		defer tracer.Trace("getResidentsMarkup")()
		user, err := userRepository.GetById(context.Background(), ctx.Sender().ID)
		if err != nil || user.Registration == nil {
			residentsMenuMarkup := bot.NewMarkup()
			var rows []tele.Row
			rows = append(rows,
				// residentsMenuMarkup.Row(intercomCodeBtn),
				residentsMenuMarkup.Row(videoCamerasBtn),
				residentsMenuMarkup.Row(pmWithResidentsBtn),
				residentsMenuMarkup.Row(helpMainMenuBtn),
			)
			if userRepository.IsAdmin(context.Background(), ctx.Sender().ID) {
				rows = append(rows, residentsMenuMarkup.Row(carsService.EntryPoint()))
			}
			residentsMenuMarkup.Inline(rows...)
			return residentsMenuMarkup
		}
		continueRegisterBtn := markup.Data("📒 Продолжить регистрацию", registrationService.EntryPoint().Unique)

		residentsMenuMarkup := bot.NewMarkup()
		residentsMenuMarkup.Inline(
			// residentsMenuMarkup.Row(intercomCodeBtn),
			residentsMenuMarkup.Row(videoCamerasBtn),
			residentsMenuMarkup.Row(pmWithResidentsBtn),
			residentsMenuMarkup.Row(continueRegisterBtn),
			residentsMenuMarkup.Row(helpMainMenuBtn),
		)
		return residentsMenuMarkup
	}

	registrationCheckApproveCode := func(ctx tele.Context, stdctx context.Context, user *User, approveCode string) error {
		if user.Registration == nil {
			return ctx.EditOrReply("Ошибка регистрации: вы не начинали регистрацию, поэтому не можете её завершить", getResidentsMarkup(ctx))
		}
		if approveCode == user.Registration.Events.Start.ApproveCode {
			userRepository.ConfirmRegistration(stdctx, ctx.Sender().ID, confirmRegistrationEvent{int64(ctx.Update().ID), approveCode})
			return ctx.EditOrReply("Спасибо. Регистрация завершена.", getResidentsMarkup(ctx))
		} else {
			userRepository.FailRegistration(stdctx, ctx.Sender().ID, failRegistrationEvent{int64(ctx.Update().ID), approveCode})
			return ctx.EditOrReply("Неверный код. Попробуем заново? Процесс такой же: выбираете дом и квартиру и ждёте правильный код на почту.", helpMenuMarkup())
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
		var approveToken useRegistrationApproveToken
		err := DecodeSignedMessage(token, &approveToken)
		if err != nil {
			return err
		}
		if approveToken.UserID != ctx.Sender().ID {
			return ctx.EditOrReply("Этот код регистрации для другого пользователя. Перепутали телефон?", helpMenuMarkup())
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
		return ctx.EditOrReply("Привет! " + botDescription + "\nИспользуйте команду /help для вызова меню")
	})

	bot.Handle("/whoami", func(ctx tele.Context) error {
		defer tracer.Trace("/whoami")()
		userID := ctx.Sender().ID
		if len(ctx.Args()) > 0 && len(ctx.Args()[0]) > 0 {
			parsedUserID, err := strconv.Atoi(ctx.Args()[0])
			if err == nil {
				userID = int64(parsedUserID)
			}
		}
		user, err := userRepository.GetById(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("не могу достать пользователя: %w", err)
		}
		userRepository.IsResident(context.Background(), userID)
		userAsJson, _ := json.MarshalIndent(*user, "", "  ")
		eventsAsJson, _ := json.MarshalIndent(user.Events, "", "  ")
		return ctx.EditOrReply(fmt.Sprintf("%#v\n\n%v\n\n%v", *user, string(userAsJson), string(eventsAsJson)))
	})

	authGroup := bot.Group()
	authGroup.Use(authMiddleware)

	residentsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("residentsHandler")()
		return ctx.EditOrSend("Немного полезностей для резидентов", getResidentsMarkup(ctx))
	}
	authGroup.Handle(&residentsBtn, residentsHandler)

	intercomHandlers := func(ctx tele.Context) error {
		defer tracer.Trace("intercomHandlers")()
		return ctx.EditOrSend("Здесь будет актуальный код для прохода через домофон. Если вы знаете теукщий код - напишите его мне.", getResidentsMarkup(ctx))
	}
	authGroup.Handle(&intercomCodeBtn, intercomHandlers)

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
	authGroup.Handle(&videoCamerasBtn, videoCamerasHandler)

	residentsChatter, err := NewResidentsChatter(userRepository, houses, helpMainMenuBtn)
	if err != nil {
		log.Fatal("Ошибка инициализации чатов", zap.Error(err))
	}

	residentsChatter.RegisterBotsHandlers(authGroup)
	pmWithResidentsHandler := func(ctx tele.Context) error {
		defer tracer.Trace("pmWithResidentsHandler")()
		return residentsChatter.HandleChatWithResident(ctx)
	}
	authGroup.Handle("/connect", pmWithResidentsHandler)
	authGroup.Handle(&pmWithResidentsBtn, pmWithResidentsHandler)

	bot.Handle("/reply", func(ctx tele.Context) error {
		if len(ctx.Args()) <= 1 {
			return nil
		}

		id, err := strconv.Atoi(ctx.Args()[0])
		if err != nil {
			return fmt.Errorf(
				"парсинг id пользователья для ответа: %v: %w",
				ctx.Reply(fmt.Sprintf("Не получилось: %v", err)),
				err,
			)
		}
		message := strings.Join(ctx.Args()[1:], " ")
		_, err = ctx.Bot().Send(&tele.User{ID: int64(id)}, message)
		if err != nil {
			return fmt.Errorf("/reply пользователю: %w", err)
		}
		return nil
	})

	bot.Handle("/manual_register", func(ctx tele.Context) error {
		if !userRepository.IsAdmin(context.Background(), ctx.Sender().ID) {
			return nil
		}
		userID, err := strconv.ParseInt(ctx.Args()[0], 10, 64)
		if err != nil {
			return ctx.Reply(fmt.Sprintf("Пользователь неверный: %v", userID))
		}

		approveCode, err := userRepository.StartRegistration(context.Background(),
			userID,
			int64(ctx.Update().ID),
			ctx.Args()[1],
			ctx.Args()[2])
		if err != nil {
			return ctx.Reply(fmt.Sprintf("Ошибка регистрации: %v", err))
		}

		if _, err := ctx.Bot().Send(
			&tele.User{ID: int64(userID)},
			`Спасибо за регистрацию. 
Пока что вам доступен раздел со ссылками на камеры видеонаблюдения.
В ваш почтовый ящик будет отправлен код подтверждения. Используйте полученный код в меню для резидентов, чтобы завершить регистрацию.
`,
		); err != nil {
			return fmt.Errorf("успешная регистраци: %w", err)
		}
		return ctx.Reply(fmt.Sprintf("Теперь отправь этот код [%v] в дом %v квартира %v", approveCode, ctx.Args()[1], ctx.Args()[2]))

	})

	bot.Handle("/status", func(ctx tele.Context) error {
		defer tracer.Trace("/status")()
		// return ctx.EditOrSend("🟡 Проводятся технические работы на линии интернета оператора МТС")
		return ctx.EditOrSend("🟢 Пока нет известных проблем")
	})

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
	botInitHandleService(bot)
}

func botInitHandleService(bot *tele.Bot) {
	defer tracer.Trace("botInitHandleService")()
	bot.Handle("/service", func(ctx tele.Context) error {
		if err := bot.SetCommands([]tele.Command{
			{Text: "help", Description: "Справка"},
			{Text: "chats", Description: "Чаты района"},
			{Text: "phones", Description: "Телефоны служб"},
			{Text: "status", Description: "Статус текущих проблем в районе."},
		}, "ru"); err != nil {
			return fmt.Errorf("/service SetCommands: %w", err)
		}

		if _, err := bot.Raw("setMyDescription", map[string]string{
			"description": botDescription,
		}); err != nil {
			return fmt.Errorf("/service setMyDescription: %w", err)
		}

		if _, err := bot.Raw("setMyShortDescription", map[string]string{
			"short_description": "Бот изумрдуного бора. Полезные телефоны, ссылки на чаты, анонсы.",
		}); err != nil {
			return fmt.Errorf("/service setMyShortDescription: %w", err)
		}
		return nil
	})
}
