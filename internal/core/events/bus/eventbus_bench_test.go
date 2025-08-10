package bus

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// helper to make a simple event quickly
func benchEvt(t string) Event {
	return NewEvent(t, "bench", nil, 0, nil)
}

// no-op handler that increments a counter to avoid compiler eliminating logic
func makeHandler(c *int64, err bool) EventHandler {
	return func(e Event) error {
		atomic.AddInt64(c, 1)
		if err {
			return assertError
		}
		return nil
	}
}

// assertError is a sentinel returned from handlers when we want to simulate errors
var assertError = errSentinel{}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel" }

type nopObserver struct{}

func (nopObserver) OnPublish(topic, eventType string, event Event)                                {}
func (nopObserver) OnDelivered(topic, eventType string, handlers int, err error, durMicros int64) {}

func BenchmarkPublishSingleSubscriber(b *testing.B) {
	bus := New()
	var c int64
	_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	e := benchEvt("tick")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.Publish(e)
	}
	b.StopTimer()
	_ = c // keep referenced
}

func BenchmarkPublishManySubscribers(b *testing.B) {
	for _, subs := range []int{1, 4, 16, 64, 256, 1024} {
		b.Run("subs="+itoa(subs), func(b *testing.B) {
			bus := New()
			var c int64
			for i := 0; i < subs; i++ {
				_, _ = bus.Subscribe("tick", makeHandler(&c, false))
			}
			e := benchEvt("tick")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = bus.Publish(e)
			}
			b.StopTimer()
			_ = c
		})
	}
}

func BenchmarkPublishManyTopics(b *testing.B) {
	topics := []int{1, 8, 32, 128}
	eventTypes := 4
	for _, tcount := range topics {
		b.Run("topics="+itoa(tcount), func(b *testing.B) {
			bus := New()
			var c int64
			// create topics and subscribe one handler per type per topic
			for ti := 0; ti < tcount; ti++ {
				topic := "t" + itoa(ti)
				_ = bus.CreateTopic(topic, TopicConfig{})
				for et := 0; et < eventTypes; et++ {
					etype := "E" + itoa(et)
					_, _ = bus.SubscribeTopic(topic, etype, makeHandler(&c, false))
				}
			}
			// cycle events across topics and types
			b.ReportAllocs()
			b.ResetTimer()
			idx := 0
			for i := 0; i < b.N; i++ {
				topic := "t" + itoa(idx%tcount)
				etype := "E" + itoa((idx/tcount)%eventTypes)
				_ = bus.PublishToTopic(topic, benchEvt(etype))
				idx++
			}
			b.StopTimer()
			_ = c
		})
	}
}

func BenchmarkConcurrentPublishers(b *testing.B) {
	bus := New()
	var c int64
	// 64 handlers to add some work
	for i := 0; i < 64; i++ {
		_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	}
	e := benchEvt("tick")
	b.ReportAllocs()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bus.Publish(e)
		}
	})
	_ = c
}

func BenchmarkPublishWithFilters(b *testing.B) {
	bus := New()
	var c int64
	_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	e := benchEvt("tick")
	pass := func(Event) bool { return true }
	drop := func(Event) bool { return false }
	b.Run("pass", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishWithFilters(e, pass, pass, pass)
		}
	})
	b.Run("drop-early", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishWithFilters(e, drop)
		}
	})
	b.Run("drop-late", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.PublishWithFilters(e, pass, pass, drop)
		}
	})
	_ = c
}

func BenchmarkObserverOverhead(b *testing.B) {
	bus := New()
	var c int64
	for i := 0; i < 32; i++ {
		_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	}
	e := benchEvt("tick")
	b.Run("no-observer", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.Publish(e)
		}
	})
	b.Run("with-observer", func(b *testing.B) {
		bus.AddObserver(nopObserver{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bus.Publish(e)
		}
		bus.RemoveObserver(nopObserver{}) // has no effect because different instance, but fine
	})
	_ = c
}

func BenchmarkPublishAsync(b *testing.B) {
	bus := New()
	var c int64
	for i := 0; i < 8; i++ {
		_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	}
	e := benchEvt("tick")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := bus.PublishAsync(e)
		<-ch
	}
	_ = c
}

func BenchmarkPublishBatch(b *testing.B) {
	bus := New()
	var c int64
	for i := 0; i < 16; i++ {
		_, _ = bus.Subscribe("tick", makeHandler(&c, false))
	}
	events := make([]Event, 64)
	for i := range events {
		events[i] = benchEvt("tick")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.PublishBatch(events...)
	}
	_ = c
}

func BenchmarkSubscribeUnsubscribeChurn(b *testing.B) {
	bus := New()
	var c int64
	stop := make(chan struct{})
	// churn goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		subs := make([]Subscription, 0, 2048)
		const (
			maxSubs     = 4096
			addBatch    = 128
			removeBatch = 64
		)
		for {
			select {
			case <-stop:
				// best-effort cleanup; check stop regularly to avoid long stalls
				for len(subs) > 0 {
					last := subs[len(subs)-1]
					subs = subs[:len(subs)-1]
					_ = bus.Unsubscribe(last)
				}
				return
			default:
				// add a batch, bounded by maxSubs, checking stop frequently
				for i := 0; i < addBatch && len(subs) < maxSubs; i++ {
					select {
					case <-stop:
						return
					default:
					}
					s, _ := bus.Subscribe("tick", makeHandler(&c, false))
					subs = append(subs, s)
				}
				// remove a batch if any
				for i := 0; i < removeBatch && len(subs) > 0; i++ {
					select {
					case <-stop:
						return
					default:
					}
					last := subs[len(subs)-1]
					subs = subs[:len(subs)-1]
					_ = bus.Unsubscribe(last)
				}
				// yield to allow publishers to proceed
				runtime.Gosched()
			}
		}
	}()
	// small yield to let churn goroutine start
	runtime.Gosched()
	e := benchEvt("tick")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.Publish(e)
	}
	b.StopTimer()
	close(stop)
	wg.Wait()
	_ = c
}

// itoa without fmt to avoid extra allocations in benchmarks' names
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	bp := len(buf)
	for i > 0 {
		bp--
		buf[bp] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		bp--
		buf[bp] = '-'
	}
	return string(buf[bp:])
}
