package services

import (
	"strings"

	"go.uber.org/zap"
)

type obsceneFilter struct {
	log *zap.Logger
}

func NewObsceneFilter(log *zap.Logger) *obsceneFilter {
	return &obsceneFilter{log: log}
}

func (f *obsceneFilter) DetectObsceneLanguage(in string) bool {
	in = strings.ToLower(in)
	return strings.Contains(in, "сука")
}
