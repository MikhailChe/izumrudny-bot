package main

import (
	"context"
	"fmt"
	"strconv"

	"mikhailche/botcomod/repositories"
	. "mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

type ResidentsChatter struct {
	users  residentsUserRepository
	houses func() repositories.THouses

	upperMenu tele.Btn

	startChat             tele.Btn
	houseIsChosen         tele.Btn
	appartmentRangeChosen tele.Btn
	appartmentChosen      tele.Btn
	chatRequestApproved   tele.Btn
	allowContact          tele.Btn
	denyContact           tele.Btn
}

type residentsUserRepository interface {
	FindByAppartment(ctx context.Context, house string, appartment string) (*User, error)
}

func NewResidentsChatter(users residentsUserRepository, houses func() repositories.THouses, upperMenu tele.Btn) (*ResidentsChatter, error) {
	defer Trace("NewResidentsChatter")()
	markup := &tele.ReplyMarkup{}
	return &ResidentsChatter{
		users:                 users,
		houses:                houses,
		upperMenu:             upperMenu,
		startChat:             markup.Data("💬 Связаться с резидентом", "chat-with-resident"),
		houseIsChosen:         markup.Data("🏠 Дом выбран", "chat-with-resident-house-chosen"),                          // псевдо-кнопка для обработчика и хранения unique
		appartmentRangeChosen: markup.Data("🚪🚪 Диапазон квартир выбран", "chat-with-resident-appart-range"),            // псевдо-кнопка для обработчика и хранения unique
		appartmentChosen:      markup.Data("🚪 Квартира выбрана", "chat-with-resident-appart-chosen"),                   // псевдо-кнопка для обработчика и хранения unique
		chatRequestApproved:   markup.Data("Крикнуть", "chat-with-resident-confirm-request"),                           // псевдо-кнопка для обработчика и хранения unique
		allowContact:          markup.Data("Разрешить отправку контактных данных", "chat-with-resident-allow-contact"), // псевдо-кнопка для обработчика и хранения unique
		denyContact:           markup.Data("Запретить отправку контактных данных", "chat-with-resident-deny-contact"),  // псевдо-кнопка для обработчика и хранения unique
	}, nil
}

type HandleRegistrator interface {
	Handle(endpoint interface{}, h tele.HandlerFunc, m ...tele.MiddlewareFunc)
}

func (r *ResidentsChatter) RegisterBotsHandlers(bot HandleRegistrator) {
	defer Trace("ResidentsChatter::RegisterBotsHandlers")()
	bot.Handle(&r.houseIsChosen, r.HandleHouseIsChosen)
	bot.Handle(&r.appartmentRangeChosen, r.HandleAppartmentRangeChosen)
	bot.Handle(&r.appartmentChosen, r.HandleAppartmentChosen)
	bot.Handle(&r.chatRequestApproved, r.HandleChatRequestApproved)
	bot.Handle(&r.allowContact, r.HandleAllowContact)
	bot.Handle(&r.denyContact, r.HandleDenyContact)
}

func (r *ResidentsChatter) HandleChatWithResident(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleChatWithResident")()
	markup := ctx.Bot().NewMarkup()
	var rows []tele.Row
	var buttons []tele.Btn
	for _, house := range r.houses() {
		buttons = append(buttons, markup.Data(house.Number, r.houseIsChosen.Unique, house.Number))
		if len(buttons) > 3 {
			rows = append(rows, markup.Row(buttons...))
			buttons = nil
		}
	}
	if len(buttons) > 0 {
		rows = append(rows, markup.Row(buttons...))
		buttons = nil
	}
	rows = append(rows, markup.Row(r.upperMenu))
	markup.Inline(rows...)
	return ctx.EditOrReply("Какие правила: можно связаться с зарегистрированным резидентом. Для этого нужно выбрать номер дома и "+
		"номер квартиры (машиноместа). Я отправлю запрос на контакт всем, кто проживает по этому адресу вместе с номером дома и квартирой, в которой проживаете вы. "+
		"Если запрос будет подтверждён, то я отправлю обоим участникам контактные данные и вы сможете связаться друг с другом.\n\n"+
		"Итак, с кем хотим связаться?\n"+
		"Выберите номер дома 🏠",
		markup,
	)
}

func (r *ResidentsChatter) houseFromContext(number string) repositories.THouse {
	defer Trace("houseFromContext")()
	for _, house := range r.houses() {
		if house.Number == number {
			return house
		}
	}
	return repositories.THouse{}
}

func (r *ResidentsChatter) HandleHouseIsChosen(ctx tele.Context) error {
	defer Trace("ResidentsChatter::ResidentsChatter")()
	var house repositories.THouse = r.houseFromContext(ctx.Args()[0])
	markup := &tele.ReplyMarkup{}
	var rows []tele.Row
	{
		var buttons []tele.Btn
		for i := house.Rooms.Min; i <= house.Rooms.Max; i += 64 {
			buttonText := fmt.Sprintf("%d - %d", i, i+64)
			buttons = append(buttons, markup.Data(buttonText, r.appartmentRangeChosen.Unique, append(ctx.Args(), fmt.Sprint(i))...))
			if len(buttons) > 3 {
				rows = append(rows, markup.Row(buttons...))
				buttons = nil
			}
		}
		if len(buttons) > 0 {
			rows = append(rows, markup.Row(buttons...))
			buttons = nil
		}
	}
	rows = append(rows, markup.Row(r.upperMenu))
	markup.Inline(rows...)
	return ctx.EditOrReply(fmt.Sprintf("🏠 %s 🏠\nКакая квартира?", house.Number), markup)
}

func (r *ResidentsChatter) HandleAppartmentRangeChosen(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleAppartmentRangeChosen")()
	var house repositories.THouse = r.houseFromContext(ctx.Args()[0])
	appartmentRangeStart, err := strconv.Atoi(ctx.Args()[1])
	if err != nil {
		return fmt.Errorf("парсинг диапазона квартир для чата резидентов [%v]: %w", ctx.Args()[1], err)
	}
	markup := &tele.ReplyMarkup{}
	var rows []tele.Row
	{
		var buttons []tele.Btn
		for i := appartmentRangeStart; i <= appartmentRangeStart+64 && i <= house.Rooms.Max; i++ {
			buttons = append(buttons, markup.Data(fmt.Sprint(i), r.appartmentChosen.Unique, append(ctx.Args(), fmt.Sprint(i))...))
			if i%8 == 0 {
				rows = append(rows, markup.Row(buttons...))
				buttons = nil
			}
		}
		if len(buttons) > 0 {
			rows = append(rows, markup.Row(buttons...))
			buttons = nil
		}
	}
	rows = append(rows, markup.Row(r.upperMenu))
	markup.Inline(rows...)
	return ctx.EditOrReply(fmt.Sprintf("🏠 %s 🏠\nКакая квартира?", house.Number), markup)
}

func (r *ResidentsChatter) HandleAppartmentChosen(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleAppartmentChosen")()
	var house repositories.THouse = r.houseFromContext(ctx.Args()[0])
	appartment, err := strconv.Atoi(ctx.Args()[2])
	if err != nil {
		return fmt.Errorf("парсинг номера квартиры для чата с резидентами [%v]: %w", ctx.Args()[2], err)
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("❌ Неверно", r.startChat.Unique),
			markup.Data("✅ Всё ок", r.chatRequestApproved.Unique, ctx.Args()...),
		),
		markup.Row(r.upperMenu),
	)
	return ctx.EditOrReply(fmt.Sprintf("Проверим, что всё правильно.\nДом 🏠 %s 🏠\nКвартира🚪 %d 🚪", house.Number, appartment), markup)
}

