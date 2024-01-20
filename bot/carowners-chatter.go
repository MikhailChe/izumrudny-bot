package bot

import (
	"context"
	"errors"
	"fmt"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/cars"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/repository"

	"github.com/mikhailche/telebot"
)

type CarOwnerChatter struct {
	upperMenu              telebot.Btn
	handleInputCarPlateBtn telebot.Btn
	confirmCarPlateBtn     telebot.Btn
	allowContact           telebot.Btn
	denyContact            telebot.Btn

	users UserByVehicleLicensePlateRepository
}

type UserByVehicleLicensePlateRepository interface {
	FindByVehicleLicensePlate(ctx context.Context, vehicleLicensePlate string) (*repository.User, error)
}

func NewCarOwnerChatter(upperMenu telebot.Btn, users UserByVehicleLicensePlateRepository) (*CarOwnerChatter, error) {
	return &CarOwnerChatter{
		upperMenu:              upperMenu,
		handleInputCarPlateBtn: markup.PMWithCarOwnersBtn,
		confirmCarPlateBtn:     markup.Data("✅ Готово", "carowner-confirm-carplate"), // псевдокнопка
		// TODO: эти две кнопки взяты из резидентского блока. надо порефачить, чтобы сливать в один флоу и использовать одну кнопку
		allowContact: markup.Data("Разрешить отправку контактных данных", "chat-with-resident-allow-contact"), // TODO: псевдо-кнопка для обработчика и хранения unique
		denyContact:  markup.Data("Запретить отправку контактных данных", "chat-with-resident-deny-contact"),  // TODO: псевдо-кнопка для обработчика и хранения unique

		users: users,
	}, nil
}

func (r *CarOwnerChatter) RegisterBotsHandlers(ctx context.Context, bot HandleRegistrator) {
	_, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::RegisterBotsHandlers"))
	defer span.Close()
	bot.Handle(&r.handleInputCarPlateBtn, r.HandleInputCarPlate)
	bot.Handle(&r.confirmCarPlateBtn, r.HandleChatRequestApproved)
}

func (r *CarOwnerChatter) HandleInputCarPlate(ctx context.Context, c telebot.Context) error {
	user := repository.CurrentUserFromContext(ctx)
	if user == nil {
		return errors.New("no user in context")
	}
	var currentPlate string
	if len(c.Args()) < 1 {
		currentPlate = ""
	} else {
		currentPlate = c.Args()[0]
	}
	nextCt := cars.NextCharacterType(currentPlate)
	var rows []telebot.Row
	if nextCt.IsLatinoCyrillic() {
		var letterButtons []telebot.Btn
		for _, letter := range cars.ABCEHKMOPTXY {
			letterButtons = append(letterButtons, markup.Data(string(letter), c.Callback().Unique, currentPlate+string(letter)))
		}
		rows = append(rows, markup.Split(4, letterButtons)...)
	}
	if nextCt.IsNumber() {
		var digitButtons []telebot.Btn
		for _, letter := range "7894561230" {
			digitButtons = append(digitButtons, markup.Data(string(letter), c.Callback().Unique, currentPlate+string(letter)))
		}
		rows = append(rows, markup.Split(3, digitButtons)...)
	}
	if len(currentPlate) > 0 {
		var l = len([]rune(currentPlate))
		rows = append(rows,
			markup.Row(
				markup.Data("✖",
					c.Callback().Unique),
				markup.Data("⌫",
					c.Callback().Unique,
					string([]rune(currentPlate)[:l-1])),
			),
		)
	}
	if len(currentPlate) >= 8 {
		rows = append(rows, markup.Row(markup.Data(r.confirmCarPlateBtn.Text, r.confirmCarPlateBtn.Unique, c.Args()...)))
	}
	rows = append(rows, markup.Row(r.upperMenu))
	return c.EditOrReply(ctx, fmt.Sprintf("Ввведите номер авто[%9s]", currentPlate), markup.InlineMarkup(rows...))
}

func (r *CarOwnerChatter) HandleChatRequestApproved(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("ResidentsChatter::HandleChatRequestApproved"))
	defer span.Close()
	var vehicleLicensePlate = c.Args()[0]

	user, err := r.users.FindByVehicleLicensePlate(ctx, vehicleLicensePlate)
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf(
			"не нашел владельца [%v]: %w; %v",
			vehicleLicensePlate, err,
			c.EditOrReply(ctx, "Я не нашел автовладельца. Придется искать другим способом. Попробуйте общий чатик в разделе /chats",
				markup.InlineMarkup(markup.Row(r.upperMenu)),
			),
		)
	}
	if err != nil {
		return fmt.Errorf("ошибка поиска автовладельца [%v]: %w",
			vehicleLicensePlate, err,
		)
	}

	if _, err := c.Bot().Send(ctx, &telebot.User{ID: user.ID},
		fmt.Sprintf(
			"С вами хочет связаться %s %s (@%s). Можно ли передать ему ваши контактные данные?",
			c.Sender().FirstName, c.Sender().LastName, c.Sender().Username,
		),
		markup.InlineMarkup(
			markup.Row(
				markup.Data("❌ Нельзя", r.denyContact.Unique, fmt.Sprint(c.Sender().ID)),
				markup.Data("✅ Отправить", r.allowContact.Unique, fmt.Sprint(c.Sender().ID)),
			),
		),
	); err != nil {
		return fmt.Errorf("не отправил запрос на контакт [%d]: %w", user.ID, err)
	}

	return c.EditOrReply(ctx,
		"Спасибо. Я отправил приглашение зарегистрированым резидентам этой квартиры. Если они согласятся пообщаться, то вы получите уведомление.",
		markup.InlineMarkup(markup.Row(r.upperMenu)),
	)
}
