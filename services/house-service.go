package services

import (
	"context"
	repositories "mikhailche/botcomod/repository"
	"time"
)

type HouseService struct {
	cache repositories.THouses
}

type houseRepo interface {
	GetHouses(ctx context.Context) (repositories.THouses, error)
}

func NewHouseService(ctx context.Context, repo houseRepo) *HouseService {
	service := HouseService{}
	delay := 1 * time.Second
	for {
		houses, err := repo.GetHouses(ctx)
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

func (h *HouseService) Houses() repositories.THouses {
	return h.cache
}