func (r *ResidentsChatter) HandleChatRequestApproved(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleChatRequestApproved")()
	var house repositories.THouse = r.houseFromContext(ctx.Args()[0])
	appartment, err := strconv.Atoi(ctx.Args()[2])
	if err != nil {
		return fmt.Errorf("парсинг номера квартиры для чата с резидентами [%v]: %w", ctx.Args()[2], err)
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(r.upperMenu))
	user, err := r.users.FindByAppartment(context.Background(), house.Number, fmt.Sprint(appartment))
	if err == ErrNotFound {
		return fmt.Errorf(
			"не нашел пользователя проживающего в [%v %d]: %w; %v",
			house.Number, appartment, err,
			ctx.EditOrReply("Я не нашел никого, зарегистрированного по этому адресу. Придется искать другим способом.", markup),
		)
	}
	if err != nil {
		return fmt.Errorf("ошибка поиска пользователя проживающего в [%v %d]: %w",
			house.Number, appartment, err,
		)
	}

	sendMyContactMarkup := &tele.ReplyMarkup{}
	sendMyContactMarkup.Inline(
		sendMyContactMarkup.Row(
			sendMyContactMarkup.Data("❌ Нельзя", r.denyContact.Unique, fmt.Sprint(ctx.Sender().ID)),
			sendMyContactMarkup.Data("✅ Отправить", r.allowContact.Unique, fmt.Sprint(ctx.Sender().ID)),
		),
		sendMyContactMarkup.Row(r.upperMenu),
	)

	if _, err := ctx.Bot().Send(&tele.User{ID: user.ID},
		fmt.Sprintf(
			"С вами хочет связаться %s %s (@%s). Можно ли передать ему ваши контактные данные?",
			ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Sender().Username,
		),
		sendMyContactMarkup,
	); err != nil {
		return fmt.Errorf("не отправил запрос на контакт [%d]: %w", user.ID, err)
	}

	return ctx.EditOrReply("Спасибо. Я отправил приглашение зарегистрированым резидентам этой квартиры. Если они согласятся пообщаться, то вы получите уведомление.", markup)
}

