package npcv2

import (
	"fmt"
	"sync"
)

// reg is an in-memory registry for plug-and-play modules.
type reg struct {
	mu    sync.RWMutex
	acts  map[string]func(map[string]any) (Action, error)
	conds map[string]func(map[string]any) (Condition, error)
	decos map[string]func(map[string]any) (Decorator, error)
	comps map[string]func(map[string]any) (Composite, error)
	sens  map[string]func(map[string]any) (Sensor, error)
}

// NewRegistry returns an empty registry.
func NewRegistry() Registry {
	return &reg{
		acts:  make(map[string]func(map[string]any) (Action, error)),
		conds: make(map[string]func(map[string]any) (Condition, error)),
		decos: make(map[string]func(map[string]any) (Decorator, error)),
		comps: make(map[string]func(map[string]any) (Composite, error)),
		sens:  make(map[string]func(map[string]any) (Sensor, error)),
	}
}

func (r *reg) RegisterAction(name string, factory func(map[string]any) (Action, error)) {
	r.mu.Lock()
	r.acts[name] = factory
	r.mu.Unlock()
}

func (r *reg) RegisterCondition(name string, factory func(map[string]any) (Condition, error)) {
	r.mu.Lock()
	r.conds[name] = factory
	r.mu.Unlock()
}

func (r *reg) RegisterDecorator(name string, factory func(map[string]any) (Decorator, error)) {
	r.mu.Lock()
	r.decos[name] = factory
	r.mu.Unlock()
}

func (r *reg) RegisterComposite(name string, factory func(map[string]any) (Composite, error)) {
	r.mu.Lock()
	r.comps[name] = factory
	r.mu.Unlock()
}

func (r *reg) RegisterSensor(name string, factory func(map[string]any) (Sensor, error)) {
	r.mu.Lock()
	r.sens[name] = factory
	r.mu.Unlock()
}

func (r *reg) NewAction(name string, params map[string]any) (Action, error) {
	r.mu.RLock()
	f := r.acts[name]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("unknown action: %s", name)
	}
	return f(params)
}

func (r *reg) NewCondition(name string, params map[string]any) (Condition, error) {
	r.mu.RLock()
	f := r.conds[name]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("unknown condition: %s", name)
	}
	return f(params)
}

func (r *reg) NewDecorator(name string, params map[string]any) (Decorator, error) {
	r.mu.RLock()
	f := r.decos[name]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("unknown decorator: %s", name)
	}
	return f(params)
}

func (r *reg) NewComposite(name string, params map[string]any) (Composite, error) {
	r.mu.RLock()
	f := r.comps[name]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("unknown composite: %s", name)
	}
	return f(params)
}

func (r *reg) NewSensor(name string, params map[string]any) (Sensor, error) {
	r.mu.RLock()
	f := r.sens[name]
	r.mu.RUnlock()
	if f == nil {
		return nil, fmt.Errorf("unknown sensor: %s", name)
	}
	return f(params)
}
