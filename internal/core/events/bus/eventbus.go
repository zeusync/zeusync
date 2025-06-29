package bus

import "time"

type EventBus interface {
	Publish(event Event) error
	Subscribe(eventType string, handler EventHandler) (Subscription, error)
	Unsubscribe(Subscription) error

	PublishWithFilters(event Event, filters ...EventFilter) error
	CreateTopic(name string, config TopicConfig) error

	PublishAsync(event Event) error
	PublishBatch(events ...Event) error

	GetMetrics() EventBusMetrics
	GetTopics() []TopicInfo
}

type Event interface {
	Type() string
	Source() string
	Timestamp() time.Time
	Data() any
	Priority() int
	Metadata() map[string]any
}

type EventHandler func(event Event) error
type EventFilter func(event Event) bool

type Subscription interface {
	ID() string
	EventType() string
	Handler() EventHandler
	IsActive() bool
	Cancel() error
}

type TopicConfig struct{}
type EventBusMetrics struct{}
type TopicInfo struct{}
