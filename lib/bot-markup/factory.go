package markup

import (
	"strings"

	"github.com/mikhailche/telebot"
)

func Markup() *telebot.ReplyMarkup {
	return &telebot.ReplyMarkup{}
}

func InlineMarkup(rows ...telebot.Row) *telebot.ReplyMarkup {
	m := Markup()
	m.Inline(rows...)
	return m
}

func ReplyMarkup(rows ...telebot.Row) *telebot.ReplyMarkup {
	m := Markup()
	m.Reply(rows...)
	return m
}

func Split(max int, btns []telebot.Btn) []telebot.Row {
	m := Markup()
	return m.Split(max, btns)
}

func Row(many ...telebot.Btn) telebot.Row {
	return many
}

func Text(text string) telebot.Btn {
	return telebot.Btn{Text: text}
}

func Contact(text string) telebot.Btn {
	return telebot.Btn{Contact: true, Text: text}
}

func Location(text string) telebot.Btn {
	return telebot.Btn{Location: true, Text: text}
}

func Poll(text string, poll telebot.PollType) telebot.Btn {
	return telebot.Btn{Poll: poll, Text: text}
}

func Data(text, unique string, data ...string) telebot.Btn {
	return telebot.Btn{
		Unique: unique,
		Text:   text,
		Data:   strings.Join(data, "|"),
	}
}

func URL(text, url string) telebot.Btn {
	return telebot.Btn{Text: text, URL: url}
}

func Query(text, query string) telebot.Btn {
	return telebot.Btn{Text: text, InlineQuery: query}
}

func QueryChat(text, query string) telebot.Btn {
	return telebot.Btn{Text: text, InlineQueryChat: query}
}

func Login(text string, login *telebot.Login) telebot.Btn {
	return telebot.Btn{Login: login, Text: text}
}

func WebApp(text string, app *telebot.WebApp) telebot.Btn {
	return telebot.Btn{Text: text, WebApp: app}
}
