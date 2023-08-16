package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

type iCommandSetter interface {
	SetCommands(opts ...interface{}) error
	Raw(method string, payload interface{}) ([]byte, error)
}

const BotDescription = `Я бот микрорайона Изумрудный Бор. Я подскажу как позвонить в пункт охраны, УК, найти общий чатик и соседские чаты домов. 
Со мной вы не пропустите важные анонсы и многое другое.

Меня разрабатывают сами жители района на добровольных началах. Если есть предложения - напишите их мне, а я передам разработчикам.
Зарегистрированные резиденты в скором времени смогут искать друг друга по номеру авто или квартиры.`

func AdminCommandController(mux botMux, adminAuth tele.MiddlewareFunc, bot iCommandSetter, userRepository iUserRepository) {
	mux.Use(adminAuth)
	mux.Handle("/chatidlink", func(ctx tele.Context) error {
		defer tracer.Trace("/chatidlink")()
		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(markup.URL("Общаться", fmt.Sprintf("tg://user?id=%s", ctx.Args()[0]))))
		return ctx.Reply("Ссылка на чат", markup)
	})

	mux.Handle("/service", func(ctx tele.Context) error {
		if err := bot.SetCommands([]tele.Command{
			{Text: "help", Description: "Справка"},
			{Text: "chats", Description: "Чаты района"},
			{Text: "phones", Description: "Телефоны служб"},
			{Text: "status", Description: "Статус текущих проблем в районе."},
		}, "ru"); err != nil {
			return fmt.Errorf("/service SetCommands: %w", err)
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
		user, err := userRepository.GetById(context.Background(), userID)
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

}
