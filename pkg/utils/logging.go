package utils

import (
	"github.com/acaloiaro/neoq/logging"
	"go.uber.org/zap"
)

type LoggingWrapper struct {
	logging.Logger

	logger *zap.SugaredLogger
}

func (l *LoggingWrapper) Debug(msg string, args ...any) {
	l.logger.Debugw(msg, args...)
}

func (l *LoggingWrapper) Error(msg string, args ...any) {
	l.logger.Errorw(msg, args...)
}

func (l *LoggingWrapper) Info(msg string, args ...any) {
	l.logger.Infow(msg, args...)
}

func WrapLogger(logger *zap.SugaredLogger) *LoggingWrapper {
	return &LoggingWrapper{
		logger: logger,
	}
}
