package main

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/fields"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"sync"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.New(log.LevelDebug)

	player := NewPlayer()

	player.Health.Set(100)

	unsub1 := player.Health.Subscribe(func(newValue int64) {
		logger.Info("Player: Health changed", log.Int64("value", newValue))
	})
	defer unsub1()

	healthUpdateCh, unsub2 := player.Health.SubscribeIfCh(func(i int64) bool {
		if i >= 1000 {
			unsub1()
			return true
		}
		return false
	})
	defer unsub2()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case value := <-healthUpdateCh:
				logger.Warn("Player: Health changed, archived desired state",
					log.Int64("target", 1000),
					log.Int64("current", value),
				)
				unsub2()
			}
		}
	}()

	for i := 0; i < 100; i++ {
		currentHealth := player.Health.Get()
		player.Health.Set(currentHealth * int64(i))
	}

	wg.Wait()
}

type Player struct {
	Health fields.FieldAccessor[int64]
}

func NewPlayer() *Player {
	return &Player{
		Health: fields.NewFA[int64](),
	}
}
