package services

import (
	"context"
	"mikhailche/botcomod/repository"
	"time"
)

type GroupChatService struct {
	cache repository.TGroupChats
	repo  getGroupChatRepo
}

type getGroupChatRepo interface {
	GetGroupChats(ctx context.Context) (repository.TGroupChats, error)
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

func (h *GroupChatService) GroupChats() repository.TGroupChats {
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
