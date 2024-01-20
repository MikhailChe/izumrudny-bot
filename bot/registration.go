package bot

import (
	"context"
	"fmt"
	markup "mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/tracer.v2"
	"mikhailche/botcomod/repository"
	"strconv"
	"time"

	"github.com/mikhailche/telebot"
	"go.uber.org/zap"
)

type telegramRegistrator struct {
	log            *zap.Logger
	userRepository *repository.UserRepository
	houses         func() repository.THouses
	//buttons
	backBtn         telebot.Btn
	adminApprove    telebot.Btn
	adminDisapprove telebot.Btn
	adminFail       telebot.Btn
}

const registrationChatID = -1001860029647

func newTelegramRegistrar(log *zap.Logger, userRepository *repository.UserRepository, houses func() repository.THouses, backBtn telebot.Btn) *telegramRegistrator {
	replyMarkup := &telebot.ReplyMarkup{}
	return &telegramRegistrator{
		backBtn:         backBtn,
		log:             log,
		userRepository:  userRepository,
		houses:          houses,
		adminApprove:    replyMarkup.Data("✅ Да, кажется всё совпадает", "admin-approve-registration"),
		adminDisapprove: replyMarkup.Data("❌ Херня какая-то", "admin-disapprove-registration"),
		adminFail:       replyMarkup.Data("🔐 В топку", "admin-fail-registration"),
	}
}

func (r *telegramRegistrator) EntryPoint() *telebot.Btn {
	return &markup.RegisterBtn
}

func (r *telegramRegistrator) Register(bot HandleRegistrator) {
	bot.Handle(r.EntryPoint(), r.HandleStartRegistration)
	bot.Handle(&r.adminApprove, r.HandleAdminApprovedRegistration)
	bot.Handle(&r.adminDisapprove, r.HandleAdminDisapprovedRegistration)
	bot.Handle(&r.adminFail, r.HandleAdminFailRegistration)
}

func (r *telegramRegistrator) HandleAdminApprovedRegistration(ctx context.Context, c telebot.Context) error {
	userID, _ := strconv.Atoi(c.Args()[0])
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := r.userRepository.ConfirmRegistration(ctx, int64(userID), repository.ConfirmRegistrationEvent{
		UpdateID: int64(c.Update().ID),
		WithCode: "квитанция",
	}); err != nil {
		return fmt.Errorf("HandleAdminApprovedRegistration: %w", err)
	}
	c.EditOrReply(ctx, c.Message().Text+"\nЗавершили регистрацию")
	_, err := c.Bot().Send(ctx, &telebot.User{ID: int64(userID)}, "Регистрация завершена. Теперь вам доступен раздел для резидентов.\n/help")
	return err
}

func (r *telegramRegistrator) HandleAdminDisapprovedRegistration(ctx context.Context, c telebot.Context) error {
	userID, _ := strconv.Atoi(c.Args()[0])
	c.EditOrReply(ctx, c.Message().Text+"\nПопросили прислать заново")
	_, err := c.Bot().Send(ctx,
		&telebot.User{ID: int64(userID)},
		"Регистрация не завершена. Кажется, есть проблемы с фото. Попробуйте сделать более четкое фото. Адрес и номер квартиры должен быть читаем.")
	return err
}

func (r *telegramRegistrator) HandleAdminFailRegistration(ctx context.Context, c telebot.Context) error {
	userID, _ := strconv.Atoi(c.Args()[0])
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := r.userRepository.FailRegistration(ctx, int64(userID), repository.FailRegistrationEvent{
		UpdateID: int64(c.Update().ID),
		WithCode: "квитанция",
	}); err != nil {
		return fmt.Errorf("HandleAdminFailRegistration: %w", err)
	}
	c.EditOrReply(ctx, c.Message().Text+"\nПровалили регистрацию")
	_, err := c.Bot().Send(ctx, &telebot.User{ID: int64(userID)}, "Регистрация провалена. Квартира в квитанции не сходится с квартирой, указанной при регистрации.")
	return err
}

