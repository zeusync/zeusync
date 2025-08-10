package npc

import (
	"errors"
	"math/rand"
	"time"
)

// Decorator nodes: Repeat, Timer, Probability, Inverter, Succeeder, Cooldown

// Repeat decorator repeats its child a fixed number of times or until failure.

type Repeat struct {
	baseNode
	child BehaviorNode
	Times int
	// StopOnFailure stops repeating on failure when true.
	StopOnFailure bool
}

func NewRepeat(name string, times int, stopOnFailure bool) *Repeat {
	return &Repeat{baseNode: baseNode{name: name}, Times: times, StopOnFailure: stopOnFailure}
}

func (r *Repeat) SetChild(child BehaviorNode) { r.child = child }

func (r *Repeat) Tick(t TickContext) (Status, error) {
	if r.child == nil {
		return StatusFailure, errors.New("repeat: child is nil")
	}
	for i := 0; i < r.Times; i++ {
		st, err := r.child.Tick(t)
		if err != nil {
			return StatusFailure, err
		}
		if st == StatusRunning {
			return StatusRunning, nil
		}
		if st == StatusFailure && r.StopOnFailure {
			return StatusFailure, nil
		}
	}
	return StatusSuccess, nil
}

// Timer decorator limits child execution by duration; returns Running until timeout, then child's status.

type Timer struct {
	baseNode
	child    BehaviorNode
	Duration time.Duration
	// in Blackboard, we store start time under key name+".start"
}

func NewTimer(name string, d time.Duration) *Timer {
	return &Timer{baseNode: baseNode{name: name}, Duration: d}
}

func (d *Timer) SetChild(child BehaviorNode) { d.child = child }

func (d *Timer) Tick(t TickContext) (Status, error) {
	if d.child == nil {
		return StatusFailure, errors.New("timer: child is nil")
	}
	key := d.name + ".start"
	val, ok := t.BB.Get(key)
	now := t.Clock()
	if !ok {
		t.BB.Set(key, now)
		return StatusRunning, nil
	}
	start, _ := val.(time.Time)
	if now.Sub(start) < d.Duration {
		return StatusRunning, nil
	}
	// time's up – run child once
	st, err := d.child.Tick(t)
	// cleanup start key for next cycles
	t.BB.Delete(key)
	return st, err
}

// Probability decorator succeeds based on chance before executing child; otherwise fails fast.

type Probability struct {
	baseNode
	child BehaviorNode
	P     float64
	rand  *rand.Rand
}

func NewProbability(name string, p float64) *Probability {
	return &Probability{baseNode: baseNode{name: name}, P: p, rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (p *Probability) SetChild(child BehaviorNode) { p.child = child }

func (p *Probability) Tick(t TickContext) (Status, error) {
	if p.child == nil {
		return StatusFailure, errors.New("probability: child is nil")
	}
	if p.P <= 0 {
		return StatusFailure, nil
	}
	if p.P >= 1 {
		return p.child.Tick(t)
	}
	if p.rand.Float64() <= p.P {
		return p.child.Tick(t)
	}
	return StatusFailure, nil
}

// Inverter decorator flips Success <-> Failure; Running passes through.

type Inverter struct {
	baseNode
	child BehaviorNode
}

func NewInverter(name string) *Inverter { return &Inverter{baseNode: baseNode{name: name}} }

func (d *Inverter) SetChild(child BehaviorNode) { d.child = child }

func (d *Inverter) Tick(t TickContext) (Status, error) {
	if d.child == nil {
		return StatusFailure, errors.New("inverter: child is nil")
	}
	st, err := d.child.Tick(t)
	switch st {
	case StatusSuccess:
		return StatusFailure, err
	case StatusFailure:
		return StatusSuccess, err
	default:
		return st, err
	}
}

// Succeeder decorator always returns Success unless Running; Running passes through.

type Succeeder struct {
	baseNode
	child BehaviorNode
}

func NewSucceeder(name string) *Succeeder { return &Succeeder{baseNode: baseNode{name: name}} }

func (d *Succeeder) SetChild(child BehaviorNode) { d.child = child }

func (d *Succeeder) Tick(t TickContext) (Status, error) {
	if d.child == nil {
		return StatusSuccess, nil
	}
	st, err := d.child.Tick(t)
	if st == StatusRunning {
		return StatusRunning, err
	}
	return StatusSuccess, err
}

// Cooldown decorator prevents executing child until a duration elapsed since last allowed run.
// If cooling down, returns Failure so selectors can try alternatives.
// Params: Duration; SuccessOnly (when true, only set cooldown after child succeeded).

type Cooldown struct {
	baseNode
	child       BehaviorNode
	Duration    time.Duration
	SuccessOnly bool
}

func NewCooldown(name string, d time.Duration, successOnly bool) *Cooldown {
	return &Cooldown{baseNode: baseNode{name: name}, Duration: d, SuccessOnly: successOnly}
}

func (d *Cooldown) SetChild(child BehaviorNode) { d.child = child }

func (d *Cooldown) Tick(t TickContext) (Status, error) {
	if d.child == nil {
		return StatusFailure, errors.New("cooldown: child is nil")
	}
	key := d.name + ".last"
	now := t.Clock()
	if v, ok := t.BB.Get(key); ok {
		if last, ok2 := v.(time.Time); ok2 {
			if now.Sub(last) < d.Duration {
				return StatusFailure, nil
			}
		}
	}
	st, err := d.child.Tick(t)
	if d.SuccessOnly {
		if st == StatusSuccess {
			t.BB.Set(key, now)
		}
	} else {
		// set cooldown on any terminal status (not Running)
		if st != StatusRunning {
			t.BB.Set(key, now)
		}
	}
	return st, err
}
