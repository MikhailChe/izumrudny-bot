package handlers

import (
	"context"
	"mikhailche/botcomod/lib/bot-markup"
	"mikhailche/botcomod/lib/tracer.v2"

	"github.com/mikhailche/telebot"
)

func PhonesController(mux botMux, helpMainMenuBtn *telebot.Btn, helpfulPhonesBtn *telebot.Btn) {
	phonesHandler := func(ctx context.Context, c telebot.Context) error {
		ctx, span := tracer.Open(ctx, tracer.Named("phonesHandler"))
		defer span.Close()
		const text = `👮 Охрана (круглосуточно) <b>+7-982-690-0793</b>
🚨 Аварийно-диспетчерская служба (круглосуточно) <b>+7-343-317-0798</b>

🧑‍💼👔 Управляющая компания
🌐 Общие вопросы, дополнительные услуги <b>+7-343-283-0555</b>
👨‍💼 Управляющий Дмитрий Романович <b>+7-982-655-7975</b>
👩‍💻 Специалист по работе с клиентами Ксения Валерьевна <b>+7-961-762-8049</b>
🚧 Вопросы связанные с обслуживанием: Кристина Романовна <b>+7-902-270-9252</b>
💼 Принятия заявлений, дополнительные услуги, оплата коммунальных платежей: Бухгалтер Елена Валерьевна <b>+7-908-636-3035</b>

Если здесь не хватает какого-то важного номера телефона - напишите мне об этом`
		if c.Callback() != nil {
			return c.EditOrSend(ctx, text, telebot.ModeHTML, markup.InlineMarkup(markup.Row(*helpMainMenuBtn)))
		} else {
			return c.EditOrSend(ctx, text, telebot.ModeHTML)

		}
	}
	mux.Handle(helpfulPhonesBtn, phonesHandler)
	mux.Handle("/phones", phonesHandler)
}
