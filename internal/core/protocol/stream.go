package protocol

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"sync/atomic"
	"time"
)

// StreamState represents the current state of a stream
type StreamState int

const (
	StreamStateOpen StreamState = iota
	StreamStateHalfClosed
	StreamStateClosed
	StreamStateError
)

// StreamType defines the type of stream
type StreamType int

const (
	StreamTypeBidirectional StreamType = iota
	StreamTypeUnidirectional
	StreamTypeReliable
	StreamTypeUnreliable
)

// Stream represents a bidirectional or unidirectional data stream
type Stream interface {
	// Identity

	ID() StreamID
	ConnectionID() ConnectionID
	Type() StreamType
	State() StreamState

	// I/O operations

	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)

	// Message operations

	SendMessage(msg *Message) error
	ReceiveMessage(ctx context.Context) (*Message, error)

	// Stream control

	Close() error
	CloseWrite() error
	CloseRead() error

	// Properties

	BytesSent() uint64
	BytesReceived() uint64
	CreatedAt() time.Time
	LastActivity() time.Time

	// Context and cancellation

	Context() context.Context
	Done() <-chan struct{}
}

// StreamConfig holds configuration for streams
type StreamConfig struct {
	Type             StreamType
	BufferSize       int
	WriteTimeout     time.Duration
	ReadTimeout      time.Duration
	MaxMessageSize   int
	EnableFraming    bool
	CompressionLevel int
}

// DefaultStreamConfig returns default stream configuration
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		Type:             StreamTypeBidirectional,
		BufferSize:       64 * 1024, // 64KB
		WriteTimeout:     5 * time.Second,
		ReadTimeout:      5 * time.Second,
		MaxMessageSize:   1024 * 1024, // 1MB
		EnableFraming:    true,
		CompressionLevel: 1,
	}
}

// BaseStream provides common stream functionality
type BaseStream struct {
	id            StreamID
	connectionID  ConnectionID
	streamType    StreamType
	state         int32 // atomic StreamState
	createdAt     time.Time
	lastActivity  int64 // atomic unix timestamp
	bytesSent     uint64
	bytesReceived uint64
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	config        StreamConfig
	logger        log.Log
}

// NewBaseStream creates a new base stream
func NewBaseStream(id StreamID, connectionID ConnectionID, streamType StreamType, config StreamConfig, logger log.Log) *BaseStream {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = log.Provide()
	}

	stream := &BaseStream{
		id:           id,
		connectionID: connectionID,
		streamType:   streamType,
		state:        int32(StreamStateOpen),
		createdAt:    time.Now(),
		lastActivity: time.Now().Unix(),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
		config:       config,
		logger:       logger.With(log.String("stream_id", string(id)), log.String("connection_id", string(connectionID))),
	}

	stream.logger.Debug("Stream created",
		log.String("type", streamType.String()),
		log.Int("buffer_size", config.BufferSize))

	return stream
}

// ID returns the stream ID
func (s *BaseStream) ID() StreamID {
	return s.id
}

// ConnectionID returns the connection ID this stream belongs to
func (s *BaseStream) ConnectionID() ConnectionID {
	return s.connectionID
}

// Type returns the stream type
func (s *BaseStream) Type() StreamType {
	return s.streamType
}

// State returns the current stream state
func (s *BaseStream) State() StreamState {
	return StreamState(atomic.LoadInt32(&s.state))
}

// SetState atomically sets the stream state
func (s *BaseStream) SetState(state StreamState) {
	oldState := StreamState(atomic.SwapInt32(&s.state, int32(state)))
	if oldState != state {
		s.logger.Debug("Stream state changed",
			log.String("old_state", oldState.String()),
			log.String("new_state", state.String()))
	}
}

// BytesSent returns the number of bytes sent
func (s *BaseStream) BytesSent() uint64 {
	return atomic.LoadUint64(&s.bytesSent)
}

// BytesReceived returns the number of bytes received
func (s *BaseStream) BytesReceived() uint64 {
	return atomic.LoadUint64(&s.bytesReceived)
}

// CreatedAt returns the stream creation time
func (s *BaseStream) CreatedAt() time.Time {
	return s.createdAt
}

