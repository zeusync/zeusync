package log

import (
	"context"
	"time"
)

type Log interface {
	Log(level Level, msg string, fields ...Field)

	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	With(fields ...Field) Log
	WithContext(ctx context.Context) Log

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

type Field struct {
	Key   string
	Type  FieldType
	Value any
}

// A FieldType indicates which member of the Field union struct should be used
// and how it should be serialized.
type FieldType uint8

const (
	UnknownType FieldType = iota
	BoolType
	ByteStringType
	Complex128Type
	Complex64Type
	DurationType
	Float64Type
	Float32Type
	IntType
	Int64Type
	Int32Type
	Int16Type
	Int8Type
	StringType
	TimeType
	TimeFullType
	UintType
	Uint64Type
	Uint32Type
	Uint16Type
	Uint8Type
	UintptrType
	ErrorType
)

func Any(key string, val any) Field {
	return Field{
		Key:   key,
		Type:  UnknownType,
		Value: val,
	}
}

func Bool(key string, val bool) Field {
	return Field{
		Key:   key,
		Type:  BoolType,
		Value: val,
	}
}

func ByteString(key string, val []byte) Field {
	return Field{
		Key:   key,
		Type:  ByteStringType,
		Value: val,
	}
}

func Complex128(key string, val complex128) Field {
	return Field{
		Key:   key,
		Type:  Complex128Type,
		Value: val,
	}
}

func Complex64(key string, val complex64) Field {
	return Field{
		Key:   key,
		Type:  Complex64Type,
		Value: val,
	}
}

func Duration(key string, val time.Duration) Field {
	return Field{
		Key:   key,
		Type:  DurationType,
		Value: val,
	}
}

func Float64(key string, val float64) Field {
	return Field{
		Key:   key,
		Type:  Float64Type,
		Value: val,
	}
}

func Float32(key string, val float32) Field {
	return Field{
		Key:   key,
		Type:  Float32Type,
		Value: val,
	}
}

func Int(key string, val int) Field {
	return Field{
		Key:   key,
		Type:  IntType,
		Value: val,
	}
}

func Int64(key string, val int64) Field {
	return Field{
		Key:   key,
		Type:  Int64Type,
		Value: val,
	}
}

func Int32(key string, val int32) Field {
	return Field{
		Key:   key,
		Type:  Int32Type,
		Value: val,
	}
}

func Int16(key string, val int16) Field {
	return Field{
		Key:   key,
		Type:  Int16Type,
		Value: val,
	}
}

func Int8(key string, val int8) Field {
	return Field{
		Key:   key,
		Type:  Int8Type,
		Value: val,
	}
}

func String(key string, val string) Field {
	return Field{
		Key:   key,
		Type:  StringType,
		Value: val,
	}
}

func Time(key string, val time.Time) Field {
	return Field{
		Key:   key,
		Type:  TimeType,
		Value: val,
	}
}

func TimeFull(key string, val time.Time) Field {
	return Field{
		Key:   key,
		Type:  TimeFullType,
		Value: val,
	}
}

func Uint(key string, val uint) Field {
	return Field{
		Key:   key,
		Type:  UintType,
		Value: val,
	}
}

func Uint64(key string, val uint64) Field {
	return Field{
		Key:   key,
		Type:  Uint64Type,
		Value: val,
	}
}

func Uint32(key string, val uint32) Field {
	return Field{
		Key:   key,
		Type:  Uint32Type,
		Value: val,
	}
}

func Uint16(key string, val uint16) Field {
	return Field{
		Key:   key,
		Type:  Uint16Type,
		Value: val,
	}
}

func Uint8(key string, val uint8) Field {
	return Field{
		Key:   key,
		Type:  Uint8Type,
		Value: val,
	}
}

func Uintptr(key string, val uintptr) Field {
	return Field{
		Key:   key,
		Type:  UintptrType,
		Value: val,
	}
}

func Error(val error) Field {
	return Field{
		Key:   "error",
		Type:  ErrorType,
		Value: val,
	}
}

func ErrorWithKey(key string, val error) Field {
	return Field{
		Key:   key,
		Type:  ErrorType,
		Value: val,
	}
}
