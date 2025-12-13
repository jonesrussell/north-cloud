package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines the interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Sync() error
}

type zapLogger struct {
	logger *zap.Logger
}

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, fields...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, fields...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		logger: l.logger.With(fields...),
	}
}

func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

func NewLogger(debug bool) (Logger, error) {
	var z *zap.Logger
	var err error

	if debug {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		config.Encoding = "console"
		config.Development = true
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.Sampling = nil

		z, err = config.Build(
			zap.AddCallerSkip(0),
			zap.AddStacktrace(zapcore.WarnLevel),
		)
	} else {
		z, err = zap.NewProduction()
	}

	if err != nil {
		return nil, err
	}

	return &zapLogger{
		logger: z,
	}, nil
}

func NewNopLogger() Logger {
	return &zapLogger{
		logger: zap.NewNop(),
	}
}

type Field = zapcore.Field
