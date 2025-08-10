package npc

import (
	"fmt"
	"time"
)

var DefaultRegistry = NewRegistry()

func init() {
	RegisterBuiltins(DefaultRegistry)
}

// Built-in basic decorators/actions/conditions/sensors for configuration convenience.
// We register a small set here to make out-of-the-box examples work.

// RegisterBuiltins registers simple reusable nodes into a Registry.
func RegisterBuiltins(r Registry) {
	// Conditions
	r.RegisterCondition("IsTrue", func(params map[string]any) (Condition, error) {
		key, _ := params["key"].(string)
		if key == "" {
			return nil, fmt.Errorf("IsTrue requires 'key'")
		}
		return ConditionFunc{baseNode: baseNode{name: "IsTrue(" + key + ")"}, Fn: func(t TickContext) (bool, error) {
			v, ok := t.BB.Get(key)
			if !ok {
				return false, nil
			}
			b, ok := v.(bool)
			return ok && b, nil
		}}, nil
	})

	// Actions
	r.RegisterAction("SetBool", func(params map[string]any) (Action, error) {
		key, _ := params["key"].(string)
		val, _ := params["value"].(bool)
		if key == "" {
			return nil, fmt.Errorf("SetBool requires 'key'")
		}
		return ActionFunc{baseNode: baseNode{name: "SetBool(" + key + ")"}, Fn: func(t TickContext) (Status, error) {
			t.BB.Set(key, val)
			return StatusSuccess, nil
		}}, nil
	})

	// Noop
	r.RegisterAction("Noop", func(params map[string]any) (Action, error) {
		return ActionFunc{baseNode: baseNode{name: "Noop"}, Fn: func(t TickContext) (Status, error) { return StatusSuccess, nil }}, nil
	})

	// Additional actions
	r.RegisterAction("SetValue", func(params map[string]any) (Action, error) {
		key, _ := params["key"].(string)
		if key == "" {
			return nil, fmt.Errorf("SetValue requires key")
		}
		val := params["value"]
		return NewSetValue("SetValue", key, val), nil
	})

	// Log
	r.RegisterAction("Log", func(params map[string]any) (Action, error) {
		key, _ := params["key"].(string)
		msg, _ := params["msg"].(string)
		if key == "" {
			return nil, fmt.Errorf("log requires key")
		}
		return NewLog("Log", key, msg), nil
	})

	// Wait
	r.RegisterAction("Wait", func(params map[string]any) (Action, error) {
		ms := 0
		if v, ok := params["ms"].(int); ok {
			ms = v
		} else if v2, ok := params["ms"].(float64); ok {
			ms = int(v2)
		}
		return NewWait("Wait", time.Duration(ms)*time.Millisecond), nil
	})

	// Decorators
	r.RegisterDecorator("Repeat", func(params map[string]any) (Decorator, error) {
		times := 1
		if v, ok := params["times"].(int); ok {
			times = v
		} else if fv, ok := params["times"].(float64); ok {
			times = int(fv)
		}
		stopOnFailure := false
		if v, ok := params["stop_on_failure"].(bool); ok {
			stopOnFailure = v
		}
		return NewRepeat("Repeat", times, stopOnFailure), nil
	})

	// Timer
	r.RegisterDecorator("Timer", func(params map[string]any) (Decorator, error) {
		ms := 0
		if v, ok := params["ms"].(int); ok {
			ms = v
		} else if fv, ok := params["ms"].(float64); ok {
			ms = int(fv)
		}
		return NewTimer("Timer", time.Duration(ms)*time.Millisecond), nil
	})

	// Inverter
	r.RegisterDecorator("Inverter", func(params map[string]any) (Decorator, error) {
		return NewInverter("Inverter"), nil
	})

	// Succeeder
	r.RegisterDecorator("Succeeder", func(params map[string]any) (Decorator, error) {
		return NewSucceeder("Succeeder"), nil
	})

	// Cooldown
	r.RegisterDecorator("Cooldown", func(params map[string]any) (Decorator, error) {
		ms := 0
		if v, ok := params["ms"].(int); ok {
			ms = v
		} else if fv, ok := params["ms"].(float64); ok {
			ms = int(fv)
		}
		successOnly := true
		if v, ok := params["success_only"].(bool); ok {
			successOnly = v
		}
		return NewCooldown("Cooldown", time.Duration(ms)*time.Millisecond, successOnly), nil
	})

	// Probability
	r.RegisterDecorator("Probability", func(params map[string]any) (Decorator, error) {
		p := 1.0
		if v, ok := params["p"].(float64); ok {
			p = v
		} else if v2, ok := params["prob"].(float64); ok {
			p = v2
		}
		return NewProbability("Probability", p), nil
	})

	// UntilSuccess
	r.RegisterDecorator("UntilSuccess", func(params map[string]any) (Decorator, error) {
		m := 0
		if v, ok := params["max"].(int); ok {
			m = v
		} else if v2, is := params["max"].(float64); is {
			m = int(v2)
		}
		return NewUntilSuccess("UntilSuccess", m), nil
	})

	// UntilFailure
	r.RegisterDecorator("UntilFailure", func(params map[string]any) (Decorator, error) {
		m := 0
		if v, ok := params["max"].(int); ok {
			m = v
		} else if v2, is := params["max"].(float64); is {
			m = int(v2)
		}
		return NewUntilFailure("UntilFailure", m), nil
	})

	// DistanceSensor
	r.RegisterSensor("DistanceSensor", func(params map[string]any) (Sensor, error) {
		srcX, _ := params["src_x"].(string)
		srcY, _ := params["src_y"].(string)
		dstX, _ := params["dst_x"].(string)
		dstY, _ := params["dst_y"].(string)
		srcKey, _ := params["src"].(string)
		dstKey, _ := params["dst"].(string)
		out, _ := params["out"].(string)
		if out == "" {
			return nil, fmt.Errorf("DistanceSensor requires 'out'")
		}
		// allow either object keys or scalar keys
		if (srcKey == "" || dstKey == "") && (srcX == "" || srcY == "" || dstX == "" || dstY == "") {
			return nil, fmt.Errorf("DistanceSensor requires either src,dst object keys or scalar src_x,src_y,dst_x,dst_y")
		}
		s := NewDistanceSensor("DistanceSensor", srcX, srcY, dstX, dstY, out)
		if srcKey != "" && dstKey != "" {
			s = s.WithObjects(srcKey, dstKey)
		}
		return s, nil
	})

	// InventorySensor
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

	// TimerSensor
	r.RegisterSensor("TimerSensor", func(params map[string]any) (Sensor, error) {
		out, _ := params["out"].(string)
		interval := 1000
		if v, ok := params["interval_ms"].(int); ok {
			interval = v
		} else if v2, ok := params["interval_ms"].(float64); ok {
			interval = int(v2)
		}
		if out == "" {
			return nil, fmt.Errorf("TimerSensor requires out")
		}
		return NewTimerSensor("TimerSensor", interval, out), nil
	})
	// SystemResourceSensor
	r.RegisterSensor("SystemResourceSensor", func(params map[string]any) (Sensor, error) {
		out, _ := params["out"].(string)
		if out == "" {
			out = "system"
		}
		return NewSystemResourceSensor("SystemResourceSensor", out), nil
	})
	// NetworkSensor
	r.RegisterSensor("NetworkSensor", func(params map[string]any) (Sensor, error) {
		out, _ := params["out"].(string)
		if out == "" {
			out = "network"
		}
		return NewNetworkSensor("NetworkSensor", out), nil
	})
	// DatabaseSensor
	r.RegisterSensor("DatabaseSensor", func(params map[string]any) (Sensor, error) {
		out, _ := params["out"].(string)
		if out == "" {
			out = "db"
		}
		return NewDatabaseSensor("DatabaseSensor", out), nil
	})
}
