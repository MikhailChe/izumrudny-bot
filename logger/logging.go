package logger

import (
	"context"
	"fmt"
	"mikhailche/botcomod/lib/tracer.v2"

	"go.uber.org/zap"
)

func New(ctx context.Context) (*zap.Logger, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("newLogger"))
	defer span.Close()

	zapConfig := zap.NewProductionConfig()
	zapConfig.DisableCaller = false
	zapConfig.Level.SetLevel(zap.DebugLevel)
	log, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("newLogger: %w", err)
	}
	return log, nil
}

func ForTests() (*zap.Logger, error) {
	return zap.NewDevelopment()
}
