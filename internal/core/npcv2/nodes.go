package npcv2

import (
	"errors"
	"math/rand"
	"time"
)

// Base node

type baseNode struct{ name string }

func (b baseNode) Name() string { return b.name }

// ActionFunc wraps a function as an Action node.

type ActionFunc struct {
	baseNode
	Fn func(t TickContext) (Status, error)
}

func (a ActionFunc) Tick(t TickContext) (Status, error) { return a.Fn(t) }

// ConditionFunc wraps a function as a Condition node.

type ConditionFunc struct {
	baseNode
	Fn func(t TickContext) (bool, error)
}

func (c ConditionFunc) Tick(t TickContext) (Status, error) {
	ok, err := c.Fn(t)
	if err != nil {
		return StatusFailure, err
	}
	if ok {
		return StatusSuccess, nil
	}
	return StatusFailure, nil
}

// Sequence runs children until one fails; success if all succeed; running if a child is running.

type Sequence struct {
	baseNode
	children []BehaviorNode
}

func NewSequence(name string, children ...BehaviorNode) *Sequence {
	return &Sequence{baseNode: baseNode{name: name}, children: children}
}

func (s *Sequence) SetChildren(children ...BehaviorNode) { s.children = children }

func (s *Sequence) Tick(t TickContext) (Status, error) {
	for _, ch := range s.children {
		st, err := ch.Tick(t)
		if err != nil {
			return StatusFailure, err
		}
		switch st {
		case StatusFailure:
			return StatusFailure, nil
		case StatusRunning:
			return StatusRunning, nil
		}
	}
	return StatusSuccess, nil
}

// Selector runs children until one succeeds; failure if all fail; running if a child is running.

type Selector struct {
	baseNode
	children []BehaviorNode
}

func NewSelector(name string, children ...BehaviorNode) *Selector {
	return &Selector{baseNode: baseNode{name: name}, children: children}
}

func (s *Selector) SetChildren(children ...BehaviorNode) { s.children = children }

func (s *Selector) Tick(t TickContext) (Status, error) {
	var lastErr error
	for _, ch := range s.children {
		st, err := ch.Tick(t)
		if err != nil {
			lastErr = err
		}
		switch st {
		case StatusSuccess:
			return StatusSuccess, err
		case StatusRunning:
			return StatusRunning, err
		}
	}
	return StatusFailure, lastErr
}

// Parallel executes all children and returns based on a policy.

type ParallelPolicy int

const (
	ParallelRequireAllSuccess ParallelPolicy = iota
	ParallelRequireOneSuccess
)

type Parallel struct {
	baseNode
	children []BehaviorNode
	policy   ParallelPolicy
}

func NewParallel(name string, policy ParallelPolicy, children ...BehaviorNode) *Parallel {
	return &Parallel{baseNode: baseNode{name: name}, policy: policy, children: children}
}

func (p *Parallel) SetChildren(children ...BehaviorNode) { p.children = children }

func (p *Parallel) Tick(t TickContext) (Status, error) {
	if len(p.children) == 0 {
		return StatusSuccess, nil
	}
	successes := 0
	var anyRunning bool
	var allErr error
	for _, ch := range p.children {
		st, err := ch.Tick(t)
		if err != nil {
			if allErr == nil {
				allErr = err
			} else {
				allErr = errors.Join(allErr, err)
			}
		}
		switch st {
		case StatusSuccess:
			successes++
		case StatusRunning:
			anyRunning = true
		}
	}
	switch p.policy {
	case ParallelRequireAllSuccess:
		if successes == len(p.children) {
			return StatusSuccess, allErr
		}
		if anyRunning {
			return StatusRunning, allErr
		}
		return StatusFailure, allErr
	case ParallelRequireOneSuccess:
		if successes > 0 {
			return StatusSuccess, allErr
		}
		if anyRunning {
			return StatusRunning, allErr
		}
		return StatusFailure, allErr
	default:
		return StatusFailure, errors.New("unknown parallel policy")
	}
}

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
	// in Blackboard we store start time under key name+".start"
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

// Tree is a simple DecisionTree implementation with a root node.

type Tree struct{ root BehaviorNode }

func (t Tree) Root() BehaviorNode { return t.root }

func (t Tree) Tick(tc TickContext) (Status, error) {
	if t.root == nil {
		return StatusSuccess, nil
	}
	return t.root.Tick(tc)
}
