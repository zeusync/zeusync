package npc

import (
	"fmt"
	"time"
)

// WaitAction waits for duration ms using BB clock ticks.
type WaitAction struct {
	baseNode
	dur time.Duration
}

func NewWait(name string, d time.Duration) Action {
	return WaitAction{baseNode: baseNode{name: name}, dur: d}
}

func (a WaitAction) Tick(t TickContext) (Status, error) {
	key := a.name + ".start"
	if v, ok := t.BB.Get(key); ok {
		if start, ok2 := v.(time.Time); ok2 {
			if t.Clock().Sub(start) >= a.dur {
				t.BB.Delete(key)
				return StatusSuccess, nil
			}
			return StatusRunning, nil
		}
	}
	t.BB.Set(key, t.Clock())
	return StatusRunning, nil
}

// LogAction appends a log string to BB under a key.
type LogAction struct {
	baseNode
	key string
	msg string
}

func NewLog(name, key, msg string) Action {
	return LogAction{baseNode: baseNode{name: name}, key: key, msg: msg}
}

func (a LogAction) Tick(t TickContext) (Status, error) {
	// very simple: store the last message
	t.BB.Set(a.key, a.msg)
	return StatusSuccess, nil
}

// SetValueAction sets BB[key] = value
type SetValueAction struct {
	baseNode
	key string
	val any
}

func NewSetValue(name, key string, val any) Action {
	return SetValueAction{baseNode: baseNode{name: name}, key: key, val: val}
}

func (a SetValueAction) Tick(t TickContext) (Status, error) {
	t.BB.Set(a.key, a.val)
	return StatusSuccess, nil
}

// CalculateAction supports op on a and b and writes to out.
type CalculateAction struct {
	baseNode
	op       string
	leftKey  string
	rightKey string
	out      string
}

func NewCalculate(name, op, leftKey, rightKey, out string) Action {
	return CalculateAction{baseNode: baseNode{name: name}, op: op, leftKey: leftKey, rightKey: rightKey, out: out}
}

func (a CalculateAction) Tick(t TickContext) (Status, error) {
	lv, ok1 := getFloat(t.BB, a.leftKey)
	rv, ok2 := getFloat(t.BB, a.rightKey)
	if !(ok1 && ok2) {
		return StatusFailure, fmt.Errorf("calc: operands missing")
	}
	switch a.op {
	case "+":
		t.BB.Set(a.out, lv+rv)
	case "-":
		t.BB.Set(a.out, lv-rv)
	case "*":
		t.BB.Set(a.out, lv*rv)
	case "/":
		if rv == 0 {
			return StatusFailure, fmt.Errorf("division by zero")
		}
		t.BB.Set(a.out, lv/rv)
	default:
		return StatusFailure, fmt.Errorf("unknown op: %s", a.op)
	}
	return StatusSuccess, nil
}
