package npc

import (
	"errors"
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
