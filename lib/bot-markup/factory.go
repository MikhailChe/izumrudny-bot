package markup

import (
	"strings"

	tele "gopkg.in/telebot.v3"
)

func Markup() *tele.ReplyMarkup {
	return &tele.ReplyMarkup{}
}

func InlineMarkup(rows ...tele.Row) *tele.ReplyMarkup {
	m := Markup()
	m.Inline(rows...)
	return m
}

func ReplyMarkup(rows ...tele.Row) *tele.ReplyMarkup {
	m := Markup()
	m.Reply(rows...)
	return m
}

func Row(many ...tele.Btn) tele.Row {
	return many
}

func Text(text string) tele.Btn {
	return tele.Btn{Text: text}
}

func Contact(text string) tele.Btn {
	return tele.Btn{Contact: true, Text: text}
}

func Location(text string) tele.Btn {
	return tele.Btn{Location: true, Text: text}
}

func Poll(text string, poll tele.PollType) tele.Btn {
	return tele.Btn{Poll: poll, Text: text}
}

func Data(text, unique string, data ...string) tele.Btn {
	return tele.Btn{
		Unique: unique,
		Text:   text,
		Data:   strings.Join(data, "|"),
	}
}

func URL(text, url string) tele.Btn {
	return tele.Btn{Text: text, URL: url}
}

func Query(text, query string) tele.Btn {
	return tele.Btn{Text: text, InlineQuery: query}
}

func QueryChat(text, query string) tele.Btn {
	return tele.Btn{Text: text, InlineQueryChat: query}
}

func Login(text string, login *tele.Login) tele.Btn {
	return tele.Btn{Login: login, Text: text}
}

func WebApp(text string, app *tele.WebApp) tele.Btn {
	return tele.Btn{Text: text, WebApp: app}
}
