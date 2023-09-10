package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	repositories "mikhailche/botcomod/repository"
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

const BotDescription = `Я бот микрорайона Изумрудный Бор. Я подскажу как позвонить в пункт охраны, УК, найти общий чатик и соседские чаты домов. 
Со мной вы не пропустите важные анонсы и многое другое.

Меня разрабатывают сами жители района на добровольных началах. Если есть предложения - напишите их мне, а я передам разработчикам.
Зарегистрированные резиденты в скором времени смогут искать друг друга по номеру авто или квартиры.`

func AdminCommandController(mux botMux, adminAuth tele.MiddlewareFunc, userRepository *repositories.UserRepository, groupChatService *services.GroupChatService) {
	mux.Use(adminAuth)
	mux.Handle("/chatidlink", func(ctx tele.Context) error {
		defer tracer.Trace("/chatidlink")()
		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(markup.URL("Общаться", fmt.Sprintf("tg://user?id=%s", ctx.Args()[0]))))
		return ctx.Reply("Ссылка на чат", markup)
	})

	mux.Handle("/service", func(ctx tele.Context) error {
		var bot = ctx.Bot()
		if err := bot.SetCommands([]tele.Command{
			{Text: "help", Description: "Справка"},
			{Text: "chats", Description: "Чаты района"},
			{Text: "phones", Description: "Телефоны служб"},
			{Text: "status", Description: "Статус текущих проблем в районе."},
		}, "ru"); err != nil {
			return fmt.Errorf("/service SetCommands: %w", err)
		}

		if err := bot.SetCommands([]tele.Command{
			{Text: "whois", Description: "Узнать информацию о пользователе"},
		},
			tele.CommandScope{Type: tele.CommandScopeAllChatAdmin},
		); err != nil {
			return fmt.Errorf("/services SetAdminCommands: %w", err)
		}

		if _, err := bot.Raw("setMyDescription", map[string]string{
			"description": BotDescription,
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

	mux.Handle("/whoami", func(ctx tele.Context) error {
		defer tracer.Trace("/whoami")()
		userID := ctx.Sender().ID
		if len(ctx.Args()) > 0 && len(ctx.Args()[0]) > 0 {
			parsedUserID, err := strconv.Atoi(ctx.Args()[0])
			if err == nil {
				userID = int64(parsedUserID)
			}
		}
		user, err := userRepository.GetUser(context.Background(), userRepository.ByID(userID))
		if err != nil {
			return fmt.Errorf("не могу достать пользователя: %w", err)
		}
		userRepository.IsResident(context.Background(), userID)
		userAsJson, _ := json.MarshalIndent(*user, "", "  ")
		eventsAsJson, _ := json.MarshalIndent(user.Events, "", "  ")
		return ctx.EditOrReply(fmt.Sprintf("%#v\n\n%v\n\n%v", *user, string(userAsJson), string(eventsAsJson)))
	})

	mux.Handle("/manual_register", func(ctx tele.Context) error {
		if len(ctx.Args()) < 3 {
			return ctx.EditOrReply("Нужно указать ID пользователя, номер дома и номер квартиры")
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

	mux.Handle("/reply", func(ctx tele.Context) error {
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

	mux.Handle("/test", func(ctx tele.Context) error {
		marshalOrEmpty := func(v any) string {
			bb, _ := json.MarshalIndent(v, "", "  ")
			return string(bb)
		}
		printChatMember := func(cm *tele.ChatMember, s *strings.Builder) {
			s.WriteString(marshalOrEmpty(*cm.User))
			s.WriteRune('\n')
			s.WriteString(string(cm.Role))
			s.WriteRune('\n')
			s.WriteString(marshalOrEmpty(cm.Rights))
		}

		chats := groupChatService.GroupChats()
		bot := ctx.Bot()
		var sb strings.Builder
		for _, chat := range chats {
			if chat.TelegramChatID == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("Чат %d\n", chat.TelegramChatID))
			currentGroup, err := bot.ChatByID(chat.TelegramChatID)
			if err != nil {
				sb.WriteString(fmt.Sprintf("Не могу получить чат по ID: %v\n", err))
				continue
			}
			admins, err := ctx.Bot().AdminsOf(currentGroup)
			if err != nil {
				sb.WriteString(fmt.Sprintf("Не могу получить админов: %v\n", err))
				continue
			}
			sb.WriteString("Админы:\n")
			for _, admin := range admins {
				printChatMember(&admin, &sb)
				sb.WriteRune('\n')
			}
			sb.WriteRune('\n')
			botAsMember, err := ctx.Bot().ChatMemberOf(currentGroup, tele.ChatID(bot.Me.ID))
			if err != nil {
				sb.WriteString(fmt.Sprintf("не могу получить информацию о боте в этом чате (бот не добавлен в чат?): %v\n", err))
				continue
			}
			printChatMember(botAsMember, &sb)
			sb.WriteRune('\n')
		}
		return ctx.Send(sb.String(), tele.ModeHTML)
	})
}
