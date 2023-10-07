package bot

import (
	"context"
	"fmt"
	"mikhailche/botcomod/lib/cars"
	"mikhailche/botcomod/repository"

	"github.com/mikhailche/telebot"
)

type carsHandler struct {
	users carsUserRepository

	upperMenu *telebot.Btn

	confirmPlateMenu *telebot.Btn
}

type carsUserRepository interface {
	RegisterCarLicensePlate(ctx context.Context, userID int64, event repository.RegisterCarLicensePlateEvent) error
}

func NewCarsHandler(users carsUserRepository, upperMenu *telebot.Btn) *carsHandler {
	markup := &telebot.ReplyMarkup{}
	confirmPlateBtn := markup.Data("✅ Готово", "confirmlicenseplate") // для хранения unique
	return &carsHandler{users: users, upperMenu: upperMenu, confirmPlateMenu: &confirmPlateBtn}
}

func (ch *carsHandler) EntryPoint() telebot.Btn {
	markup := &telebot.ReplyMarkup{}
	return markup.Data("Добавить автомобиль", "add-automoibile")
}

func (ch *carsHandler) Register(bot HandleRegistrator) {
	ep := ch.EntryPoint()
	bot.Handle(&ep, ch.HandleAddCar)
	bot.Handle(ch.confirmPlateMenu, ch.ConfirmPlateHandler)
}

func (ch *carsHandler) HandleAddCar(ctx context.Context, c telebot.Context) error {
	var currentPlate = c.Args()[0]
	var markup = &telebot.ReplyMarkup{}
	var rows []telebot.Row
	nextCt := cars.NextCharacterType(currentPlate)
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
		rows = append(rows, markup.Row(markup.Data("✅ Готово", ch.confirmPlateMenu.Unique, currentPlate)))
	}
	rows = append(rows, markup.Row(*ch.upperMenu))
	markup.Inline(rows...)
	return c.EditOrReply(ctx, fmt.Sprintf("Введите номер своего автомобиля: %s\n%s", c.Args(), cars.LicensePlateHints(currentPlate)), markup)
}

func (ch *carsHandler) ConfirmPlateHandler(ctx context.Context, c telebot.Context) error {
	if err := ch.users.RegisterCarLicensePlate(
		ctx,
		c.Sender().ID,
		repository.RegisterCarLicensePlateEvent{UpdateID: int64(c.Update().ID), LicensePlate: c.Args()[0]},
	); err != nil {
		return fmt.Errorf("ошибка регистрации авто: %v: %w",
			c.Reply("Ошибка регистрации автомобиля. Попробуйте позже"),
			err,
		)
	}
	markup := &telebot.ReplyMarkup{}
	markup.Inline(markup.Row(*ch.upperMenu))
	return c.EditOrReply(ctx, `Добавили ваш номер автомобиля в базу. Теперь с вами смогут связаться по нему.`)
}