func (r *telegramRegistrator) HandleMediaCreated(ctx context.Context, user *repository.User, c telebot.Context) error {
	if c.Message().Photo == nil {
		return c.EditOrReply(ctx, "Для регистрации нужно отправить фото вашей квитнации за квартиру. Так мы сможем убидеться, что вы являетесь резидентом района.")
	}
	_ = c.Reply("Спасибо. Мы проверим и сообщим о результате.")
	replyMarkup := &telebot.ReplyMarkup{}
	replyMarkup.Inline(replyMarkup.Row(
		replyMarkup.Data(r.adminApprove.Text, r.adminApprove.Unique, fmt.Sprint(c.Sender().ID)),
		replyMarkup.Data(r.adminDisapprove.Text, r.adminDisapprove.Unique, fmt.Sprint(c.Sender().ID)),
		replyMarkup.Data(r.adminFail.Text, r.adminFail.Unique, fmt.Sprint(c.Sender().ID)),
	))
	if err := c.ForwardTo(&telebot.Chat{ID: registrationChatID}, replyMarkup); err != nil {
		return fmt.Errorf("HandleMediaCreated: %w", err)
	}
	return sendToRegistrationGroup(ctx, c, r.log,
		`Фото от нового пользователя: %v %v %v.
		Регистрация для адреса такой пользователь: %v %v.
		Сравни с квитанцией. Похоже?`,
		[]any{
			c.Sender().Username, c.Sender().FirstName, c.Sender().LastName,
			user.Registration.Events.Start.HouseNumber, user.Registration.Events.Start.Apartment},
		replyMarkup)
}

