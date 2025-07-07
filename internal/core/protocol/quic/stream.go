package quic

import (
	"context"
	"encoding/binary"
	"io"
	"sync/atomic"
	"time"

	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"

	"github.com/quic-go/quic-go"
)

// Stream implements the Stream interface for QUIC streams
type Stream struct {
	*protocol.BaseStream
	stream *quic.Stream
	closed int32 // atomic bool
	logger log.Log
}

// NewQUICStream creates a new QUIC stream
func NewQUICStream(stream *quic.Stream, connectionID protocol.ConnectionID, streamType protocol.StreamType, config protocol.StreamConfig, logger log.Log) *Stream {
	id := protocol.GenerateStreamID()

	if logger == nil {
		logger = log.Provide()
	}

	baseStream := protocol.NewBaseStream(id, connectionID, streamType, config, logger)

	qstream := &Stream{
		BaseStream: baseStream,
		stream:     stream,
		logger:     logger.With(log.String("quic_stream_id", stream.StreamID().InitiatedBy().String())),
	}

	qstream.logger.Debug("QUIC stream created",
		log.String("quic_stream_id", stream.StreamID().InitiatedBy().String()),
		log.String("type", streamType.String()))

	return qstream
}

// Read reads data from the stream
func (s *Stream) Read(p []byte) (n int, err error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return 0, protocol.ErrStreamClosed
	}

	n, err = s.stream.Read(p)
	if n > 0 {
		s.UpdateActivity()
		s.AddBytesReceived(uint64(n))
		s.logger.Debug("Data read from stream", log.Int("bytes", n))
	}

	if err == io.EOF {
		s.SetState(protocol.StreamStateHalfClosed)
		s.logger.Debug("Stream reached EOF")
	}

	return n, err
}

// Write writes data to the stream
func (s *Stream) Write(p []byte) (n int, err error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return 0, protocol.ErrStreamClosed
	}

	n, err = s.stream.Write(p)
	if n > 0 {
		s.UpdateActivity()
		s.AddBytesSent(uint64(n))
		s.logger.Debug("Data written to stream", log.Int("bytes", n))
	}

	return n, err
}

// SendMessage sends a framed message over the stream
func (s *Stream) SendMessage(msg *protocol.Message) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return protocol.ErrStreamClosed
	}

	s.logger.Debug("Sending message",
		log.String("message_id", string(msg.ID)),
		log.String("type", msg.Type.String()),
		log.Int("payload_size", len(msg.Payload)))

	// Create message frame
	frame := s.createMessageFrame(msg)

	// Write frame header (length + message data)
	header := make([]byte, 8) // 8 bytes for uint64 length
	binary.BigEndian.PutUint64(header, uint64(len(frame)))

	// Write header
	_, err := s.Write(header)
	if err != nil {
		s.logger.Error("Failed to write message header",
			log.String("message_id", string(msg.ID)),
			log.Error(err))
		return protocol.WrapError(err, "failed to write message header")
	}

	// Write frame data
	_, err = s.Write(frame)
	if err != nil {
		s.logger.Error("Failed to write message data",
			log.String("message_id", string(msg.ID)),
			log.Error(err))
		return protocol.WrapError(err, "failed to write message data")
	}

	s.logger.Debug("Message sent successfully",
		log.String("message_id", string(msg.ID)),
		log.Int("frame_size", len(frame)))

	return nil
}

// ReceiveMessage receives a framed message from the stream
func (s *Stream) ReceiveMessage(_ context.Context) (*protocol.Message, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, protocol.ErrStreamClosed
	}

	// Read frame header
	header := make([]byte, 8)
	_, err := io.ReadFull(s.stream, header)
	if err != nil {
		if err == io.EOF {
			s.logger.Debug("Stream reached EOF while reading header")
			return nil, io.EOF
		}
		s.logger.Error("Failed to read message header", log.Error(err))
		return nil, protocol.WrapError(err, "failed to read message header")
	}

	// Parse frame length
	frameLength := binary.BigEndian.Uint64(header)
	if frameLength > uint64(s.BaseStream.Config().MaxMessageSize) {
		s.logger.Error("Message too large",
			log.Uint64("frame_length", frameLength),
			log.Int("max_size", s.BaseStream.Config().MaxMessageSize))
		return nil, protocol.ErrMessageTooLarge
	}

	// Read frame data
	frameData := make([]byte, frameLength)
	_, err = io.ReadFull(s.stream, frameData)
	if err != nil {
		s.logger.Error("Failed to read message data",
			log.Uint64("expected_length", frameLength),
			log.Error(err))
		return nil, protocol.WrapError(err, "failed to read message data")
	}

	// Parse message from frame
	msg, err := s.parseMessageFrame(frameData)
	if err != nil {
		s.logger.Error("Failed to parse message frame", log.Error(err))
		return nil, protocol.WrapError(err, "failed to parse message frame")
	}

	s.logger.Debug("Message received successfully",
		log.String("message_id", string(msg.ID)),
		log.String("type", msg.Type.String()),
		log.Int("payload_size", len(msg.Payload)))

	return msg, nil
}

// Close closes the stream
func (s *Stream) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil // Already closed
	}

	s.logger.Info("Closing QUIC stream")
	s.SetState(protocol.StreamStateClosed)
	s.MarkClosed()

	return s.stream.Close()
}

// CloseWrite closes the write side of the stream
func (s *Stream) CloseWrite() error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return protocol.ErrStreamClosed
	}

	s.logger.Debug("Closing write side of stream")
	s.SetState(protocol.StreamStateHalfClosed)
	return s.stream.Close()
}

