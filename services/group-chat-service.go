package services

import (
	"context"
	repositories "mikhailche/botcomod/repository"
	"time"
)

type GroupChatService struct {
	cache repositories.TGroupChats
	repo  getGroupChatRepo
}

type getGroupChatRepo interface {
	GetGroupChats(ctx context.Context) (repositories.TGroupChats, error)
	UpdateChatByTelegramId(
		ctx context.Context,
		telegramChatID int64,
		telegramChatTitle string,
		telegramChatType string,
	) error
}

func NewGroupChatService(ctx context.Context, repo getGroupChatRepo) *GroupChatService {
	service := GroupChatService{
		repo: repo,
	}
	delay := 1 * time.Second
	for {
		houses, err := repo.GetGroupChats(ctx)
		if err != nil {
			delay += 1 * time.Second
			time.Sleep(delay)
			continue
		}
		service.cache = houses
		break
	}
	return &service
}

func (h *GroupChatService) GroupChats() repositories.TGroupChats {
	return h.cache
}

func (h *GroupChatService) IsAntiObsceneEnabled(chatID int64) bool {
	for _, chat := range h.GroupChats() {
		if chat.TelegramChatID == chatID {
			return chat.AntiObscene
		}
	}
	return false
}

func (h *GroupChatService) UpdateChatByTelegramId(
	ctx context.Context,
	telegramChatID int64,
	telegramChatTitle string,
	telegramChatType string,
) error {
	return h.repo.UpdateChatByTelegramId(ctx, telegramChatID, telegramChatTitle, telegramChatType)
}
