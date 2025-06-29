package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
)

type HTTPServer struct {
	server      *http.Server
	proto       intrefaces.Protocol
	clients     map[string]chan []byte
	mu          sync.Mutex
	chatHistory *ChatHistory
}

func NewHTTPServer(proto intrefaces.Protocol, chatHistory *ChatHistory) *HTTPServer {
	return &HTTPServer{
		proto:       proto,
		clients:     make(map[string]chan []byte),
		chatHistory: chatHistory,
	}
}

func (s *HTTPServer) Start(ctx context.Context, config intrefaces.ProtocolConfig) error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: s,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// TODO: log error
		}
	}()

	return nil
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/chat" {
		s.handleHTTPChat(w, r)
		return
	}

	http.NotFound(w, r)
}
