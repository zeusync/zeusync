package protocol

import (
	"encoding/json"
	"fmt"
)

// --- Message and Codec Implementations ---

// BasicMessage is a simple, concrete implementation of the Message interface.
type BasicMessage struct {
	MsgType    string `json:"type"`
	MsgPayload []byte `json:"payload"`
}

// NewMessage creates a new BasicMessage.
func NewMessage(msgType string, payload []byte) *BasicMessage {
	return &BasicMessage{
		MsgType:    msgType,
		MsgPayload: payload,
	}
}

// Type returns the message type.
func (m *BasicMessage) Type() string {
	return m.MsgType
}

// Payload returns the message payload.
func (m *BasicMessage) Payload() []byte {
	return m.MsgPayload
}

// JSONCodec implements the Codec interface using the JSON format.
// It's simple and human-readable, but can be replaced with a more
// performant binary codec like Protocol Buffers or MessagePack.
type JSONCodec struct{}

// Encode converts a Message into a JSON byte slice.
func (c *JSONCodec) Encode(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

// Decode converts a JSON byte slice back into a Message.
func (c *JSONCodec) Decode(data []byte) (Message, error) {
	// We need a way to know which concrete type to unmarshal to.
	// A common approach is to have a wrapper object or inspect a type field.
	var genericMsg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &genericMsg); err != nil {
		return nil, fmt.Errorf("failed to decode message type: %w", err)
	}

	// Here you could have a factory or switch statement to create the correct message type.
	// For this example, we'll just use BasicMessage for everything.
	var msg BasicMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to BasicMessage: %w", err)
	}
	return &msg, nil
}
