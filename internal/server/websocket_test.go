package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketChat(t *testing.T) {
	chatHistory := NewChatHistory()
	authMiddleware := &SimpleAuthMiddleware{}
	server := NewWebSocketServer(nil, chatHistory, authMiddleware)

	s := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Test without token
	_, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		t.Fatalf("Expected error when connecting without token")
	}

	// Test with invalid token
	_, _, err = websocket.DefaultDialer.Dial(u+"?token=invalid", nil)
	if err == nil {
		t.Fatalf("Expected error when connecting with invalid token")
	}

	// Test with valid token
	conn, _, err := websocket.DefaultDialer.Dial(u+"?token=supersecrettoken", nil)
	if err != nil {
		t.Fatalf("Could not connect with valid token: %v", err)
	}
	defer conn.Close()

	// Test sending and receiving messages
	msg := ChatMessage{Sender: "test", Message: "hello"}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("Could not send message: %v", err)
	}

	var receivedMsg ChatMessage
	if err := conn.ReadJSON(&receivedMsg); err != nil {
		t.Fatalf("Could not read message: %v", err)
	}

	if receivedMsg.Message != msg.Message {
		t.Errorf("Expected message '%s', got '%s'", msg.Message, receivedMsg.Message)
	}
}
