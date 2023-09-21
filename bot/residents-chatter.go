package bot

import (
	"context"
	"fmt"
	"mikhailche/botcomod/lib/tracer.v2"
	"strconv"

	markup "mikhailche/botcomod/lib/bot-markup"
	repositories "mikhailche/botcomod/repository"

	"github.com/mikhailche/telebot"
)

type ResidentsChatter struct {
	users  residentsUserRepository
	houses func() repositories.THouses

	upperMenu telebot.Btn

	startChat             telebot.Btn
	houseIsChosen         telebot.Btn
	appartmentRangeChosen telebot.Btn
	appartmentChosen      telebot.Btn
	chatRequestApproved   telebot.Btn
	allowContact          telebot.Btn
	denyContact           telebot.Btn
}

type residentsUserRepository interface {
	FindByAppartment(ctx context.Context, house string, appartment string) (*repositories.User, error)
}

func NewResidentsChatter(ctx context.Context, users residentsUserRepository, houses func() repositories.THouses, upperMenu telebot.Btn) (*ResidentsChatter, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("NewResidentsChatter"))
	defer span.Close()
	markup := &telebot.ReplyMarkup{}
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
	Handle(endpoint interface{}, h telebot.HandlerFunc, m ...telebot.MiddlewareFunc)
}

func (r *ResidentsChatter) RegisterBotsHandlers(ctx context.Context, bot HandleRegistrator) {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::RegisterBotsHandlers"))
	defer span.Close()
	bot.Handle(&r.startChat, r.HandleChatWithResident)
	bot.Handle(&r.houseIsChosen, r.HandleHouseIsChosen)
	bot.Handle(&r.appartmentRangeChosen, r.HandleAppartmentRangeChosen)
	bot.Handle(&r.appartmentChosen, r.HandleAppartmentChosen)
	bot.Handle(&r.chatRequestApproved, r.HandleChatRequestApproved)
	bot.Handle(&r.allowContact, r.HandleAllowContact)
	bot.Handle(&r.denyContact, r.HandleDenyContact)
}

func (r *ResidentsChatter) HandleChatWithResident(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleChatWithResident"))
	defer span.Close()
	var rows []telebot.Row
	var buttons []telebot.Btn
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
	return c.EditOrReply(ctx, "Какие правила: можно связаться с зарегистрированным резидентом. Для этого нужно выбрать номер дома и "+
		"номер квартиры (машиноместа). Я отправлю запрос на контакт всем, кто проживает по этому адресу вместе с номером дома и квартирой, в которой проживаете вы. "+
		"Если запрос будет подтверждён, то я отправлю обоим участникам контактные данные и вы сможете связаться друг с другом.\n\n"+
		"Итак, с кем хотим связаться?\n"+
		"Выберите номер дома 🏠",
		markup.InlineMarkup(rows...),
	)
}

func (r *ResidentsChatter) houseFromContext(ctx context.Context, number string) repositories.THouse {
	ctx, span := tracer.Open(ctx, tracer.Named("houseFromContext"))
	defer span.Close()
	for _, house := range r.houses() {
		if house.Number == number {
			return house
		}
	}
	return repositories.THouse{}
}

