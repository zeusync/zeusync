package bus

import "time"

// EventBus defines a thread-safe, in-process pub/sub event bus.
//
// Key characteristics:
// - Type-based fan-out: handlers subscribe by Event.Type() string.
// - Optional topics: handlers can subscribe within a topic for isolation and scoping.
// - Synchronous delivery: Publish calls handler callbacks in the caller goroutine.
// - Error aggregation: multiple handler errors are joined and returned from Publish/PublishBatch.
// - Optional helpers: async publish, batch publish, pre-delivery filters.
// - Optional observability: metrics are produced only when observers are registered.
//
// Notes:
// - Filters are evaluated before delivery. If any filter rejects an event, it is dropped without error.
// - Topics are logical groupings; the default topic is "" (empty string). Legacy methods operate on the default topic.
// - Handlers should be quick or offload heavy work to avoid blocking publishers.
// - All methods must be safe for concurrent use.
type EventBus interface {
	// Publish delivers the event synchronously to all active subscribers of event.Type()
	// in the default topic. If one or more handlers return an error, a joined error is
	// returned.
	Publish(event Event) error
	// Subscribe registers a handler for a specific event type in the default topic and
	// returns a Subscription handle that can be used to cancel later.
	Subscribe(eventType string, handler EventHandler) (Subscription, error)
	// Unsubscribe cancels the given Subscription. It is safe to call with nil; does nothing.
	Unsubscribe(Subscription) error

	// PublishWithFilters applies filters before delivery; if any filter returns false,
	// the event is dropped and not delivered to handlers.
	PublishWithFilters(event Event, filters ...EventFilter) error

	// CreateTopic declares a logical topic. Implementations may no-op; repeat declarations are idempotent.
	CreateTopic(name string, config TopicConfig) error
	// SubscribeTopic registers a handler for eventType within a topic.
	SubscribeTopic(topic, eventType string, handler EventHandler) (Subscription, error)
	// PublishToTopic publishes to a specific topic.
	PublishToTopic(topic string, event Event) error

	// PublishAsync publishes in a separate goroutine and returns a channel that will receive
	// a joined error (or nil) when delivery completes; then the channel is closed.
	PublishAsync(event Event) <-chan error
	// PublishBatch publishes a set of events sequentially and aggregates errors across them.
	PublishBatch(events ...Event) error

	// AddObserver registers an observer to receive metrics callbacks.
	AddObserver(obs EventBusObserver)
	// RemoveObserver unregisters a previously added observer.
	RemoveObserver(obs EventBusObserver)
	// GetMetrics returns a best-effort snapshot of accumulated metrics. Metrics are only
	// collected when at least one observer is registered.
	GetMetrics() EventBusMetrics
	// GetTopics returns a snapshot list of known topics.
	GetTopics() []TopicInfo

	// SaveState serializes the bus state (topics and minimal metadata) into a binary blob.
	// Note: subscriptions/handlers are not persisted.
	SaveState() ([]byte, error)
	// LoadState restores the bus state from a binary blob produced by SaveState.
	LoadState(data []byte) error
}

// Event is an immutable message transported by the EventBus.
//
// Fields:
// - Type: routing key used to select handlers (required for delivery).
// - Source: identifier of the publisher (free-form).
// - Timestamp: creation time of the event.
// - Data: opaque payload for consumers.
// - Priority: optional priority hint (not used by the in-memory bus).
// - Metadata: small key/value annotations for additional context.
//
// Implementations should treat Event values as read-only.
type Event interface {
	Type() string
	Source() string
	Timestamp() time.Time
	Data() any
	Priority() int
	Metadata() map[string]any
}

// EventHandler is a user callback invoked per delivered event. If it returns an
// error, Publish/PublishBatch aggregates and returns it.
type (
	EventHandler func(event Event) error
	// EventFilter decides whether an event should be delivered. If any filter
	// returns false, the event is dropped silently.
	EventFilter func(event Event) bool
)

// Subscription represents a registered handler bound to an event type.
// Use Cancel or EventBus.Unsubscribe to stop receiving events.
type Subscription interface {
	// ID is a unique identifier for this subscription.
	ID() string
	// EventType returns the event type this subscription listens to.
	EventType() string
	// Handler returns the callback associated with this subscription.
	Handler() EventHandler
	// IsActive reports whether this subscription is still registered.
	IsActive() bool
	// Cancel de-registers the handler from the bus. Multiple calls are safe.
	Cancel() error
}

// TopicConfig describes topic-level settings (reserved for future use).
type TopicConfig struct{}

// EventBusObserver is notified about deliveries and errors. Implementations can
// export metrics, tracing, or logs. Observers should return quickly.
type EventBusObserver interface {
	OnPublish(topic, eventType string, event Event)
	OnDelivered(topic, eventType string, handlers int, err error, durationMicros int64)
}

// EventBusMetrics represents a minimal set of counters; it is updated only when
// at least one observer is registered.
type EventBusMetrics struct {
	Published         uint64
	DeliveredHandlers uint64
	Errors            uint64
	DroppedByFilters  uint64
	SubscribersActive uint64
	Topics            uint64
}

// TopicInfo provides a minimal snapshot about a topic.
type TopicInfo struct {
	Name       string
	EventTypes int
	Subs       int
}
