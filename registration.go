package main

import (
	"context"
	"fmt"
	"mikhailche/botcomod/repositories"
	. "mikhailche/botcomod/tracer"
	"strconv"
	"time"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type telegramRegistrator struct {
	log            *zap.Logger
	userRepository *UserRepository
	houses         func() repositories.THouses
	//buttons
	backBtn         tele.Btn
	adminApprove    tele.Btn
	adminDisapprove tele.Btn
	adminFail       tele.Btn
}

const registrationChatID = -1001860029647

func newTelegramRegistrator(log *zap.Logger, userRepository *UserRepository, houses func() repositories.THouses, backBtn tele.Btn) *telegramRegistrator {
	markup := &tele.ReplyMarkup{}
	return &telegramRegistrator{
		backBtn:         backBtn,
		log:             log,
		userRepository:  userRepository,
		houses:          houses,
		adminApprove:    markup.Data("✅ Да, кажется всё совпадает", "admin-approve-registration"),
		adminDisapprove: markup.Data("❌ Херня какая-то", "admin-disapprove-registration"),
		adminFail:       markup.Data("🔐 В топку", "admin-fail-registration"),
	}
}

func (r *telegramRegistrator) EntryPoint() *tele.Btn {
	markup := &tele.ReplyMarkup{}
	e := markup.Data("📒 Начать регистрацию", "registration")
	return &e
}

func (r *telegramRegistrator) Register(bot HandleRegistrator) {
	bot.Handle(r.EntryPoint(), r.HandleStartRegistration)
	bot.Handle(&r.adminApprove, r.HandleAdminApprovedRegistration)
	bot.Handle(&r.adminDisapprove, r.HandleAdminDisapprovedRegistration)
	bot.Handle(&r.adminFail, r.HandleAdminFailRegistration)
}

func (r *telegramRegistrator) HandleAdminApprovedRegistration(ctx tele.Context) error {
	userID, _ := strconv.Atoi(ctx.Args()[0])
	stdctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.userRepository.ConfirmRegistration(stdctx, int64(userID), confirmRegistrationEvent{
		UpdateID: int64(ctx.Update().ID),
		WithCode: "квитанция",
	}); err != nil {
		return fmt.Errorf("HandleAdminApprovedRegistration: %w", err)
	}
	ctx.EditOrReply(ctx.Message().Text + "\nЗавершили регистрацию")
	_, err := ctx.Bot().Send(&tele.User{ID: int64(userID)}, "Регистрация завершена. Теперь вам доступен раздел для резидентов.\n/help")
	return err
}

func (r *telegramRegistrator) HandleAdminDisapprovedRegistration(ctx tele.Context) error {
	userID, _ := strconv.Atoi(ctx.Args()[0])
	ctx.EditOrReply(ctx.Message().Text + "\nПопросили прислать заново")
	_, err := ctx.Bot().Send(
		&tele.User{ID: int64(userID)},
		"Регистрация не завершена. Кажется, есть проблемы с фото. Попробуйте сделать более четкое фото. Адрес и номер квартиры должен быть читаем.")
	return err
}

func (r *telegramRegistrator) HandleAdminFailRegistration(ctx tele.Context) error {
	userID, _ := strconv.Atoi(ctx.Args()[0])
	stdctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.userRepository.FailRegistration(stdctx, int64(userID), failRegistrationEvent{
		UpdateID: int64(ctx.Update().ID),
		WithCode: "квитанция",
	}); err != nil {
		return fmt.Errorf("HandleAdminFailRegistration: %w", err)
	}
	ctx.EditOrReply(ctx.Message().Text + "\nПровалили регистрацию")
	_, err := ctx.Bot().Send(&tele.User{ID: int64(userID)}, "Регистрация провалена. Квартира в квитанции не сходится с квартирой, указанной при регистрации.")
	return err
}

func (r *telegramRegistrator) HandleMediaCreated(user *User, ctx tele.Context) error {
	if ctx.Message().Photo == nil {
		return ctx.EditOrReply("Для регистрации нужно отправить фото вашей квитнации за квартиру. Так мы сможем убидеться, что вы являетесь резидентом района.")
	}
	ctx.Reply("Спасибо. Мы проверим и сообщим о результате.")
	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(
		markup.Data(r.adminApprove.Text, r.adminApprove.Unique, fmt.Sprint(ctx.Sender().ID)),
		markup.Data(r.adminDisapprove.Text, r.adminDisapprove.Unique, fmt.Sprint(ctx.Sender().ID)),
		markup.Data(r.adminFail.Text, r.adminFail.Unique, fmt.Sprint(ctx.Sender().ID)),
	))
	if err := ctx.ForwardTo(&tele.Chat{ID: registrationChatID}, markup); err != nil {
		return fmt.Errorf("HandleMediaCreated: %w", err)
	}
	return sendToRegistrationGroup(ctx, r.log,
		`Фото от нового пользователя: %v %v %v.
		Регистрация для адреса такой пользователь: %v %v.
		Сравни с квитанцией. Похоже?`,
		[]any{
			ctx.Sender().Username, ctx.Sender().FirstName, ctx.Sender().LastName,
			user.Registration.Events.Start.HouseNumber, user.Registration.Events.Start.Appartment},
		markup)
}

