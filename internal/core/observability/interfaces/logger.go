package interfaces

import "context"

type Logger interface {
	Log(level Level, msg string, fields ...Filed)

	Debug(msg string, fields ...Filed)
	Info(msg string, fields ...Filed)
	Warn(msg string, fields ...Filed)
	Error(msg string, fields ...Filed)
	Fatal(msg string, fields ...Filed)

	With(fields ...Filed) Logger
	WithContext(ctx context.Context) Logger

	SetLevel(level Level)
	GetLevel() Level
}

type Level uint8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelTrace  Level = 100
	LevelSilent Level = 101
	LevelNone   Level = 0xFF
)

type Filed struct {
	Key   string
	Value any
}
