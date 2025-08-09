package bus

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// simpleEvent is a basic implementation of Event.
// It can be used by callers who don't have their own Event types.
type simpleEvent struct {
	typeStr string
	source  string
	ts      time.Time
	data    any
	prio    int
	meta    map[string]any
}

// persistence structures
type busState struct {
	Topics []string
}

func (e simpleEvent) Type() string             { return e.typeStr }
func (e simpleEvent) Source() string           { return e.source }
func (e simpleEvent) Timestamp() time.Time     { return e.ts }
func (e simpleEvent) Data() any                { return e.data }
func (e simpleEvent) Priority() int            { return e.prio }
func (e simpleEvent) Metadata() map[string]any { return e.meta }

// NewEvent creates a simple Event implementation.
func NewEvent(typ, src string, data any, priority int, metadata map[string]any) Event {
	return simpleEvent{typeStr: typ, source: src, ts: time.Now(), data: data, prio: priority, meta: metadata}
}

// subscription implements Subscription interface.
type subscription struct {
	id        string
	eventType string
	handler   EventHandler
	active    bool
	cancel    func()
}

func (s *subscription) ID() string            { return s.id }
func (s *subscription) EventType() string     { return s.eventType }
func (s *subscription) Handler() EventHandler { return s.handler }
func (s *subscription) IsActive() bool        { return s.active }
func (s *subscription) Cancel() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.active = false
	return nil
}

// inMemoryBus is a thread-safe implementation of EventBus with optional topics and observers.
type inMemoryBus struct {
	mu sync.RWMutex
	// handlers: topic -> eventType -> subID -> subscription
	handlers  map[string]map[string]map[string]*subscription
	topics    map[string]TopicConfig
	metrics   EventBusMetrics
	observers map[EventBusObserver]struct{}
}

// New creates a new EventBus instance.
func New() EventBus { // constructor exported from package
	return &inMemoryBus{
		handlers:  make(map[string]map[string]map[string]*subscription),
		topics:    make(map[string]TopicConfig),
		observers: make(map[EventBusObserver]struct{}),
	}
}

func (b *inMemoryBus) Publish(event Event) error {
	return b.deliver("", event)
}

func (b *inMemoryBus) PublishToTopic(topic string, event Event) error {
	return b.deliver(topic, event)
}

func (b *inMemoryBus) PublishWithFilters(event Event, filters ...EventFilter) error {
	for _, f := range filters {
		if !f(event) {
			// count drop if observing
			b.mu.Lock()
			if len(b.observers) > 0 {
				b.metrics.DroppedByFilters += 1
			}
			b.mu.Unlock()
			return nil
		}
	}
	return b.Publish(event)
}

func (b *inMemoryBus) Subscribe(eventType string, handler EventHandler) (Subscription, error) {
	return b.SubscribeTopic("", eventType, handler)
}

func (b *inMemoryBus) SubscribeTopic(topic, eventType string, handler EventHandler) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensureTopicLocked(topic)
	if b.handlers[topic][eventType] == nil {
		b.handlers[topic][eventType] = make(map[string]*subscription)
	}
	id := uuid.NewString()
	s := &subscription{id: id, eventType: eventType, handler: handler, active: true}
	s.cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if mm, ok := b.handlers[topic][eventType]; ok {
			delete(mm, id)
		}
		s.active = false
	}
	b.handlers[topic][eventType][id] = s
	return s, nil
}

func (b *inMemoryBus) Unsubscribe(sub Subscription) error {
	if sub == nil {
		return nil
	}
	return sub.Cancel()
}

func (b *inMemoryBus) PublishAsync(event Event) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- b.Publish(event)
		close(ch)
	}()
	return ch
}

func (b *inMemoryBus) PublishBatch(events ...Event) error {
	var all error
	for _, e := range events {
		if err := b.Publish(e); err != nil {
			if all == nil {
				all = err
			} else {
				all = errors.Join(all, err)
			}
		}
	}
	return all
}

func (b *inMemoryBus) CreateTopic(name string, config TopicConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.topics[name]; exists {
		return nil
	}
	b.topics[name] = config
	if b.handlers[name] == nil {
		b.handlers[name] = make(map[string]map[string]*subscription)
	}
	return nil
}

func (b *inMemoryBus) AddObserver(obs EventBusObserver) {
	b.mu.Lock()
	b.observers[obs] = struct{}{}
	b.mu.Unlock()
}

func (b *inMemoryBus) RemoveObserver(obs EventBusObserver) {
	b.mu.Lock()
	delete(b.observers, obs)
	b.mu.Unlock()
}

func (b *inMemoryBus) GetMetrics() EventBusMetrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.metrics
}

func (b *inMemoryBus) GetTopics() []TopicInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]TopicInfo, 0, len(b.topics))
	for name := range b.topics {
		info := TopicInfo{Name: name}
		if hm := b.handlers[name]; hm != nil {
			info.EventTypes = len(hm)
			subs := 0
			for _, m := range hm {
				subs += len(m)
			}
			info.Subs = subs
		}
		out = append(out, info)
	}
	return out
}

func (b *inMemoryBus) SaveState() ([]byte, error) {
	b.mu.RLock()
	s := busState{Topics: make([]string, 0, len(b.topics))}
	for name := range b.topics {
		s.Topics = append(s.Topics, name)
	}
	b.mu.RUnlock()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *inMemoryBus) LoadState(data []byte) error {
	var s busState
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&s); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, name := range s.Topics {
		if _, ok := b.topics[name]; !ok {
			b.topics[name] = TopicConfig{}
		}
		if b.handlers[name] == nil {
			b.handlers[name] = make(map[string]map[string]*subscription)
		}
	}
	return nil
}

// ensureTopic ensures maps exist for a topic
func (b *inMemoryBus) ensureTopicLocked(topic string) {
	if _, ok := b.topics[topic]; !ok {
		b.topics[topic] = TopicConfig{}
	}
	if b.handlers[topic] == nil {
		b.handlers[topic] = make(map[string]map[string]*subscription)
	}
}

func (b *inMemoryBus) deliver(topic string, event Event) error {
	start := time.Now()
	b.mu.RLock()
	etype := event.Type()
	var subs []*subscription
	if inner := b.handlers[topic]; inner != nil {
		if m := inner[etype]; m != nil {
			subs = make([]*subscription, 0, len(m))
			for _, s := range m {
				subs = append(subs, s)
			}
		}
	}
	obsCount := len(b.observers)
	b.mu.RUnlock()

	if obsCount > 0 {
		for obs := range b.observers {
			obs.OnPublish(topic, etype, event)
		}
	}

	var all error
	for _, s := range subs {
		if !s.active {
			continue
		}
		if err := s.handler(event); err != nil {
			if all == nil {
				all = err
			} else {
				all = errors.Join(all, err)
			}
		}
	}

	if obsCount > 0 {
		dur := time.Since(start).Microseconds()
		for obs := range b.observers {
			obs.OnDelivered(topic, etype, len(subs), all, dur)
		}
		// update metrics only when observing
		b.mu.Lock()
		b.metrics.Published += 1
		b.metrics.DeliveredHandlers += uint64(len(subs))
		if all != nil {
			b.metrics.Errors += 1
		}
		b.metrics.Topics = uint64(len(b.topics))
		// recompute subscribers active (approx): sum of all subs
		var subsCount uint64
		for _, et := range b.handlers {
			for _, m := range et {
				subsCount += uint64(len(m))
			}
		}
		b.metrics.SubscribersActive = subsCount
		b.mu.Unlock()
	}
	return all
}
