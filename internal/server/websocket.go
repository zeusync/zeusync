package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Room struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

type WebSocketServer struct {
	proto          intrefaces.Protocol
	rooms          map[string]*Room
	mu             sync.Mutex
	chatHistory    *ChatHistory
	authMiddleware intrefaces.ProtocolMiddleware
}

func NewWebSocketServer(proto intrefaces.Protocol, chatHistory *ChatHistory, authMiddleware intrefaces.ProtocolMiddleware) *WebSocketServer {
	return &WebSocketServer{
		proto:          proto,
		rooms:          make(map[string]*Room),
		chatHistory:    chatHistory,
		authMiddleware: authMiddleware,
	}
}

func (s *WebSocketServer) Start(ctx context.Context, config intrefaces.ProtocolConfig) error {
	go func() {
		http.HandleFunc("/ws", s.handleWebSocket)
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil); err != nil {
			// TODO: log error
		}
	}()
	return nil
}

func (s *WebSocketServer) Stop(ctx context.Context) error {
	// TODO: Implement graceful shutdown
	return nil
}

func (s *WebSocketServer) getOrCreateRoom(roomID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, exists := s.rooms[roomID]; exists {
		return room
	}

	room := &Room{
		clients: make(map[*websocket.Conn]bool),
	}
	s.rooms[roomID] = room
	return room
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("roomID")
	if roomID == "" {
		roomID = "general"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// TODO: log error
		return
	}

	clientInfo := intrefaces.ClientInfo{
		ID:            conn.RemoteAddr().String(),
		RemoteAddress: conn.RemoteAddr().String(),
		Metadata:      make(map[string]any),
	}

	// Extract token from query params for WebSocket
	token := r.URL.Query().Get("token")
	clientInfo.Metadata["token"] = token

	if s.authMiddleware != nil {
		if err := s.authMiddleware.OnConnect(context.Background(), clientInfo); err != nil {
			// TODO: log error
			conn.Close()
			return
		}
	}

	room := s.getOrCreateRoom(roomID)
	room.mu.Lock()
	room.clients[conn] = true
	room.mu.Unlock()

	s.handleWebSocketChat(conn, clientInfo, room)
}
