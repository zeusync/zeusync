package protocol

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol/quic"
	"github.com/zeusync/zeusync/internal/core/protocol/websocket"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketProtocol_Lifecycle(t *testing.T) {
	config := Config{
		Host:           "127.0.0.1",
		Port:           8080,
		MaxConnections: 100,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		BufferSize:     1024 * 4,
		WorkerCount:    5,
		QueueSize:      1000,
	}

	logger := log.New(log.LevelDebug)

	protocol := websocket.NewWebSocketProtocol(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test Start
	err := protocol.Start(ctx, config)
	require.NoError(t, err, "protocol should start without error")
	assert.True(t, protocol.IsRunning(), "protocol should be running")

	// Test protocol properties
	assert.Equal(t, "WebSocket", protocol.Name())
	assert.Equal(t, "1.0.0", protocol.Version())
	assert.Equal(t, WebSocket, protocol.Type())

	// Test Stop
	err = protocol.Stop(ctx)
	require.NoError(t, err, "protocol should stop without error")
	assert.False(t, protocol.IsRunning(), "protocol should not be running")
}

func TestWebSocketProtocol_MessageHandlers(t *testing.T) {
	config := Config{
		Host:        "127.0.0.1",
		Port:        8081,
		BufferSize:  4096,
		WorkerCount: 5,
		QueueSize:   1000,
	}

	logger := log.New(log.LevelDebug)

	protocol := websocket.NewWebSocketProtocol(config, logger)

	// Test handler registration
	handlerCalled := false
	testHandler := func(ctx context.Context, client ClientInfo, message IMessage) error {
		handlerCalled = true
		return nil
	}
	_ = handlerCalled

	err := protocol.RegisterHandler("test", testHandler)
	require.NoError(t, err, "should register handler without error")

	// Test duplicate handler registration
	err = protocol.RegisterHandler("test", testHandler)
	assert.Error(t, err, "should not allow duplicate handler registration")

	// Test handler retrieval
	handler, exists := protocol.GetHandler("test")
	assert.True(t, exists, "handler should exist")
	assert.NotNil(t, handler, "handler should not be nil")

	// Test handler unregistration
	err = protocol.UnregisterHandler("test")
	require.NoError(t, err, "should unregister handler without error")

	_, exists = protocol.GetHandler("test")
	assert.False(t, exists, "handler should not exist after unregistration")

	// Test default handler
	protocol.SetDefaultHandler(testHandler)
}

func TestWebSocketProtocol_Groups(t *testing.T) {
	config := Config{
		Host:        "127.0.0.1",
		Port:        8082,
		BufferSize:  4096,
		WorkerCount: 5,
		QueueSize:   1000,
	}

	logger := log.New(log.LevelDebug)

	protocol := websocket.NewWebSocketProtocol(config, logger)

	// Test group creation
	err := protocol.CreateGroup("test-group")
	require.NoError(t, err, "should create group without error")

	// Test duplicate group creation
	err = protocol.CreateGroup("test-group")
	assert.Error(t, err, "should not allow duplicate group creation")

	// Test group members (empty initially)
	members, err := protocol.GetGroupMembers("test-group")
	require.NoError(t, err, "should get group members without error")
	assert.Empty(t, members, "group should be empty initially")

	// Test group deletion
	err = protocol.DeleteGroup("test-group")
	require.NoError(t, err, "should delete group without error")

	// Test getting members of non-existent group
	_, err = protocol.GetGroupMembers("test-group")
	assert.Error(t, err, "should error when getting members of non-existent group")
}

func TestQuicProtocol_Lifecycle(t *testing.T) {
	config := Config{
		Host:           "127.0.0.1",
		Port:           9090,
		MaxConnections: 100,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		BufferSize:     4096,
		WorkerCount:    5,
		QueueSize:      1000,
		TLSEnabled:     false, // Use self-signed cert for testing
	}

	logger := log.New(log.LevelDebug)

	protocol := quic.NewQuicProtocol(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test Start
	err := protocol.Start(ctx, config)
	require.NoError(t, err, "protocol should start without error")
	assert.True(t, protocol.IsRunning(), "protocol should be running")

	// Test protocol properties
	assert.Equal(t, "QUIC", protocol.Name())
	assert.Equal(t, "1.0.0", protocol.Version())
	assert.Equal(t, Custom, protocol.Type())

	// Test Stop
	err = protocol.Stop(ctx)
	require.NoError(t, err, "protocol should stop without error")
	assert.False(t, protocol.IsRunning(), "protocol should not be running")
}

func TestQuicProtocol_MessageHandlers(t *testing.T) {
	config := Config{
		Host:        "127.0.0.1",
		Port:        9091,
		BufferSize:  4096,
		WorkerCount: 5,
		QueueSize:   1000,
		TLSEnabled:  false,
	}

	logger := log.New(log.LevelDebug)

	protocol := quic.NewQuicProtocol(config, logger)

	// Test handler registration
	handlerCalled := false
	testHandler := func(ctx context.Context, client ClientInfo, message IMessage) error {
		handlerCalled = true
		return nil
	}
	_ = handlerCalled

	err := protocol.RegisterHandler("test", testHandler)
	require.NoError(t, err, "should register handler without error")

	// Test duplicate handler registration
	err = protocol.RegisterHandler("test", testHandler)
	assert.Error(t, err, "should not allow duplicate handler registration")

	// Test handler retrieval
	handler, exists := protocol.GetHandler("test")
	assert.True(t, exists, "handler should exist")
	assert.NotNil(t, handler, "handler should not be nil")

	// Test handler unregistration
	err = protocol.UnregisterHandler("test")
	require.NoError(t, err, "should unregister handler without error")

	_, exists = protocol.GetHandler("test")
	assert.False(t, exists, "handler should not exist after unregistration")

	// Test default handler
	protocol.SetDefaultHandler(testHandler)
}

func TestMessage_Operations(t *testing.T) {
	// Test message creation
	payload := map[string]interface{}{
		"text": "Hello, World!",
		"num":  42,
	}

	message := NewMessage("test", payload)
	assert.NotEmpty(t, message.ID(), "message should have an ID")
	assert.Equal(t, "test", message.Type(), "message type should match")
	assert.Equal(t, payload, message.Payload(), "payload should match")

	// Test headers
	message.SetHeader("source", "client1")
	message.SetHeader("target", "client2")
	assert.Equal(t, "client1", message.GetHeader("source"))
	assert.Equal(t, "client2", message.GetHeader("target"))

	// Test marshaling
	data, err := message.Marshal()
	require.NoError(t, err, "should marshal without error")
	assert.NotEmpty(t, data, "marshaled data should not be empty")

	// Test unmarshalling
	newMessage := NewMessage("", nil)
	err = newMessage.Unmarshal(data)
	require.NoError(t, err, "should unmarshal without error")
	assert.Equal(t, message.ID(), newMessage.ID(), "IDs should match")
	assert.Equal(t, message.Type(), newMessage.Type(), "types should match")

	// Test cloning
	clone := message.Clone()
	assert.Equal(t, message.ID(), clone.ID(), "clone ID should match")
	assert.Equal(t, message.Type(), clone.Type(), "clone type should match")

	// Test response creation
	response := message.CreateResponse("response payload")
	assert.True(t, response.IsResponse(), "should be a response")
	assert.Equal(t, message.ID(), response.ResponseTo(), "should reference original message")

	// Test compression
	err = message.Compress()
	require.NoError(t, err, "should compress without error")

	err = message.Decompress()
	require.NoError(t, err, "should decompress without error")

	// Test priority and QoS
	message.SetPriority(PriorityHigh)
	assert.Equal(t, PriorityHigh, message.Priority())

	message.SetQoS(QoSExactlyOnce)
	assert.Equal(t, QoSExactlyOnce, message.QoS())
}

func TestMessage_Validation(t *testing.T) {
	message := NewMessage("test", "payload")

	// Test valid message
	err := message.Validate()
	assert.NoError(t, err, "valid message should pass validation")

	// Test message size limits
	// Создаем payload, который точно превысит лимит после JSON маршалинга
	largePayload := make(map[string]interface{})
	// Создаем большую строку, которая после JSON кодирования будет больше 1MB
	bigString := string(make([]byte, 1024*1024)) // 1MB строка
	largePayload["data"] = bigString

	largeMessage := NewMessage("large", largePayload, MessageOptions{
		MaxSize: 512 * 1024, // 512KB limit - меньше чем размер payload
	})

	err = largeMessage.Validate()
	assert.Error(t, err, "oversized message should fail validation")

	normalMessage := NewMessage("normal", "small payload", MessageOptions{
		MaxSize: 1024 * 1024, // 1MB limit
	})

	err = normalMessage.Validate()
	assert.NoError(t, err, "normal sized message should pass validation")
}

// Benchmark tests
func BenchmarkMessage_Marshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
	}
	message := NewMessage("benchmark", payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := message.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessage_Unmarshal(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
	}
	message := NewMessage("benchmark", payload)
	data, err := message.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newMessage := NewMessage("", nil)
		err = newMessage.Unmarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessage_Clone(b *testing.B) {
	payload := map[string]interface{}{
		"text":   "Hello, World!",
		"number": 42,
		"array":  []int{1, 2, 3, 4, 5},
	}
	message := NewMessage("benchmark", payload)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = message.Clone()
	}
}
