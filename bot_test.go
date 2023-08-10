package main_test

import (
	main "mikhailche/botcomod"
	"mikhailche/botcomod/logger"
	"testing"
)

func TestBotAnswersToUser(t *testing.T) {
	log, err := logger.ForTests()
	if err != nil {
		t.FailNow()
	}

	userRepository
	
	bot, err := main.NewBot(log, userRepository, houseService.Houses, groupChatService.GroupChats)

}