// CloseRead closes the read side of the stream
func (s *Stream) CloseRead() error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return protocol.ErrStreamClosed
	}

	s.logger.Debug("Closing read side of stream")
	s.SetState(protocol.StreamStateHalfClosed)
	// QUIC streams don't have separate read/write close, so we cancel the read context
	return nil
}

// createMessageFrame creates a binary frame from a message
func (s *Stream) createMessageFrame(msg *protocol.Message) []byte {
	// Binary format with UUID support:
	// [36 bytes: MessageID (UUID string)]
	// [1 byte: MessageType]
	// [1 byte: Priority]
	// [8 bytes: Timestamp (Unix nano)]
	// [8 bytes: TTL (nanoseconds)]
	// [36 bytes: SourceClientID (UUID string)]
	// [36 bytes: TargetClientID (UUID string)]
	// [36 bytes: GroupID (UUID string)]
	// [36 bytes: ScopeID (UUID string)]
	// [4 bytes: Payload length]
	// [N bytes: Payload]

	headerSize := 36 + 1 + 1 + 8 + 8 + 36 + 36 + 36 + 36 + 4 // 234 bytes
	frame := make([]byte, headerSize+len(msg.Payload))

	offset := 0

	// MessageID (36 bytes, padded with zeros if shorter)
	copy(frame[offset:offset+36], []byte(msg.ID))
	offset += 36

	// MessageType
	frame[offset] = byte(msg.Type)
	offset += 1

	// Priority
	frame[offset] = byte(msg.Priority)
	offset += 1

	// Timestamp
	binary.BigEndian.PutUint64(frame[offset:], uint64(msg.Timestamp.UnixNano()))
	offset += 8

	// TTL
	binary.BigEndian.PutUint64(frame[offset:], uint64(msg.TTL.Nanoseconds()))
	offset += 8

	// SourceClientID (36 bytes, padded with zeros if shorter)
	copy(frame[offset:offset+36], []byte(msg.SourceClientID))
	offset += 36

	// TargetClientID (36 bytes, padded with zeros if shorter)
	copy(frame[offset:offset+36], []byte(msg.TargetClientID))
	offset += 36

	// GroupID (36 bytes, padded with zeros if shorter)
	copy(frame[offset:offset+36], []byte(msg.GroupID))
	offset += 36

	// ScopeID (36 bytes, padded with zeros if shorter)
	copy(frame[offset:offset+36], []byte(msg.ScopeID))
	offset += 36

	// Payload length
	binary.BigEndian.PutUint32(frame[offset:], uint32(len(msg.Payload)))
	offset += 4

	// Payload
	copy(frame[offset:], msg.Payload)

	return frame
}

// parseMessageFrame parses a message from a binary frame
func (s *Stream) parseMessageFrame(frame []byte) (*protocol.Message, error) {
	if len(frame) < 234 { // Minimum header size
		return nil, protocol.ErrInvalidMessage
	}

	offset := 0

	// MessageID (36 bytes)
	messageIDBytes := make([]byte, 36)
	copy(messageIDBytes, frame[offset:offset+36])
	messageID := protocol.MessageID(string(messageIDBytes[:findNullTerminator(messageIDBytes)]))
	offset += 36

	// MessageType
	messageType := protocol.MessageType(frame[offset])
	offset += 1

	// Priority
	priority := protocol.Priority(frame[offset])
	offset += 1

	// Timestamp
	timestamp := time.Unix(0, int64(binary.BigEndian.Uint64(frame[offset:])))
	offset += 8

	// TTL
	ttl := time.Duration(binary.BigEndian.Uint64(frame[offset:]))
	offset += 8

	// SourceClientID (36 bytes)
	sourceClientIDBytes := make([]byte, 36)
	copy(sourceClientIDBytes, frame[offset:offset+36])
	sourceClientID := protocol.ClientID(sourceClientIDBytes[:findNullTerminator(sourceClientIDBytes)])
	offset += 36

	// TargetClientID (36 bytes)
	targetClientIDBytes := make([]byte, 36)
	copy(targetClientIDBytes, frame[offset:offset+36])
	targetClientID := protocol.ClientID(targetClientIDBytes[:findNullTerminator(targetClientIDBytes)])
	offset += 36

	// GroupID (36 bytes)
	groupIDBytes := make([]byte, 36)
	copy(groupIDBytes, frame[offset:offset+36])
	groupID := protocol.GroupID(groupIDBytes[:findNullTerminator(groupIDBytes)])
	offset += 36

	// ScopeID (36 bytes)
	scopeIDBytes := make([]byte, 36)
	copy(scopeIDBytes, frame[offset:offset+36])
	scopeID := protocol.ScopeID(scopeIDBytes[:findNullTerminator(scopeIDBytes)])
	offset += 36

	// Payload length
	payloadLength := binary.BigEndian.Uint32(frame[offset:])
	offset += 4

	// Validate payload length
	if offset+int(payloadLength) != len(frame) {
		return nil, protocol.ErrInvalidMessage
	}

	// Payload
	payload := make([]byte, payloadLength)
	copy(payload, frame[offset:])

	// Create message
	msg := protocol.GetMessage()
	msg.ID = messageID
	msg.Type = messageType
	msg.Priority = priority
	msg.Timestamp = timestamp
	msg.TTL = ttl
	msg.SourceClientID = sourceClientID
	msg.TargetClientID = targetClientID
	msg.GroupID = groupID
	msg.ScopeID = scopeID
	msg.Payload = payload

	return msg, nil
}

// findNullTerminator finds the first null byte in a byte slice
func findNullTerminator(data []byte) int {
	for i, b := range data {
		if b == 0 {
			return i
		}
	}
	return len(data)
}

// QUICStreamID returns the underlying QUIC stream ID
func (s *Stream) QUICStreamID() quic.StreamID {
	return s.stream.StreamID()
}
