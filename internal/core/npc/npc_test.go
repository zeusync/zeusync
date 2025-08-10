package npc

import (
	"bytes"
	"context"
	"testing"
)

func equals42(v any) bool {
	switch vv := v.(type) {
	case int:
		return vv == 42
	case int64:
		return vv == 42
	case float64:
		return vv == 42
	default:
		return false
	}
}

func TestLoadAndRunSimpleTree(t *testing.T) {
	jsonCfg := []byte(`{
  "root":"Root",
  "nodes":{
    "Root":{"type":"Sequence", "children":["IsReady","DoWork"]},
    "IsReady":{"type":"Condition","condition":"IsTrue","params":{"key":"ready"}},
    "DoWork":{"type":"Action","action":"SetBool","params":{"key":"done","value":true}}
  },
  "sensors":[]
}`)
	cfg, err := LoadJSON(bytes.NewReader(jsonCfg))
	if err != nil {
		t.Fatalf("load json: %v", err)
	}
	r := NewRegistry()
	RegisterBuiltins(r)
	RegisterSensors(r)
	tree, sensors, err := cfg.Build(r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	a := NewAgent(NewBlackboard(), NewMemory(), NewEventBus(), tree, sensors)
	a.Blackboard().Set("ready", true)
	st, err := a.Step(context.Background())
	if err != nil {
		t.Fatalf("step error: %v", err)
	}
	if st != StatusSuccess {
		t.Fatalf("expected success, got %v", st)
	}
	if v, ok := a.Blackboard().Get("done"); !ok || v != true {
		t.Fatalf("expected done=true, got %v,%v", v, ok)
	}
}

func TestAgentPersistence(t *testing.T) {
	jsonCfg := []byte(`{"root":"Root","nodes":{"Root":{"type":"Action","action":"Noop"}},"sensors":[]}`)
	cfg, err := LoadJSON(bytes.NewReader(jsonCfg))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := NewRegistry()
	RegisterBuiltins(r)
	RegisterSensors(r)
	tree, sensors, err := cfg.Build(r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	a := NewAgent(NewBlackboard(), NewMemory(), NewEventBus(), tree, sensors)
	a.Blackboard().Set("x", 42)
	if _, err = a.Step(context.Background()); err != nil {
		t.Fatalf("step: %v", err)
	}
	saved, err := a.SaveState()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	b := NewAgent(NewBlackboard(), NewMemory(), NewEventBus(), tree, sensors)
	if err = b.LoadState(saved); err != nil {
		t.Fatalf("load: %v", err)
	}
	if v, ok := b.Blackboard().Get("x"); !ok || !equals42(v) {
		t.Fatalf("restore failed: %v,%v", v, ok)
	}
}
