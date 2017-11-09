package log

import "go.uber.org/zap"

var logger *zap.SugaredLogger = zap.NewNop().Sugar()

func RegisterLogger(l *zap.SugaredLogger) {
	logger = l
}

func Log() *zap.SugaredLogger {
	return logger
}