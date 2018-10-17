// Copyright © 2018 Kowala SEZC <info@kowala.tech>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import "go.uber.org/zap"

var defaultLogger *zap.Logger

func init() {
	/*
		lvl := new(zapcore.Level)
		if err := lvl.Set(verbosity); err != nil {
			return err
		}
	*/
	logger, err := zap.NewProduction(zap.AddStacktrace(zap.InfoLevel), zap.AddCaller())
	if err != nil {
		panic(err)
	}
	defaultLogger = logger
}

// Debug is a convenient alias for defaultLogger.Debug
func Debug(msg string, fields ...zap.Field) {
	defaultLogger.Debug(msg, fields...)
}

// Info is a convenient alias for defaultLogger.Info
func Info(msg string, fields ...zap.Field) {
	defaultLogger.Info(msg, fields...)
}

// Error is a convenient alias for defaultLogger.Error
func Error(msg string, fields ...zap.Field) {
	defaultLogger.Error(msg, fields...)
}

// DPanic is a convenient alias for defaultLogger.DPanic
func DPanic(msg string, fields ...zap.Field) {
	defaultLogger.DPanic(msg, fields...)
}

// Panic is a convenient alias for defaultLogger.Panic
func Panic(msg string, fields ...zap.Field) {
	defaultLogger.Panic(msg, fields...)
}

// Fatal is a convenient alias for defaultLogger.Fatal
func Fatal(msg string, fields ...zap.Field) {
	defaultLogger.Fatal(msg, fields...)
}
