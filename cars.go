package main

import (
	"context"
	"fmt"

	tele "gopkg.in/telebot.v3"
)

type carRegistrator interface {
	RegisterCarRequest()
	ConfirmCarRegistration()
}

type carUserSearcher interface {
	GetUserByCar(licensePlate string)
}

type carsHandler struct {
	users carsUserRepository

	upperMenu *tele.Btn

	confirmPlateMenu *tele.Btn
}

type carsUserRepository interface {
	RegisterCarLicensePlate(ctx context.Context, userID int64, event registerCarLicensePlateEvent) error
}

func NewCarsHandller(users carsUserRepository, upperMenu *tele.Btn) *carsHandler {
	markup := &tele.ReplyMarkup{}
	confirmPlateBtn := markup.Data("✅ Готово", "confirmlicenseplate") // для хранения unique
	return &carsHandler{users: users, upperMenu: upperMenu, confirmPlateMenu: &confirmPlateBtn}
}

func (c *carsHandler) EntryPoint() tele.Btn {
	markup := &tele.ReplyMarkup{}
	return markup.Data("Добавить автомобиль", "add-automoibile")
}

func (c *carsHandler) Register(bot HandleRegistrator) {
	ep := c.EntryPoint()
	bot.Handle(&ep, c.HandleAddCar)
	bot.Handle(c.confirmPlateMenu, c.ConfirmPlateHandler)
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

func (c *carsHandler) HandleAddCar(ctx tele.Context) error {
	var currentPlate = ctx.Args()[0]
	var markup = &tele.ReplyMarkup{}
	var rows []tele.Row
	if needLetter(currentPlate) {
		var letterButtons []tele.Btn
		for _, letter := range []rune("ABCEHKMOPTXY") {
			letterButtons = append(letterButtons, markup.Data(string(letter), ctx.Callback().Unique, currentPlate+string(letter)))
		}
		rows = append(rows, markup.Split(4, letterButtons)...)
	}
	if needDigit(currentPlate) {
		var digitButtons []tele.Btn
		for _, letter := range []rune("7894561230") {
			digitButtons = append(digitButtons, markup.Data(string(letter), ctx.Callback().Unique, currentPlate+string(letter)))
		}
		rows = append(rows, markup.Split(3, digitButtons)...)
	}
	if len(currentPlate) > 0 {
		var l = len([]rune(currentPlate))
		rows = append(rows,
			markup.Row(
				markup.Data("✖",
					ctx.Callback().Unique),
				markup.Data("⌫",
					ctx.Callback().Unique,
					string([]rune(currentPlate)[:l-1])),
			),
		)
	}
	if len(currentPlate) >= 8 {
		rows = append(rows, markup.Row(markup.Data("✅ Готово", c.confirmPlateMenu.Unique, currentPlate)))
	}
	rows = append(rows, markup.Row(*c.upperMenu))
	markup.Inline(rows...)
	return ctx.EditOrReply(fmt.Sprintf("Введите номер своего автомобиля: %s\n%s", ctx.Args(), licensePlateHints(currentPlate)), markup)
}

func (c *carsHandler) ConfirmPlateHandler(ctx tele.Context) error {
	if err := c.users.RegisterCarLicensePlate(context.Background(), ctx.Sender().ID, registerCarLicensePlateEvent{int64(ctx.Update().ID), ctx.Args()[0]}); err != nil {
		return fmt.Errorf("ошибка регистрации авто: %v: %w",
			ctx.Reply("Ошибка регистрации автомобиля. Попробуйте позже"),
			err,
		)
	}
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(*c.upperMenu))
	return ctx.EditOrReply(fmt.Sprintf(`Добавили ваш номер автомобиля в базу. Теперь с вами смогут связаться по нему.`))
}
