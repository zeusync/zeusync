package npcv2

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is a unified structure able to describe nodes in JSON or YAML.
// It uses a node registry by name to instantiate runtime structures.
type Config struct {
	Root    string                `json:"root" yaml:"root"`
	Nodes   map[string]ConfigNode `json:"nodes" yaml:"nodes"`
	Sensors []ConfigSensor        `json:"sensors" yaml:"sensors"`
}

type ConfigSensor struct {
	Name   string         `json:"name" yaml:"name"`
	Type   string         `json:"type" yaml:"type"`
	Params map[string]any `json:"params" yaml:"params"`
}

type ConfigNode struct {
	Type      string         `json:"type" yaml:"type"`
	Children  []string       `json:"children,omitempty" yaml:"children,omitempty"`
	Child     string         `json:"child,omitempty" yaml:"child,omitempty"`
	Action    string         `json:"action,omitempty" yaml:"action,omitempty"`
	Condition string         `json:"condition,omitempty" yaml:"condition,omitempty"`
	Params    map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// LoadJSON loads config from JSON reader.
func LoadJSON(r io.Reader) (*Config, error) {
	var c Config
	dec := json.NewDecoder(r)
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// LoadYAML loads config from YAML reader.
func LoadYAML(r io.Reader) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Build constructs the decision tree and sensors from config using a registry.
func (c *Config) Build(reg Registry) (DecisionTree, []Sensor, error) {
	if c.Root == "" {
		return Tree{}, nil, nil
	}
	// create node instances on demand with memoization
	created := make(map[string]BehaviorNode)
	var buildNode func(name string) (BehaviorNode, error)
	buildNode = func(name string) (BehaviorNode, error) {
		if n, ok := created[name]; ok {
			return n, nil
		}
		nc, ok := c.Nodes[name]
		if !ok {
			return nil, fmt.Errorf("unknown node in config: %s", name)
		}
		switch nc.Type {
		case "Sequence", "sequence":
			seq := NewSequence(name)
			children := make([]BehaviorNode, 0, len(nc.Children))
			for _, chname := range nc.Children {
				ch, err := buildNode(chname)
				if err != nil {
					return nil, err
				}
				children = append(children, ch)
			}
			seq.SetChildren(children...)
			created[name] = seq
			return seq, nil
		case "Selector", "selector":
			sel := NewSelector(name)
			children := make([]BehaviorNode, 0, len(nc.Children))
			for _, chname := range nc.Children {
				ch, err := buildNode(chname)
				if err != nil {
					return nil, err
				}
				children = append(children, ch)
			}
			sel.SetChildren(children...)
			created[name] = sel
			return sel, nil
		case "Parallel", "parallel":
			policy := ParallelRequireAllSuccess
			if v, ok := nc.Params["policy"]; ok {
				if s, ok := v.(string); ok && (s == "one" || s == "any") {
					policy = ParallelRequireOneSuccess
				}
			}
			par := NewParallel(name, policy)
			children := make([]BehaviorNode, 0, len(nc.Children))
			for _, chname := range nc.Children {
				ch, err := buildNode(chname)
				if err != nil {
					return nil, err
				}
				children = append(children, ch)
			}
			par.SetChildren(children...)
			created[name] = par
			return par, nil
		case "Decorator", "decorator":
			// select decorator by name in Action field for simplicity
			decName, _ := nc.Params["name"].(string)
			dec, err := reg.NewDecorator(decName, nc.Params)
			if err != nil {
				return nil, err
			}
			if nc.Child == "" {
				return nil, fmt.Errorf("decorator %s requires child", name)
			}
			ch, err := buildNode(nc.Child)
			if err != nil {
				return nil, err
			}
			dec.SetChild(ch)
			created[name] = dec
			return dec, nil
		case "Action", "action":
			a, err := reg.NewAction(nc.Action, nc.Params)
			if err != nil {
				return nil, err
			}
			created[name] = a
			return a, nil
		case "Condition", "condition":
			cnd, err := reg.NewCondition(nc.Condition, nc.Params)
			if err != nil {
				return nil, err
			}
			created[name] = cnd
			return cnd, nil
		default:
			return nil, fmt.Errorf("unsupported node type: %s", nc.Type)
		}
	}
	root, err := buildNode(c.Root)
	if err != nil {
		return nil, nil, err
	}
	// sensors
	sensors := make([]Sensor, 0, len(c.Sensors))
	for _, s := range c.Sensors {
		sen, err := reg.NewSensor(s.Type, s.Params)
		if err != nil {
			return nil, nil, fmt.Errorf("sensor %s: %w", s.Name, err)
		}
		sensors = append(sensors, sen)
	}
	return Tree{root: root}, sensors, nil
}

// Built-in basic decorators/actions/conditions for configuration convenience.
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
	r.RegisterAction("Noop", func(params map[string]any) (Action, error) {
		return ActionFunc{baseNode: baseNode{name: "Noop"}, Fn: func(t TickContext) (Status, error) { return StatusSuccess, nil }}, nil
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
	r.RegisterDecorator("Timer", func(params map[string]any) (Decorator, error) {
		ms := 0
		if v, ok := params["ms"].(int); ok {
			ms = v
		} else if fv, ok := params["ms"].(float64); ok {
			ms = int(fv)
		}
		return NewTimer("Timer", time.Duration(ms)*time.Millisecond), nil
	})
}