func (r *ResidentsChatter) HandleHouseIsChosen(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::ResidentsChatter"))
	defer span.Close()
	var house repositories.THouse = r.houseFromContext(ctx, c.Args()[0])
	markup := &telebot.ReplyMarkup{}
	var rows []telebot.Row
	{
		var buttons []telebot.Btn
		for i := house.Rooms.Min; i <= house.Rooms.Max; i += 64 {
			buttonText := fmt.Sprintf("%d - %d", i, i+64)
			buttons = append(buttons, markup.Data(buttonText, r.appartmentRangeChosen.Unique, append(c.Args(), fmt.Sprint(i))...))
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
	return c.EditOrReply(ctx, fmt.Sprintf("🏠 %s 🏠\nКакая квартира?", house.Number), markup)
}

func (r *ResidentsChatter) HandleAppartmentRangeChosen(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleAppartmentRangeChosen"))
	defer span.Close()
	var house repositories.THouse = r.houseFromContext(ctx, c.Args()[0])
	appartmentRangeStart, err := strconv.Atoi(c.Args()[1])
	if err != nil {
		return fmt.Errorf("парсинг диапазона квартир для чата резидентов [%v]: %w", c.Args()[1], err)
	}
	markup := &telebot.ReplyMarkup{}
	var rows []telebot.Row
	{
		var buttons []telebot.Btn
		for i := appartmentRangeStart; i <= appartmentRangeStart+64 && i <= house.Rooms.Max; i++ {
			buttons = append(buttons, markup.Data(fmt.Sprint(i), r.appartmentChosen.Unique, append(c.Args(), fmt.Sprint(i))...))
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
	return c.EditOrReply(ctx, fmt.Sprintf("🏠 %s 🏠\nКакая квартира?", house.Number), markup)
}

func (r *ResidentsChatter) HandleAppartmentChosen(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleAppartmentChosen"))
	defer span.Close()
	var house repositories.THouse = r.houseFromContext(ctx, c.Args()[0])
	appartment, err := strconv.Atoi(c.Args()[2])
	if err != nil {
		return fmt.Errorf("парсинг номера квартиры для чата с резидентами [%v]: %w", c.Args()[2], err)
	}
	markup := &telebot.ReplyMarkup{}
	markup.Inline(
		markup.Row(
			markup.Data("❌ Неверно", r.startChat.Unique),
			markup.Data("✅ Всё ок", r.chatRequestApproved.Unique, c.Args()...),
		),
		markup.Row(r.upperMenu),
	)
	return c.EditOrReply(ctx, fmt.Sprintf("Проверим, что всё правильно.\nДом 🏠 %s 🏠\nКвартира🚪 %d 🚪", house.Number, appartment), markup)
}

func (r *ResidentsChatter) HandleChatRequestApproved(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleChatRequestApproved"))
	defer span.Close()
	var house repositories.THouse = r.houseFromContext(ctx, c.Args()[0])
	appartment, err := strconv.Atoi(c.Args()[2])
	if err != nil {
		return fmt.Errorf("парсинг номера квартиры для чата с резидентами [%v]: %w", c.Args()[2], err)
	}

	user, err := r.users.FindByAppartment(ctx, house.Number, fmt.Sprint(appartment))
	if err == repositories.ErrNotFound {
		return fmt.Errorf(
			"не нашел пользователя проживающего в [%v %d]: %w; %v",
			house.Number, appartment, err,
			c.EditOrReply(ctx, "Я не нашел никого, зарегистрированного по этому адресу. Придется искать другим способом.", markup.InlineMarkup(markup.Row(r.upperMenu))),
		)
	}
	if err != nil {
		return fmt.Errorf("ошибка поиска пользователя проживающего в [%v %d]: %w",
			house.Number, appartment, err,
		)
	}

	sendMyContactMarkup := &telebot.ReplyMarkup{}
	sendMyContactMarkup.Inline(
		sendMyContactMarkup.Row(
			sendMyContactMarkup.Data("❌ Нельзя", r.denyContact.Unique, fmt.Sprint(c.Sender().ID)),
			sendMyContactMarkup.Data("✅ Отправить", r.allowContact.Unique, fmt.Sprint(c.Sender().ID)),
		),
		sendMyContactMarkup.Row(r.upperMenu),
	)

	if _, err := c.Bot().Send(ctx, &telebot.User{ID: user.ID},
		fmt.Sprintf(
			"С вами хочет связаться %s %s (@%s). Можно ли передать ему ваши контактные данные?",
			c.Sender().FirstName, c.Sender().LastName, c.Sender().Username,
		),
		sendMyContactMarkup,
	); err != nil {
		return fmt.Errorf("не отправил запрос на контакт [%d]: %w", user.ID, err)
	}

	return c.EditOrReply(ctx,
		"Спасибо. Я отправил приглашение зарегистрированым резидентам этой квартиры. Если они согласятся пообщаться, то вы получите уведомление.",
		markup.InlineMarkup(markup.Row(r.upperMenu)),
	)
}

func (r *ResidentsChatter) HandleAllowContact(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleAllowContact"))
	defer span.Close()
	recepient, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		return fmt.Errorf("парсинг ID получателя для разрешения контакта [%v]: %w",
			c.Args()[0], err,
		)
	}
	enjoy := &telebot.ReplyMarkup{}
	enjoy.Inline(
		enjoy.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", c.Sender().ID))),
		enjoy.Row(r.upperMenu),
	)
	c.Bot().Send(ctx, &telebot.User{ID: int64(recepient)},
		fmt.Sprintf(
			"Пользователь %s %s (@%s) разрешил поделиться контактом. Общайтесь!",
			c.Sender().FirstName, c.Sender().LastName, c.Sender().Username,
		),
		enjoy,
	)

	enjoyReceipent := &telebot.ReplyMarkup{}
	enjoyReceipent.Inline(
		enjoyReceipent.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", recepient))),
		enjoy.Row(r.upperMenu),
	)
	return c.EditOrReply(ctx, "Отправил ваши контакты.", enjoyReceipent)
}

func (r *ResidentsChatter) HandleDenyContact(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleDenyContact"))
	defer span.Close()
	recepient, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		return fmt.Errorf("парсинг получателя для отказа в контакте: %w", err)
	}
	enjoy := &telebot.ReplyMarkup{}
	enjoy.Inline(
		enjoy.Row(r.upperMenu),
	)
	c.Bot().Send(ctx, &telebot.User{ID: int64(recepient)},
		"Пользователь запретил делаться контактом. Придется сходить к нему пешком.",
		enjoy,
	)

	enjoyReceipent := &telebot.ReplyMarkup{}
	enjoyReceipent.Inline(
		enjoyReceipent.Row(enjoy.URL("💬 Связаться", fmt.Sprintf("tg://user?id=%d", recepient))),
		enjoy.Row(r.upperMenu),
	)
	return c.EditOrReply(ctx, "Ну ладно, возможно там было что-то важное...", enjoyReceipent)
}
