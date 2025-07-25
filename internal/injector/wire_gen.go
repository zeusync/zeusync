// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package injector

import (
	"github.com/zeusync/zeusync/internal/core/observability/log"
)

// Injectors from injector.go:

func ProvideLogger() *log.Logger {
	logger := log.Provide()
	return logger
}
