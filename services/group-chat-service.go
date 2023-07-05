package services

import (
	"context"
	"mikhailche/botcomod/repositories"
	"time"
)

type GroupChatService struct {
	cache repositories.TGroupChats
}

type getGroupChatRepo interface {
	GetGroupChats(ctx context.Context) (repositories.TGroupChats, error)
}

func NewGroupChatService(repo getGroupChatRepo) *GroupChatService {
	service := GroupChatService{}
	ctx := context.Background()
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
