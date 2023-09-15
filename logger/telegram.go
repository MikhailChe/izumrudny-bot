package logger

import (
	"fmt"
	"github.com/mikhailche/telebot"
	"go.uber.org/zap/zapcore"
	"html/template"
	"strings"
)

type Bot interface {
	Send(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error)
}

type telegramCore struct {
	level      zapcore.Level
	fields     []zapcore.Field
	bot        Bot
	receiverID int64
}

func (t telegramCore) Enabled(level zapcore.Level) bool {
	return t.level.Enabled(level)
}

func (t telegramCore) With(fields []zapcore.Field) zapcore.Core {
	newFields := append([]zapcore.Field{}, t.fields...)
	newFields = append(newFields, fields...)
	t.fields = newFields
	return t
}

func (t telegramCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if t.Enabled(entry.Level) {
		return ce.AddCore(entry, t)
	}
	return ce
}

var telegramMessageTmplt = template.Must(template.New("telegramMessageTmplt").
	Funcs(map[string]any{"Upper": strings.ToUpper}).
	Parse(`{{if .Entry.LoggerName}}[{{.Entry.LoggerName}}] {{end}}<b>{{Upper .Entry.Level.String}}</b> {{.Entry.Message}}
<pre>
{{range .Fields}} {{.Key}} = {{if ne .Integer 0}}{{.Integer}}{{end}}{{.String}}{{.Interface}}
{{end}}</pre>
`))

func (t telegramCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	var message strings.Builder
	allFields := append(t.fields, fields...)
	err := telegramMessageTmplt.Execute(&message, map[string]any{"Entry": entry, "Fields": allFields})
	if err != nil {
		return err
	}
	if _, err := t.bot.Send(telebot.ChatID(t.receiverID), message.String(), telebot.ModeHTML); err != nil {
		return fmt.Errorf("сообщение разработчику %v: %w", message.String(), err)
	}
	return nil
}

func (t telegramCore) Sync() error {
	return nil
}

func NewTelegramCore(level zapcore.Level, bot Bot, receiverID int64) zapcore.Core {
	return telegramCore{
		level:      level,
		fields:     nil,
		bot:        bot,
		receiverID: receiverID,
	}
}
