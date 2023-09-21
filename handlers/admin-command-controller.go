package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"mikhailche/botcomod/lib/tracer.v2"
	"strconv"
	"strings"

	repositories "mikhailche/botcomod/repository"
	"mikhailche/botcomod/services"

	"github.com/mikhailche/telebot"
)

const BotDescription = `Я бот микрорайона Изумрудный Бор. Я подскажу как позвонить в пункт охраны, УК, найти общий чатик и соседские чаты домов. 
Со мной вы не пропустите важные анонсы и многое другое.

Меня разрабатывают сами жители района на добровольных началах. Если есть предложения - напишите их мне, а я передам разработчикам.
Зарегистрированные резиденты в скором времени смогут искать друг друга по номеру авто или квартиры.`

func AdminCommandController(mux botMux, adminAuth telebot.MiddlewareFunc, userRepository *repositories.UserRepository, groupChatService *services.GroupChatService) {
	mux.Use(adminAuth)
	mux.Handle("/chatidlink", func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("/chatidlink"))
		defer span.Close()
		markup := &telebot.ReplyMarkup{}
		markup.Inline(markup.Row(markup.URL("Общаться", fmt.Sprintf("tg://user?id=%s", c.Args()[0]))))
		return c.Reply("Ссылка на чат", markup)
	})

	mux.Handle("/service", func(ctx context.Context, c telebot.Context) error {
		var bot = c.Bot()
		if err := bot.SetCommands([]telebot.Command{
			{Text: "help", Description: "Справка"},
			{Text: "chats", Description: "Чаты района"},
			{Text: "phones", Description: "Телефоны служб"},
			{Text: "status", Description: "Статус текущих проблем в районе."},
		}, "ru"); err != nil {
			return fmt.Errorf("/service SetCommands: %w", err)
		}

		if err := bot.SetCommands([]telebot.Command{
			{Text: "whois", Description: "Узнать информацию о пользователе"},
		},
			telebot.CommandScope{Type: telebot.CommandScopeAllChatAdmin},
		); err != nil {
			return fmt.Errorf("/services SetAdminCommands: %w", err)
		}

		if _, err := bot.Raw(ctx, "setMyDescription", map[string]string{
			"description": BotDescription,
		}); err != nil {
			return fmt.Errorf("/service setMyDescription: %w", err)
		}

		if _, err := bot.Raw(ctx, "setMyShortDescription", map[string]string{
			"short_description": "Бот изумрдуного бора. Полезные телефоны, ссылки на чаты, анонсы.",
		}); err != nil {
			return fmt.Errorf("/service setMyShortDescription: %w", err)
		}
		return nil
	})

	mux.Handle("/whoami", func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("/whoami"))
		defer span.Close()
		userID := c.Sender().ID
		if len(c.Args()) > 0 && len(c.Args()[0]) > 0 {
			parsedUserID, err := strconv.Atoi(c.Args()[0])
			if err == nil {
				userID = int64(parsedUserID)
			}
		}
		user, err := userRepository.GetUser(ctx, userRepository.ByID(userID))
		if err != nil {
			return fmt.Errorf("не могу достать пользователя: %w", err)
		}
		userRepository.IsResident(ctx, userID)
		userAsJson, _ := json.MarshalIndent(*user, "", "  ")
		eventsAsJson, _ := json.MarshalIndent(user.Events, "", "  ")
		return c.EditOrReply(ctx, fmt.Sprintf("%#v\n\n%v\n\n%v", *user, string(userAsJson), string(eventsAsJson)))
	})

	mux.Handle("/manual_register", func(ctx context.Context, c telebot.Context) error {
		if len(c.Args()) < 3 {
			return c.EditOrReply(ctx, "Нужно указать ID пользователя, номер дома и номер квартиры")
		}
		userID, err := strconv.ParseInt(c.Args()[0], 10, 64)
		if err != nil {
			return c.Reply(fmt.Sprintf("Пользователь неверный: %v", userID))
		}

		approveCode, err := userRepository.StartRegistration(
			ctx,
			userID,
			int64(c.Update().ID),
			c.Args()[1],
			c.Args()[2])
		if err != nil {
			return c.Reply(fmt.Sprintf("Ошибка регистрации: %v", err))
		}

		if _, err := c.Bot().Send(ctx,
			&telebot.User{ID: int64(userID)},
			`Спасибо за регистрацию. 
Пока что вам доступен раздел со ссылками на камеры видеонаблюдения.
В ваш почтовый ящик будет отправлен код подтверждения. Используйте полученный код в меню для резидентов, чтобы завершить регистрацию.
`,
		); err != nil {
			return fmt.Errorf("успешная регистраци: %w", err)
		}
		return c.Reply(fmt.Sprintf("Теперь отправь этот код [%v] в дом %v квартира %v", approveCode, c.Args()[1], c.Args()[2]))

	})

	mux.Handle("/reply", func(ctx context.Context, c telebot.Context) error {
		if len(c.Args()) <= 1 {
			return nil
		}

		id, err := strconv.Atoi(c.Args()[0])
		if err != nil {
			return fmt.Errorf(
				"парсинг id пользователья для ответа: %v: %w",
				c.Reply(fmt.Sprintf("Не получилось: %v", err)),
				err,
			)
		}
		message := strings.Join(c.Args()[1:], " ")
		_, err = c.Bot().Send(ctx, &telebot.User{ID: int64(id)}, message)
		if err != nil {
			return fmt.Errorf("/reply пользователю: %w", err)
		}
		return nil
	})

	mux.Handle("/test", func(ctx context.Context, c telebot.Context) error {
		marshalOrEmpty := func(v any) string {
			bb, _ := json.MarshalIndent(v, "", "  ")
			return string(bb)
		}
		printChatMember := func(cm *telebot.ChatMember, s *strings.Builder) {
			s.WriteString(marshalOrEmpty(*cm.User))
			s.WriteRune('\n')
			s.WriteString(string(cm.Role))
			s.WriteRune('\n')
			s.WriteString(marshalOrEmpty(cm.Rights))
		}

		chats := groupChatService.GroupChats()
		bot := c.Bot()
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
			admins, err := c.Bot().AdminsOf(currentGroup)
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
			botAsMember, err := c.Bot().ChatMemberOf(currentGroup, telebot.ChatID(bot.Me.ID))
			if err != nil {
				sb.WriteString(fmt.Sprintf("не могу получить информацию о боте в этом чате (бот не добавлен в чат?): %v\n", err))
				continue
			}
			printChatMember(botAsMember, &sb)
			sb.WriteRune('\n')
		}
		return c.Send(ctx, sb.String(), telebot.ModeHTML)
	})
}
