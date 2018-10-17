package log

import "go.uber.org/zap"

var (
	Logger, _ = zap.NewProduction()
)
