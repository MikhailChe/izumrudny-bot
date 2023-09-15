package logger

import (
	"fmt"
	"mikhailche/botcomod/tracer"

	"go.uber.org/zap"
)

func New() (*zap.Logger, error) {
	defer tracer.Trace("newLogger")()

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
