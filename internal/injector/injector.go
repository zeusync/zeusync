//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package injector

import (
	"github.com/google/wire"
	"github.com/zeusync/zeusync/internal/core/observability/log"
)

func ProvideLogger() *log.Logger {
	wire.Build(log.Provide)
	return log.New(log.LevelDebug)
}
