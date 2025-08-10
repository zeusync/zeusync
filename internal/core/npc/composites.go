package npc

import (
	"errors"
	"math/rand"
	"time"
)

// Composite nodes: Sequence, Selector, Parallel

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
		default:
			// continue to next child
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
		default:
			// try next child
		}
	}
	return StatusFailure, lastErr
}

type ParallelPolicy int

const (
	ParallelRequireAllSuccess ParallelPolicy = iota
	ParallelRequireOneSuccess
)

// Parallel executes all children and returns based on a policy.
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
		default:
			// Handle failure status
			continue
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

// Random composite picks a random child each tick and runs it.
// If no children – Success.
type Random struct {
	baseNode
	children []BehaviorNode
	rand     *rand.Rand
}

func NewRandom(name string, children ...BehaviorNode) *Random {
	return &Random{baseNode: baseNode{name: name}, children: children, rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (r *Random) SetChildren(children ...BehaviorNode) { r.children = children }

func (r *Random) Tick(t TickContext) (Status, error) {
	if len(r.children) == 0 {
		return StatusSuccess, nil
	}
	idx := r.rand.Intn(len(r.children))
	return r.children[idx].Tick(t)
}

// Priority composite executes children in order and returns on first non-failure
// It behaves like Selector but separates intent.
type Priority struct {
	baseNode
	children []BehaviorNode
}

func NewPriority(name string, children ...BehaviorNode) *Priority {
	return &Priority{baseNode: baseNode{name: name}, children: children}
}

func (p *Priority) SetChildren(children ...BehaviorNode) { p.children = children }

func (p *Priority) Tick(t TickContext) (Status, error) {
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
			return StatusSuccess, allErr
		case StatusRunning:
			return StatusRunning, allErr
		default:
			continue
		}
	}
	return StatusFailure, allErr
}
