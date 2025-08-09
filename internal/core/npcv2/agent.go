package npcv2

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"time"

	bus "github.com/zeusync/zeusync/internal/core/events/bus"
)

// agent is a default Agent implementation.
type agent struct {
	bb      Blackboard
	mem     Memory
	events  bus.EventBus
	tree    DecisionTree
	sensors []Sensor
	clock   func() time.Time
}

// NewAgent constructs a new Agent from components.
func NewAgent(bb Blackboard, mem Memory, eb bus.EventBus, tree DecisionTree, sensors []Sensor) Agent {
	if bb == nil {
		bb = NewBlackboard()
	}
	if mem == nil {
		mem = NewMemory()
	}
	if eb == nil {
		eb = NewEventBus()
	}
	return &agent{bb: bb, mem: mem, events: eb, tree: tree, sensors: sensors, clock: time.Now}
}

func (a *agent) Blackboard() Blackboard { return a.bb }
func (a *agent) Memory() Memory         { return a.mem }
func (a *agent) Events() bus.EventBus   { return a.events }

func (a *agent) Step(ctx context.Context) (Status, error) {
	// 1) sensors
	for _, s := range a.sensors {
		if err := s.Update(ctx, a.bb); err != nil {
			return StatusFailure, err
		}
	}
	// 2) behavior tree
	tc := TickContext{Ctx: ctx, BB: a.bb, Memory: a.mem, Clock: a.clock}
	start := a.clock()
	st, err := a.tree.Tick(tc)
	// 3) history
	a.mem.AppendDecision(DecisionRecord{Node: a.tree.Root().Name(), Status: st, Duration: a.clock().Sub(start), Timestamp: a.clock()})
	return st, err
}

func (a *agent) SaveState() ([]byte, error) {
	bbBytes, err := a.bb.MarshalBinary()
	if err != nil {
		return nil, err
	}
	memBytes, err := a.mem.Save()
	if err != nil {
		return nil, err
	}
	// Use gob to encode a compact binary snapshot
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(struct{ BB, Mem []byte }{BB: bbBytes, Mem: memBytes}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *agent) LoadState(b []byte) error {
	var state struct{ BB, Mem []byte }
	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&state); err != nil {
		return err
	}
	if len(state.BB) > 0 {
		if err := a.bb.UnmarshalBinary(state.BB); err != nil {
			return err
		}
	}
	if len(state.Mem) > 0 {
		if err := a.mem.Load(state.Mem); err != nil {
			return err
		}
	}
	return nil
}

// BuildAgentFromConfig helper to build a minimal agent with default components using a config and registry
func BuildAgentFromConfig(ctx context.Context, cfg *Config, reg Registry) (Agent, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	tree, sensors, err := cfg.Build(reg)
	if err != nil {
		return nil, err
	}
	return NewAgent(NewBlackboard(), NewMemory(), NewEventBus(), tree, sensors), nil
}
