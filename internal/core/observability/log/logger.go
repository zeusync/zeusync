package log

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ Log = (*Logger)(nil)

var (
	innerLogger          *Logger
	loggerInitializeOnce sync.Once
)

type Logger struct {
	zapLogger *zap.Logger
	zapLevel  zapcore.Level
}

func New(level Level) *Logger {
	zapLevel := toZapLevel(level)
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		DisableCaller:    true,
	}

	zapLogger, err := config.Build()
	if err != nil {
		panic(err)
	}

	logger := &Logger{
		zapLogger: zapLogger,
		zapLevel:  zapLevel,
	}

	loggerInitializeOnce.Do(func() { innerLogger = logger })

	return logger
}

func Provide() *Logger {
	return innerLogger
}

func (l *Logger) Log(level Level, msg string, fields ...Field) {
	if !l.checkLevel(level) {
		return
	}
	l.zapLogger.Log(toZapLevel(level), msg, toZapFields(fields...)...)
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.zapLogger.Debug(msg, toZapFields(fields...)...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.zapLogger.Info(msg, toZapFields(fields...)...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.zapLogger.Warn(msg, toZapFields(fields...)...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.zapLogger.Error(msg, toZapFields(fields...)...)
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.zapLogger.Fatal(msg, toZapFields(fields...)...)
}

func (l *Logger) With(fields ...Field) Log {
	return &Logger{
		zapLogger: l.zapLogger.With(toZapFields(fields...)...),
		zapLevel:  l.zapLevel,
	}
}

func (l *Logger) WithContext(_ context.Context) Log {
	// Zap doesn't have a direct equivalent of WithContext, but you can extract values
	// from the context and add them as fields.
	// For now, we return the same innerLogger.
	return l
}

func (l *Logger) SetLevel(level Level) {
	l.zapLevel = toZapLevel(level)
}

func (l *Logger) GetLevel() Level {
	return fromZapLevel(l.zapLevel)
}

func (l *Logger) checkLevel(level Level) bool {
	return l.zapLevel.Enabled(toZapLevel(level))
}

// Helper functions to convert between levels and fields

func toZapLevel(level Level) zapcore.Level {
	switch level {
	case LevelDebug:
		return zap.DebugLevel
	case LevelInfo:
		return zap.InfoLevel
	case LevelWarn:
		return zap.WarnLevel
	case LevelError:
		return zap.ErrorLevel
	case LevelFatal:
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

func fromZapLevel(level zapcore.Level) Level {
	switch level {
	case zap.DebugLevel:
		return LevelDebug
	case zap.InfoLevel:
		return LevelInfo
	case zap.WarnLevel:
		return LevelWarn
	case zap.ErrorLevel:
		return LevelError
	case zap.FatalLevel:
		return LevelFatal
	default:
		return LevelInfo
	}
}

func toZapFields(fields ...Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		switch f.Type {
		case BoolType:
			zapFields[i] = zap.Bool(f.Key, f.Value.(bool))
		case ByteStringType:
			zapFields[i] = zap.ByteString(f.Key, f.Value.([]byte))
		case Complex128Type:
			zapFields[i] = zap.Complex128(f.Key, f.Value.(complex128))
		case Complex64Type:
			zapFields[i] = zap.Complex64(f.Key, f.Value.(complex64))
		case DurationType:
			zapFields[i] = zap.Duration(f.Key, f.Value.(time.Duration))
		case Float64Type:
			zapFields[i] = zap.Float64(f.Key, f.Value.(float64))
		case Float32Type:
			zapFields[i] = zap.Float32(f.Key, f.Value.(float32))
		case Int64Type:
			zapFields[i] = zap.Int64(f.Key, f.Value.(int64))
		case Int32Type:
			zapFields[i] = zap.Int32(f.Key, f.Value.(int32))
		case Int16Type:
			zapFields[i] = zap.Int16(f.Key, f.Value.(int16))
		case Int8Type:
			zapFields[i] = zap.Int8(f.Key, f.Value.(int8))
		case StringType:
			zapFields[i] = zap.String(f.Key, f.Value.(string))
		case Uint64Type:
			zapFields[i] = zap.Uint64(f.Key, f.Value.(uint64))
		case Uint32Type:
			zapFields[i] = zap.Uint32(f.Key, f.Value.(uint32))
		case Uint16Type:
			zapFields[i] = zap.Int16(f.Key, f.Value.(int16))
		case Uint8Type:
			zapFields[i] = zap.Int8(f.Key, f.Value.(int8))
		case UintptrType:
			zapFields[i] = zap.Uintptr(f.Key, f.Value.(uintptr))
		case ErrorType:
			zapFields[i] = zap.NamedError(f.Key, f.Value.(error))
		default:
			zapFields[i] = zap.Any(f.Key, f.Value)
		}
	}
	return zapFields
}

func toZapLevelForSlog(level slog.Level) zapcore.Level {
	switch level {
	case slog.LevelDebug:
		return zap.DebugLevel
	case slog.LevelInfo:
		return zap.InfoLevel
	case slog.LevelWarn:
		return zap.WarnLevel
	case slog.LevelError:
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func toZapFieldsFromSlog(r slog.Record) []zap.Field {
	fields := make([]zap.Field, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, slogAttrToZapField(a))
		return true
	})
	return fields
}

func toZapFieldsFromSlogAttrs(attrs []slog.Attr) []zap.Field {
	fields := make([]zap.Field, len(attrs))
	for i, a := range attrs {
		fields[i] = slogAttrToZapField(a)
	}
	return fields
}

func slogAttrToZapField(a slog.Attr) zap.Field {
	switch a.Value.Kind() {
	case slog.KindBool:
		return zap.Bool(a.Key, a.Value.Bool())
	case slog.KindDuration:
		return zap.Duration(a.Key, a.Value.Duration())
	case slog.KindFloat64:
		return zap.Float64(a.Key, a.Value.Float64())
	case slog.KindInt64:
		return zap.Int64(a.Key, a.Value.Int64())
	case slog.KindString:
		return zap.String(a.Key, a.Value.String())
	case slog.KindTime:
		return zap.Time(a.Key, a.Value.Time())
	case slog.KindUint64:
		return zap.Uint64(a.Key, a.Value.Uint64())
	case slog.KindGroup:
		return zap.Any(a.Key, a.Value.Group())
	case slog.KindLogValuer:
		return zap.Any(a.Key, a.Value.LogValuer())
	default:
		return zap.Any(a.Key, a.Value.Any())
	}
}
