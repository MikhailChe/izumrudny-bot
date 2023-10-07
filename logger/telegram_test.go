package logger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mikhailche/telebot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type mockBot struct {
	mock.Mock
}

func (m *mockBot) Send(_ context.Context, to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error) {
	argsToReturn := m.Called(to, what, opts)
	msg, ok := argsToReturn.Get(0).(*telebot.Message)
	if !ok {
		msg = nil
	}

	return msg, argsToReturn.Error(1)
}

func TestTelegramZapCore(t *testing.T) {
	bot := &mockBot{}
	bot.Test(t)
	var sentString string
	bot.On("Send", telebot.ChatID(666), mock.Anything, []any{telebot.ModeHTML}).
		Return(nil, nil).
		Run(func(args mock.Arguments) { sentString = args.String(1) }).
		Once()
	logger := zap.New(NewTelegramCore(zapcore.InfoLevel, bot, 666))
	logger.Info("Hello, world!", zap.Int("foo", 8033), zap.Error(fmt.Errorf("bar")))
	bot.AssertExpectations(t)
	assert.Contains(t, sentString, "INFO")
	assert.Contains(t, sentString, "Hello, world!")
	assert.Contains(t, sentString, "foo")
	assert.Contains(t, sentString, "8033")
	assert.Contains(t, sentString, "error")
	assert.Contains(t, sentString, "bar")
}

func TestTelegramZapCoreWithFields(t *testing.T) {
	bot := &mockBot{}
	bot.Test(t)
	var sentString string
	bot.On("Send", telebot.ChatID(666), mock.Anything, []any{telebot.ModeHTML}).
		Return(nil, nil).
		Run(func(args mock.Arguments) { sentString = args.String(1) }).
		Once()
	logger := zap.New(NewTelegramCore(zapcore.InfoLevel, bot, 666)).With(zap.String("baz", "bat"))
	logger.Info("Hello, world!", zap.Int("foo", 8033), zap.Error(fmt.Errorf("bar")))
	bot.AssertExpectations(t)
	assert.Contains(t, sentString, "INFO")
	assert.Contains(t, sentString, "Hello, world!")
	assert.Contains(t, sentString, "foo")
	assert.Contains(t, sentString, "8033")
	assert.Contains(t, sentString, "error")
	assert.Contains(t, sentString, "bar")
	assert.Contains(t, sentString, "baz")
	assert.Contains(t, sentString, "bat")
}

func TestTelegramZapCoreBotFails(t *testing.T) {
	bot := &mockBot{}
	bot.Test(t)
	bot.On("Send", telebot.ChatID(666), mock.Anything, []any{telebot.ModeHTML}).
		Return(nil, errors.New("some sort of telegram error")).
		Once()

	var buffer strings.Builder
	logger := zap.New(NewTelegramCore(zapcore.InfoLevel, bot, 666), zap.ErrorOutput(zapcore.AddSync(&buffer)))
	logger.Info("Hello, world!", zap.Int("foo", 8033), zap.Error(fmt.Errorf("bar")))
	bot.AssertExpectations(t)
	assert.NotEmpty(t, buffer.String())
}
