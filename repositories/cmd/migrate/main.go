package main

import (
	"context"
	"mikhailche/botcomod/logger"
	"mikhailche/botcomod/repositories"
	"mikhailche/botcomod/repositories/ydb"

	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	ydbd, err := ydb.NewYDBDriver(ctx, log)
	if err != nil {
		panic(err)
	}
	houses := repositories.HouseRepository{DB: ydbd}
	hh, err := houses.GetHouses(ctx)
	if err != nil {
		panic(err)
	}
	log.Info("Дома", zap.Any("hh", hh))
}
