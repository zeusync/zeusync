package main

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/fields"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"math/rand/v2"
	"time"
)

func main() {
	logger := log.New(log.LevelDebug)

	player := &Player{
		Health: fields.NewFA[int64](100),
		Money:  fields.NewFA[int64](0),
	}

	unsub := player.Money.Subscribe(func(newValue int64) {
		logger.Info("Player's money changed", log.Int64("value", newValue))
	})
	defer unsub()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wg := fields.NewWaitGroup()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(time.Millisecond * 100)
			for {
				select {
				case <-ctx.Done():
					unsub()
					return
				case <-ticker.C:
					player.Money.Transaction(func(current int64) int64 {
						return current + rand.Int64N(10_000)
					})
				}
			}
		}()
	}

	wg.Wait()

	logger.Info("Money", log.Uint64("version", player.Money.Version()), log.Int64("value", player.Money.Get()))

	data, _ := player.Money.Serialize()

	fmt.Println("Serialized data:", string(data))
}

type Player struct {
	Health fields.FieldAccessor[int64]
	Money  fields.FieldAccessor[int64]
}