func (r *ResidentsChatter) HandleAllowContact(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleAllowContact")()
	recepient, err := strconv.Atoi(ctx.Args()[0])
	if err != nil {
		return fmt.Errorf("парсинг ID получателя для разрешения контакта [%v]: %w",
			ctx.Args()[0], err,
		)
	}
	enjoy := &tele.ReplyMarkup{}
	enjoy.Inline(
		enjoy.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", ctx.Sender().ID))),
		enjoy.Row(r.upperMenu),
	)
	ctx.Bot().Send(&tele.User{ID: int64(recepient)},
		fmt.Sprintf(
			"Пользователь %s %s (@%s) разрешил поделиться контактом. Общайтесь!",
			ctx.Sender().FirstName, ctx.Sender().LastName, ctx.Sender().Username,
		),
		enjoy,
	)

	enjoyReceipent := &tele.ReplyMarkup{}
	enjoyReceipent.Inline(
		enjoyReceipent.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", recepient))),
		enjoy.Row(r.upperMenu),
	)
	return ctx.EditOrReply("Отправил ваши контакты.", enjoyReceipent)
}

func (r *ResidentsChatter) HandleDenyContact(ctx tele.Context) error {
	defer Trace("ResidentsChatter::HandleDenyContact")()
	recepient, err := strconv.Atoi(ctx.Args()[0])
	if err != nil {
		return fmt.Errorf("парсинг получателя для отказа в контакте: %w", err)
	}
	enjoy := &tele.ReplyMarkup{}
	enjoy.Inline(
		enjoy.Row(r.upperMenu),
	)
	ctx.Bot().Send(&tele.User{ID: int64(recepient)},
		"Пользователь запретил делаться контактом. Придется сходить к нему пешком.",
		enjoy,
	)

	enjoyReceipent := &tele.ReplyMarkup{}
	enjoyReceipent.Inline(
		enjoyReceipent.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", recepient))),
		enjoy.Row(r.upperMenu),
	)
	return ctx.EditOrReply("Ну ладно, возможно там было что-то важное...", enjoyReceipent)
}