func (r *telegramRegistrator) HandleStartRegistration(ctx tele.Context) error {
	defer Trace("registerBtn")()
	stdctx := context.Background()
	user, err := r.userRepository.GetById(stdctx, ctx.Sender().ID)
	if err != nil {
		return fmt.Errorf("регистрация: %w", err)
	}
	if user.Registration != nil {
		return ctx.EditOrReply(`Регистрация уже началась. Для завершение регистрации отправьте фотографию вашей квитанции за квартиру. Так мы сможем убедиться, что вы являетесь резидентом района.`)
	}
	data := ctx.Args()
	if len(data) == 0 || len(data) == 1 && data[0] == "" {
		chooseHouseMenu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, house := range r.houses() {
			rows = append(rows, chooseHouseMenu.Row(chooseHouseMenu.Data(house.Number, r.EntryPoint().Unique, house.Number)))
		}
		rows = append(rows, chooseHouseMenu.Row(r.backBtn))
		chooseHouseMenu.Inline(rows...)
		return ctx.EditOrReply("Выберите номер дома", chooseHouseMenu)
	}
	houseNumber := data[0]
	var house *repositories.THouse
	for _, h := range r.houses() {
		if houseNumber == h.Number {
			house = &h
			break
		}
	}
	if house == nil {
		return ctx.EditOrReply("Что-то пошло не по плану")
	}
	// Доступен номер дома
	if len(data) == 1 {
		chooseAppartmentRangeMenu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for i := house.Rooms.Min; i <= house.Rooms.Max; i += 64 {
			range_min := i
			range_max := i + 63
			if range_max > house.Rooms.Max {
				range_max = house.Rooms.Max
			}
			rangeFmt := fmt.Sprintf("%d - %d", range_min, range_max)
			rows = append(rows, chooseAppartmentRangeMenu.Row(chooseAppartmentRangeMenu.Data(rangeFmt, r.EntryPoint().Unique, house.Number, fmt.Sprint(range_min))))
		}
		rows = append(rows, chooseAppartmentRangeMenu.Row(r.backBtn))
		chooseAppartmentRangeMenu.Inline(rows...)
		return ctx.EditOrReply("🏠 Дом "+house.Number+". Выберите номер квартиры", chooseAppartmentRangeMenu)
	}
	appartmentRangeMin, err := strconv.Atoi(data[1])
	if err != nil {
		return ctx.EditOrReply("Что-то пошло не по плану")
	}
	// Доступен диапазон квартир
	if len(data) == 2 {
		chooseAppartmentMenu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var buttons []tele.Btn

		for i := appartmentRangeMin; i <= appartmentRangeMin+65 && i <= house.Rooms.Max; i++ {
			buttons = append(buttons, chooseAppartmentMenu.Data(
				fmt.Sprint(i),
				r.EntryPoint().Unique, house.Number, fmt.Sprint(appartmentRangeMin), fmt.Sprint(i)))
			if i%8 == 0 {
				rows = append(rows, chooseAppartmentMenu.Row(buttons...))
				buttons = nil
			}
		}
		if len(buttons) > 0 {
			rows = append(rows, chooseAppartmentMenu.Row(buttons...))
			buttons = nil
		}
		rows = append(rows, chooseAppartmentMenu.Row(r.backBtn))
		chooseAppartmentMenu.Inline(rows...)
		return ctx.EditOrReply("🏠 Дом "+house.Number+". Выберите номер квартиры", chooseAppartmentMenu)
	}
	appartmentNumber, err := strconv.Atoi(data[2])
	if err != nil {
		return ctx.EditOrReply("Что-то пошло не по плану")
	}
	if len(data) == 3 {
		confirmMenu := &tele.ReplyMarkup{}
		confirmMenu.Inline(
			confirmMenu.Row(confirmMenu.Data("✅ Да, всё верно", r.EntryPoint().Unique, house.Number, fmt.Sprint(appartmentRangeMin), fmt.Sprint(appartmentNumber), fmt.Sprint("OK"))),
			confirmMenu.Row(confirmMenu.Data("❌ Неверная квартира", r.EntryPoint().Unique, house.Number)),
			confirmMenu.Row(confirmMenu.Data("❌ Неверный номер дома", r.EntryPoint().Unique)),
			confirmMenu.Row(r.backBtn),
		)
		return ctx.EditOrReply(fmt.Sprintf(`Давайте проверим, что всё верно.
🏠 Дом %s
🚪 Квартира %d
Всё верно?`,
			houseNumber, appartmentNumber,
		),
			confirmMenu,
		)
	}
	code, err := r.userRepository.StartRegistration(context.Background(), ctx.Sender().ID, int64(ctx.Update().ID), houseNumber, fmt.Sprint(appartmentNumber))
	if err != nil {
		if serr := ctx.EditOrReply(`Извините, в процессе регистрации произошла ошибка. Исправим как можно скорее.`); serr != nil {
			return serr
		}
		return fmt.Errorf("старт регистрации: %w", err)
	}

	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(r.backBtn))
	if err := ctx.EditOrReply(`Для завершение регистрации отправьте фотографию вашей квитанции за квартиру. Так мы сможем убедиться, что вы проживаете в квартире и являетесь резидентом района.`, markup); err != nil {
		return fmt.Errorf("отправка сообщения регистрации: %w", err)
	}
	return sendToRegistrationGroup(ctx, r.log, "Новая регистрация. Дом %s квартира %d. Код регистрации: %s", []any{houseNumber, appartmentNumber, code})
}

func sendToRegistrationGroup(ctx tele.Context, log *zap.Logger, message string, args []any, opts ...any) error {
	defer Trace("sendToRegistrationGroup")()
	log.Named("регистратор").Info(message, zap.Any("args", args))
	if _, err := ctx.Bot().Send(&tele.Chat{ID: registrationChatID}, fmt.Sprintf(message, args...), opts...); err != nil {
		return fmt.Errorf("сообщение регистратору %v: %w", message, err)
	}
	return nil
}
