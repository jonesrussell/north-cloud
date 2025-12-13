package logger

import (
	"time"

	"go.uber.org/zap"
)

func String(key, val string) Field {
	return zap.String(key, val)
}

func Int(key string, val int) Field {
	return zap.Int(key, val)
}

func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}

func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}

func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

func Error(err error) Field {
	return zap.Error(err)
}

func Any(key string, val any) Field {
	return zap.Any(key, val)
}

func Strings(key string, val []string) Field {
	return zap.Strings(key, val)
}