func (r *telegramRegistrator) HandleStartRegistration(ctx context.Context, c telebot.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("registerBtn"))
	defer span.Close()
	user, err := r.userRepository.GetUser(ctx, r.userRepository.ByID(c.Sender().ID))
	if err != nil {
		return fmt.Errorf("регистрация: %w", err)
	}
	if user.Registration != nil {
		return c.EditOrReply(ctx,
			`Регистрация уже началась. Для завершение регистрации отправьте фотографию вашей квитанции за комуналку. Так мы сможем убедиться, что вы являетесь резидентом района.`,
			markup.InlineMarkup(markup.Row(markup.BackToResidentsBtn)),
		)
	}
	data := c.Args()
	if len(data) == 0 || len(data) == 1 && data[0] == "" {
		chooseHouseMenu := &telebot.ReplyMarkup{}
		var rows []telebot.Row
		for _, house := range r.houses() {
			rows = append(rows, chooseHouseMenu.Row(chooseHouseMenu.Data(house.Number, r.EntryPoint().Unique, house.Number)))
		}
		rows = append(rows, chooseHouseMenu.Row(r.backBtn))
		chooseHouseMenu.Inline(rows...)
		return c.EditOrReply(ctx, "Выберите номер дома", chooseHouseMenu)
	}
	houseNumber := data[0]
	var house *repository.THouse
	for _, h := range r.houses() {
		if houseNumber == h.Number {
			house = &h
			break
		}
	}
	if house == nil {
		return c.EditOrReply(ctx, "Что-то пошло не по плану")
	}
	// Доступен номер дома
	if len(data) == 1 {
		chooseAppartmentRangeMenu := &telebot.ReplyMarkup{}
		var rows []telebot.Row
		for i := house.Rooms.Min; i <= house.Rooms.Max; i += 64 {
			rangeMin := i
			rangeMax := i + 63
			if rangeMax > house.Rooms.Max {
				rangeMax = house.Rooms.Max
			}
			rangeFmt := fmt.Sprintf("%d - %d", rangeMin, rangeMax)
			rows = append(rows, chooseAppartmentRangeMenu.Row(chooseAppartmentRangeMenu.Data(rangeFmt, r.EntryPoint().Unique, house.Number, fmt.Sprint(rangeMin))))
		}
		rows = append(rows, chooseAppartmentRangeMenu.Row(r.backBtn))
		chooseAppartmentRangeMenu.Inline(rows...)
		return c.EditOrReply(ctx, "🏠 Дом "+house.Number+". Выберите номер квартиры", chooseAppartmentRangeMenu)
	}
	appartmentRangeMin, err := strconv.Atoi(data[1])
	if err != nil {
		return c.EditOrReply(ctx, "Что-то пошло не по плану")
	}
	// Доступен диапазон квартир
	if len(data) == 2 {
		chooseAppartmentMenu := &telebot.ReplyMarkup{}
		var rows []telebot.Row
		var buttons []telebot.Btn

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
		return c.EditOrReply(ctx, "🏠 Дом "+house.Number+". Выберите номер квартиры", chooseAppartmentMenu)
	}
	appartmentNumber, err := strconv.Atoi(data[2])
	if err != nil {
		return c.EditOrReply(ctx, "Что-то пошло не по плану")
	}
	if len(data) == 3 {
		confirmMenu := &telebot.ReplyMarkup{}
		confirmMenu.Inline(
			confirmMenu.Row(confirmMenu.Data("✅ Да, всё верно", r.EntryPoint().Unique, house.Number, fmt.Sprint(appartmentRangeMin), fmt.Sprint(appartmentNumber), "OK")),
			confirmMenu.Row(confirmMenu.Data("❌ Неверная квартира", r.EntryPoint().Unique, house.Number)),
			confirmMenu.Row(confirmMenu.Data("❌ Неверный номер дома", r.EntryPoint().Unique)),
			confirmMenu.Row(r.backBtn),
		)
		return c.EditOrReply(ctx, fmt.Sprintf(`Давайте проверим, что всё верно.
🏠 Дом %s
🚪 Квартира %d
Всё верно?`,
			houseNumber, appartmentNumber,
		),
			confirmMenu,
		)
	}
	houseID := func() uint64 {
		for _, house := range r.houses() {
			if house.Number == houseNumber {
				return house.ID
			}
		}
		return 0
	}
	code, err := r.userRepository.StartRegistration(ctx, c.Sender().ID, int64(c.Update().ID), houseID(), houseNumber, fmt.Sprint(appartmentNumber))
	if err != nil {
		if serr := c.EditOrReply(ctx, `Извините, в процессе регистрации произошла ошибка. Исправим как можно скорее.`); serr != nil {
			return serr
		}
		return fmt.Errorf("старт регистрации: %w", err)
	}

	replyMarkup := &telebot.ReplyMarkup{}
	replyMarkup.Inline(replyMarkup.Row(r.backBtn))
	if err := c.EditOrReply(ctx, `Для завершение регистрации отправьте фотографию вашей квитанции за квартиру. Так мы сможем убедиться, что вы проживаете в квартире и являетесь резидентом района.`, replyMarkup); err != nil {
		return fmt.Errorf("отправка сообщения регистрации: %w", err)
	}
	return sendToRegistrationGroup(ctx, c, r.log, "Новая регистрация. Дом %s квартира %d. Код регистрации: %s", []any{houseNumber, appartmentNumber, code})
}

func sendToRegistrationGroup(ctx context.Context, c telebot.Context, log *zap.Logger, message string, args []any, opts ...any) error {
	ctx, span := tracer.Open(ctx, tracer.Named("sendToRegistrationGroup"))
	defer span.Close()
	log.Named("регистратор").Info(message, zap.Any("args", args))
	if _, err := c.Bot().Send(ctx, &telebot.Chat{ID: registrationChatID}, fmt.Sprintf(message, args...), opts...); err != nil {
		return fmt.Errorf("сообщение регистратору %v: %w", message, err)
	}
	return nil
}
