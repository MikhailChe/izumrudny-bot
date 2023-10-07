package main

import (
	"context"
	"mikhailche/botcomod/logger"
	"mikhailche/botcomod/repository"
	"mikhailche/botcomod/repository/ydb"

	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	log, err := logger.New(ctx)
	if err != nil {
		panic(err)
	}
	ydbd, err := ydb.NewYDBDriver(ctx, log)
	if err != nil {
		panic(err)
	}
	houses := repository.HouseRepository{DB: ydbd}
	hh, err := houses.GetHouses(ctx)
	if err != nil {
		panic(err)
	}
	log.Info("Дома", zap.Any("hh", hh))
}
