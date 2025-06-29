package server

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
)

type Server struct {
	ctx    context.Context
	logger *log.Logger
}

func NewServer(globalCtx context.Context) *Server {
	logger := log.New(log.LevelDebug)

	return &Server{
		ctx:    globalCtx,
		logger: logger,
	}
}

func (s *Server) Start(_ context.Context) error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
