package npc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"time"

	phys "github.com/zeusync/zeusync/internal/core/systems/physics"
)

// Example built-in sensors

// DistanceSensor computes distance between two 2D points stored in blackboard keys: src(x,y) and dst(x,y)
// Params: src_x, src_y, dst_x, dst_y (keys). Writes distance to key 'out'
type DistanceSensor struct {
	name string
	// Generalized: prefer src/dst object keys; fallback to scalar coords.
	srcKey, dstKey string
	srcX, srcY     string
	dstX, dstY     string
	out            string
}

func NewDistanceSensor(name string, srcX, srcY, dstX, dstY, out string) *DistanceSensor {
	return &DistanceSensor{name: name, srcX: srcX, srcY: srcY, dstX: dstX, dstY: dstY, out: out}
}

func (d *DistanceSensor) WithObjects(srcKey, dstKey string) *DistanceSensor {
	d.srcKey = srcKey
	d.dstKey = dstKey
	return d
}

func (d *DistanceSensor) Name() string { return d.name }

func (d *DistanceSensor) Update(_ context.Context, bb Blackboard) error {
	// 1) Try physics-aware objects first
	if d.srcKey != "" && d.dstKey != "" {
		as, aok := bb.Get(d.srcKey)
		bs, bok := bb.Get(d.dstKey)
		if aok && bok {
			// Transform
			if at, ok1 := as.(phys.Transform); ok1 {
				if bt, ok2 := bs.(phys.Transform); ok2 {
					bb.Set(d.out, phys.DistanceT(at, bt))
					return nil
				}
			}
			// Vector2 fallback
			if av, ok1 := as.(phys.Vector2); ok1 {
				if bv, ok2 := bs.(phys.Vector2); ok2 {
					bb.Set(d.out, phys.Distance2V(av, bv))
					return nil
				}
			}
		}
	}
	// 2) Fallback to scalar coordinates
	x1, ok1 := getFloat(bb, d.srcX)
	y1, ok2 := getFloat(bb, d.srcY)
	x2, ok3 := getFloat(bb, d.dstX)
	y2, ok4 := getFloat(bb, d.dstY)
	if !(ok1 && ok2 && ok3 && ok4) {
		return fmt.Errorf("distance sensor missing coordinates or objects")
	}
	dist := math.Hypot(x2-x1, y2-y1)
	bb.Set(d.out, dist)
	return nil
}

func getFloat(bb Blackboard, key string) (float64, bool) {
	v, ok := bb.Get(key)
	if !ok {
		return 0, false
	}
	switch tv := v.(type) {
	case float64:
		return tv, true
	case float32:
		return float64(tv), true
	case int:
		return float64(tv), true
	case int64:
		return float64(tv), true
	default:
		return 0, false
	}
}

// InventorySensor checks if quantity under key 'inv_key' >= 'threshold' and writes boolean to 'out'
type InventorySensor struct {
	name      string
	invKey    string
	threshold float64
	out       string
}

func NewInventorySensor(name, invKey, out string, threshold float64) *InventorySensor {
	return &InventorySensor{name: name, invKey: invKey, threshold: threshold, out: out}
}

func (i *InventorySensor) Name() string { return i.name }

func (i *InventorySensor) Update(_ context.Context, bb Blackboard) error {
	v, ok := getFloat(bb, i.invKey)
	if !ok {
		return errors.New("inventory sensor: invalid inv value type")
	}
	bb.Set(i.out, v >= i.threshold)
	return nil
}

// TimerSensor toggles a boolean key true every interval ms for one tick.
type TimerSensor struct {
	name     string
	interval int // milliseconds
	out      string
}

func NewTimerSensor(name string, intervalMS int, out string) *TimerSensor {
	return &TimerSensor{name: name, interval: intervalMS, out: out}
}

func (s *TimerSensor) Name() string { return s.name }

func (s *TimerSensor) Update(ctx context.Context, bb Blackboard) error {
	// naive implementation using the last timestamp stored in BB
	key := s.name + ".last"
	nowAny, _ := bb.Get("__now_ms__") // allow external time injection if present
	var nowMS int64
	if v, ok := nowAny.(int64); ok {
		nowMS = v
	} else {
		nowMS = timeNowMS()
	}
	if lastAny, ok := bb.Get(key); ok {
		if last, ok2 := lastAny.(int64); ok2 {
			if nowMS-int64(s.interval) < last {
				bb.Set(s.out, false)
				return nil
			}
		}
	}
	bb.Set(s.out, true)
	bb.Set(key, nowMS)
	return nil
}

// SystemResourceSensor writes num_goroutine and alloc bytes to BB under prefix out
// Note: this is a lightweight snapshot using the runtime package.

type SystemResourceSensor struct{ name, out string }

func NewSystemResourceSensor(name, out string) *SystemResourceSensor {
	return &SystemResourceSensor{name: name, out: out}
}
func (s *SystemResourceSensor) Name() string { return s.name }
func (s *SystemResourceSensor) Update(ctx context.Context, bb Blackboard) error {
	n := runtimeNumGoroutine()
	alloc := runtimeMemAlloc()
	bb.Set(s.out+".goroutines", n)
	bb.Set(s.out+".alloc_bytes", alloc)
	return nil
}

// NetworkSensor relies on a BB function 'network_check' or boolean key 'network.ok'

type NetworkSensor struct{ name, out string }

func NewNetworkSensor(name, out string) *NetworkSensor { return &NetworkSensor{name: name, out: out} }
func (s *NetworkSensor) Name() string                  { return s.name }
func (s *NetworkSensor) Update(ctx context.Context, bb Blackboard) error {
	if v, ok := bb.Get("network_check"); ok {
		if f, ok2 := v.(func(context.Context) bool); ok2 {
			bb.Set(s.out+".ok", f(ctx))
			return nil
		}
	}
	if v, ok := bb.Get("network.ok"); ok {
		if b, ok2 := v.(bool); ok2 {
			bb.Set(s.out+".ok", b)
			return nil
		}
	}
	bb.Set(s.out+".ok", false)
	return nil
}

// DatabaseSensor relies on a BB function 'db_ping' -> error
type DatabaseSensor struct{ name, out string }

func NewDatabaseSensor(name, out string) *DatabaseSensor {
	return &DatabaseSensor{name: name, out: out}
}
func (s *DatabaseSensor) Name() string { return s.name }
func (s *DatabaseSensor) Update(ctx context.Context, bb Blackboard) error {
	if v, ok := bb.Get("db_ping"); ok {
		if f, ok2 := v.(func(context.Context) error); ok2 {
			err := f(ctx)
			bb.Set(s.out+".ok", err == nil)
			return nil
		}
	}
	bb.Set(s.out+".ok", false)
	return nil
}

// helpers for timers and system runtime isolated for testability
func timeNowMS() int64 { return timeNow().UnixNano() / 1e6 }

var (
	timeNow             = func() time.Time { return time.Now() }
	runtimeNumGoroutine = func() int { return runtime.NumGoroutine() }
	runtimeMemAlloc     = func() uint64 { var m runtime.MemStats; runtime.ReadMemStats(&m); return m.Alloc }
)
