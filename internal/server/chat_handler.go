package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"

	"github.com/zeusync/zeusync/internal/core/protocol/intrefaces"
)

type ChatMessage struct {
	Sender  string `json:"sender"`
	Message string `json:"message"`
	UserID  string `json:"userID"`
}

func (s *HTTPServer) handleHTTPChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var msg ChatMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		history, _ := s.chatHistory.Get(context.Background())
		newHistory := append(history, msg)
		s.chatHistory.Set(context.Background(), newHistory)

		b, err := json.Marshal(msg)
		if err != nil {
			// TODO: log error
			return
		}

		s.mu.Lock()
		for _, client := range s.clients {
			client <- b
		}
		s.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		client := make(chan []byte)
		s.mu.Lock()
		s.clients[r.RemoteAddr] = client
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			delete(s.clients, r.RemoteAddr)
			s.mu.Unlock()
		}()

		history, _ := s.chatHistory.Get(context.Background())
		for _, msg := range history {
			b, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}

		for {
			select {
			case msg := <-client:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *WebSocketServer) handleWebSocketChat(conn *websocket.Conn, clientInfo intrefaces.ClientInfo, room *Room) {
	defer func() {
		room.mu.Lock()
		delete(room.clients, conn)
		room.mu.Unlock()
		conn.Close()
	}()

	history, _ := s.chatHistory.Get(context.Background())
	for _, msg := range history {
		if err := conn.WriteJSON(msg); err != nil {
			// TODO: log error
			return
		}
	}

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			// TODO: log error
			return
		}

		var msg ChatMessage
		if err := json.Unmarshal(p, &msg); err != nil {
			// TODO: log error
			continue
		}
		msg.UserID = clientInfo.UserID

		history, _ := s.chatHistory.Get(context.Background())
		newHistory := append(history, msg)
		s.chatHistory.Set(context.Background(), newHistory)

		room.mu.Lock()
		for client := range room.clients {
			if err := client.WriteJSON(msg); err != nil {
				// TODO: log error
				client.Close()
				delete(room.clients, client)
			}
		}
		room.mu.Unlock()
	}
}