// LastActivity returns the last activity time
func (s *BaseStream) LastActivity() time.Time {
	return time.Unix(atomic.LoadInt64(&s.lastActivity), 0)
}

// UpdateActivity updates the last activity timestamp
func (s *BaseStream) UpdateActivity() {
	atomic.StoreInt64(&s.lastActivity, time.Now().Unix())
}

// AddBytesSent atomically adds to bytes sent counter
func (s *BaseStream) AddBytesSent(bytes uint64) {
	atomic.AddUint64(&s.bytesSent, bytes)
	s.UpdateActivity()
}

// AddBytesReceived atomically adds to bytes received counter
func (s *BaseStream) AddBytesReceived(bytes uint64) {
	atomic.AddUint64(&s.bytesReceived, bytes)
	s.UpdateActivity()
}

// Context returns the stream context
func (s *BaseStream) Context() context.Context {
	return s.ctx
}

// Done returns a channel that's closed when the stream is closed
func (s *BaseStream) Done() <-chan struct{} {
	return s.done
}

// MarkClosed marks the stream as closed
func (s *BaseStream) MarkClosed() {
	s.SetState(StreamStateClosed)
	s.cancel()
	select {
	case <-s.done:
	default:
		close(s.done)
		s.logger.Info("Stream closed",
			log.Duration("duration", time.Since(s.createdAt)),
			log.Uint64("bytes_sent", atomic.LoadUint64(&s.bytesSent)),
			log.Uint64("bytes_received", atomic.LoadUint64(&s.bytesReceived)))
	}
}

// Logger returns the stream logger
func (s *BaseStream) Logger() log.Log {
	return s.logger
}

// Config returns the stream configuration
func (s *BaseStream) Config() StreamConfig {
	return s.config
}

// StreamManager manages multiple streams within a connection
type StreamManager interface {
	// Stream lifecycle

	OpenStream(ctx context.Context, config StreamConfig) (Stream, error)
	AcceptStream(ctx context.Context) (Stream, error)
	CloseStream(id StreamID) error

	// Stream queries

	GetStream(id StreamID) (Stream, bool)
	ListStreams() []Stream
	StreamCount() int

	// Cleanup

	Close() error
}

// StreamMultiplexer handles multiplexing of multiple streams over a single connection
type StreamMultiplexer interface {
	// Stream operations

	CreateStream(ctx context.Context, streamType StreamType) (Stream, error)
	HandleStream(stream Stream, handler StreamHandler) error

	// Flow control

	SetStreamLimit(limit int)
	GetStreamLimit() int

	// Statistics

	ActiveStreams() int
	TotalStreams() uint64

	// Lifecycle

	Close() error
}

// StreamFrame represents a framed piece of stream data
type StreamFrame struct {
	StreamID StreamID
	Type     uint8
	Flags    uint8
	Length   uint32
	Data     []byte
}

// Frame types
const (
	FrameTypeData uint8 = iota
	FrameTypeHeaders
	FrameTypeReset
	FrameTypeSettings
	FrameTypePing
	FrameTypeGoAway
	FrameTypeWindowUpdate
)

// Frame flags
const (
	FlagEndStream  uint8 = 0x1
	FlagEndHeaders uint8 = 0x4
	FlagPadded     uint8 = 0x8
	FlagPriority   uint8 = 0x20
)

// StreamFramer handles framing and reframing of stream data
type StreamFramer interface {
	// Frame operations

	WriteFrame(frame *StreamFrame) error
	ReadFrame() (*StreamFrame, error)

	// Configuration

	MaxFrameSize() uint32
	SetMaxFrameSize(size uint32)

	// Lifecycle

	Close() error
}

// StreamState string representation
func (ss StreamState) String() string {
	switch ss {
	case StreamStateOpen:
		return "open"
	case StreamStateHalfClosed:
		return "half_closed"
	case StreamStateClosed:
		return "closed"
	case StreamStateError:
		return "error"
	default:
		return "unknown"
	}
}

// StreamType string representation
func (st StreamType) String() string {
	switch st {
	case StreamTypeBidirectional:
		return "bidirectional"
	case StreamTypeUnidirectional:
		return "unidirectional"
	case StreamTypeReliable:
		return "reliable"
	case StreamTypeUnreliable:
		return "unreliable"
	default:
		return "unknown"
	}
}
