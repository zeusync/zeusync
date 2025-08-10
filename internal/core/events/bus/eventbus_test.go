package bus

import (
	"errors"
	"testing"
	"time"
)

type testObserver struct {
	publishCount   int
	deliveredCount int
	lastErr        error
}

func (o *testObserver) OnPublish(_, _ string, _ Event) {
	o.publishCount++
}

func (o *testObserver) OnDelivered(_, _ string, handlers int, err error, _ int64) {
	o.deliveredCount += handlers
	o.lastErr = err
}

func TestBasicPublishSubscribe(t *testing.T) {
	b := New()
	done := make(chan struct{})
	_, err := b.Subscribe("test.event", func(e Event) error {
		close(done)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err = b.Publish(NewEvent("test.event", "tester", 123, 0, nil)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler not called")
	}
}

func TestPublishAsyncReturnsErrorChannel(t *testing.T) {
	b := New()
	handlerErr := errors.New("fail")
	_, err := b.Subscribe("x", func(e Event) error { return handlerErr })
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	ch := b.PublishAsync(NewEvent("x", "src", nil, 0, nil))
	select {
	case e := <-ch:
		if e == nil {
			t.Fatalf("expected error, got nil")
		}
	default:
		// allow some time for goroutine
		e := <-ch
		if e == nil {
			t.Fatalf("expected error")
		}
	}
}

func TestTopicsIsolation(t *testing.T) {
	b := New()
	if err := b.CreateTopic("t1", TopicConfig{}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	if err := b.CreateTopic("t2", TopicConfig{}); err != nil {
		t.Fatalf("topic: %v", err)
	}
	count1 := 0
	count2 := 0
	_, _ = b.SubscribeTopic("t1", "ev", func(e Event) error { count1++; return nil })
	_, _ = b.SubscribeTopic("t2", "ev", func(e Event) error { count2++; return nil })
	_ = b.PublishToTopic("t1", NewEvent("ev", "src", nil, 0, nil))
	if count1 != 1 || count2 != 0 {
		t.Fatalf("topic isolation failed: %d %d", count1, count2)
	}
}

func TestPersistenceTopics(t *testing.T) {
	b := New()
	_ = b.CreateTopic("ta", TopicConfig{})
	_ = b.CreateTopic("tb", TopicConfig{})
	data, err := b.SaveState()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	b2 := New()
	if err = b2.LoadState(data); err != nil {
		t.Fatalf("load: %v", err)
	}
	names := map[string]bool{}
	for _, ti := range b2.GetTopics() {
		names[ti.Name] = true
	}
	if !names["ta"] || !names["tb"] {
		t.Fatalf("topics not restored: %#v", names)
	}
}

func TestObserverMetricsOptional(t *testing.T) {
	b := New()
	// without observer, metrics should remain zero despite activity
	_, _ = b.Subscribe("e", func(e Event) error { return nil })
	_ = b.Publish(NewEvent("e", "s", nil, 0, nil))
	m := b.GetMetrics()
	if m.Published != 0 && m.DeliveredHandlers != 0 {
		t.Fatalf("metrics should be zero without observers: %+v", m)
	}
	// now add observer and expect metrics to update
	obs := &testObserver{}
	b.AddObserver(obs)
	_ = b.Publish(NewEvent("e", "s", nil, 0, nil))
	m2 := b.GetMetrics()
	if m2.Published == 0 || m2.DeliveredHandlers == 0 {
		t.Fatalf("metrics should update with observer: %+v", m2)
	}
	if obs.publishCount == 0 || obs.deliveredCount == 0 {
		t.Fatalf("observer not called: %+v", obs)
	}
}
