package npcv2

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// Example built-in sensors

// DistanceSensor computes distance between two 2D points stored in blackboard keys: src(x,y) and dst(x,y)
// Params: src_x, src_y, dst_x, dst_y (keys). Writes distance to key 'out'

type DistanceSensor struct {
	name string
	srcX string
	srcY string
	dstX string
	dstY string
	out  string
}

func NewDistanceSensor(name string, srcX, srcY, dstX, dstY, out string) *DistanceSensor {
	return &DistanceSensor{name: name, srcX: srcX, srcY: srcY, dstX: dstX, dstY: dstY, out: out}
}

func (d *DistanceSensor) Name() string { return d.name }

func (d *DistanceSensor) Update(_ context.Context, bb Blackboard) error {
	x1, ok1 := getFloat(bb, d.srcX)
	y1, ok2 := getFloat(bb, d.srcY)
	x2, ok3 := getFloat(bb, d.dstX)
	y2, ok4 := getFloat(bb, d.dstY)
	if !(ok1 && ok2 && ok3 && ok4) {
		return fmt.Errorf("distance sensor missing coordinates")
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

func (i *InventorySensor) Update(ctx context.Context, bb Blackboard) error {
	v, ok := getFloat(bb, i.invKey)
	if !ok {
		return errors.New("inventory sensor: invalid inv value type")
	}
	bb.Set(i.out, v >= i.threshold)
	return nil
}

// RegisterSensors registers built-in sensors in registry.
func RegisterSensors(r Registry) {
	r.RegisterSensor("DistanceSensor", func(params map[string]any) (Sensor, error) {
		srcX, _ := params["src_x"].(string)
		srcY, _ := params["src_y"].(string)
		dstX, _ := params["dst_x"].(string)
		dstY, _ := params["dst_y"].(string)
		out, _ := params["out"].(string)
		if srcX == "" || srcY == "" || dstX == "" || dstY == "" || out == "" {
			return nil, fmt.Errorf("DistanceSensor requires src_x,src_y,dst_x,dst_y,out")
		}
		return NewDistanceSensor("DistanceSensor", srcX, srcY, dstX, dstY, out), nil
	})

	r.RegisterSensor("InventorySensor", func(params map[string]any) (Sensor, error) {
		key, _ := params["key"].(string)
		out, _ := params["out"].(string)
		thr := 0.0
		if v, ok := params["threshold"].(float64); ok {
			thr = v
		} else if v2, ok := params["threshold"].(int); ok {
			thr = float64(v2)
		}
		if key == "" || out == "" {
			return nil, fmt.Errorf("InventorySensor requires key,out")
		}
		return NewInventorySensor("InventorySensor", key, out, thr), nil
	})
}
