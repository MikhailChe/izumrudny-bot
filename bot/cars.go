package bot

import (
	"context"
	"fmt"
	repositories "mikhailche/botcomod/repository"

	tele "github.com/mikhailche/telebot"
)

type carsHandler struct {
	users carsUserRepository

	upperMenu *tele.Btn

	confirmPlateMenu *tele.Btn
}

type carsUserRepository interface {
	RegisterCarLicensePlate(ctx context.Context, userID int64, event repositories.RegisterCarLicensePlateEvent) error
}

func NewCarsHandler(users carsUserRepository, upperMenu *tele.Btn) *carsHandler {
	markup := &tele.ReplyMarkup{}
	confirmPlateBtn := markup.Data("✅ Готово", "confirmlicenseplate") // для хранения unique
	return &carsHandler{users: users, upperMenu: upperMenu, confirmPlateMenu: &confirmPlateBtn}
}

func (ch *carsHandler) EntryPoint() tele.Btn {
	markup := &tele.ReplyMarkup{}
	return markup.Data("Добавить автомобиль", "add-automoibile")
}

func (ch *carsHandler) Register(bot HandleRegistrator) {
	ep := ch.EntryPoint()
	bot.Handle(&ep, ch.HandleAddCar)
	bot.Handle(ch.confirmPlateMenu, ch.ConfirmPlateHandler)
}

func needLetter(plate string) bool {
	if len(plate) == 0 || len(plate) >= 4 && len(plate) < 6 {
		return true
	}
	return false
}
func needDigit(plate string) bool {
	if len(plate) > 0 && len(plate) < 4 || len(plate) >= 6 {
		return true
	}
	return false
}

func licensePlateHints(plate string) string {
	switch len(plate) {
	case 0:
		return "Начнём с первого символа."
	case 1:
		return "Теперь 3 цифры."
	case 2:
		return "Ещё две цифры."
	case 3:
		return "И ещё одна цифра"
	case 4:
		return "Две последние буквы"
	case 5:
		return "И ещё одна"
	case 6:
		return "Теперь номер региона. 96?"
	case 7:
		if plate[6] == '7' {
			return "Москва? Питер?"
		}
	}
	return ""
}

func (ch *carsHandler) HandleAddCar(ctx context.Context, c tele.Context) error {
	var currentPlate = c.Args()[0]
	var markup = &tele.ReplyMarkup{}
	var rows []tele.Row
	if needLetter(currentPlate) {
		var letterButtons []tele.Btn
		for _, letter := range "ABCEHKMOPTXY" {
			letterButtons = append(letterButtons, markup.Data(string(letter), c.Callback().Unique, currentPlate+string(letter)))
		}
		rows = append(rows, markup.Split(4, letterButtons)...)
	}
	if needDigit(currentPlate) {
		var digitButtons []tele.Btn
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
	return c.EditOrReply(fmt.Sprintf("Введите номер своего автомобиля: %s\n%s", c.Args(), licensePlateHints(currentPlate)), markup)
}

func (ch *carsHandler) ConfirmPlateHandler(ctx context.Context, c tele.Context) error {
	if err := ch.users.RegisterCarLicensePlate(
		ctx,
		c.Sender().ID,
		repositories.RegisterCarLicensePlateEvent{UpdateID: int64(c.Update().ID), LicensePlate: c.Args()[0]},
	); err != nil {
		return fmt.Errorf("ошибка регистрации авто: %v: %w",
			c.Reply("Ошибка регистрации автомобиля. Попробуйте позже"),
			err,
		)
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(*ch.upperMenu))
	return c.EditOrReply(`Добавили ваш номер автомобиля в базу. Теперь с вами смогут связаться по нему.`)
}
