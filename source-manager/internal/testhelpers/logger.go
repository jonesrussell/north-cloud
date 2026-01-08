package testhelpers

import (
	"io"

	"github.com/jonesrussell/north-cloud/source-manager/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewTestLogger creates a logger suitable for testing (discards output by default)
func NewTestLogger() logger.Logger {
	// Use the NopLogger which discards all output
	return logger.NewNopLogger()
}

// NewTestLoggerWithWriter creates a logger that writes to the provided writer (useful for debugging)
func NewTestLoggerWithWriter(w io.Writer) logger.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(w),
		zapcore.DebugLevel,
	)
	z := zap.New(core, zap.AddCallerSkip(1))
	return &testLogger{logger: z}
}

type testLogger struct {
	logger *zap.Logger
}

func (l *testLogger) Debug(msg string, fields ...logger.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *testLogger) Info(msg string, fields ...logger.Field) {
	l.logger.Info(msg, fields...)
}

func (l *testLogger) Warn(msg string, fields ...logger.Field) {
	l.logger.Warn(msg, fields...)
}

func (l *testLogger) Error(msg string, fields ...logger.Field) {
	l.logger.Error(msg, fields...)
}

func (l *testLogger) With(fields ...logger.Field) logger.Logger {
	return &testLogger{logger: l.logger.With(fields...)}
}

func (l *testLogger) Sync() error {
	return l.logger.Sync()
}
