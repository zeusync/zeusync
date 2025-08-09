package npc

import (
	"encoding/json"
	"fmt"
	"time"
)

// BehaviorTreeConfig represents the configuration for a behavior tree
type BehaviorTreeConfig struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Root        *NodeConfig            `json:"root" yaml:"root"`
	Variables   map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`
	Version     string                 `json:"version,omitempty" yaml:"version,omitempty"`
}

// NodeConfig represents the configuration for a single node
type NodeConfig struct {
	Name        string                 `json:"name" yaml:"name"`
	Type        string                 `json:"type" yaml:"type"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Children    []*NodeConfig          `json:"children,omitempty" yaml:"children,omitempty"`
	Child       *NodeConfig            `json:"child,omitempty" yaml:"child,omitempty"`
	Enabled     bool                   `json:"enabled" yaml:"enabled"`
}

// AgentConfig represents the configuration for an AI agent
type AgentConfig struct {
	ID            string                 `json:"id" yaml:"id"`
	Name          string                 `json:"name" yaml:"name"`
	Description   string                 `json:"description,omitempty" yaml:"description,omitempty"`
	BehaviorTree  *BehaviorTreeConfig    `json:"behavior_tree" yaml:"behavior_tree"`
	Sensors       []*SensorConfig        `json:"sensors,omitempty" yaml:"sensors,omitempty"`
	Memory        *MemoryConfig          `json:"memory,omitempty" yaml:"memory,omitempty"`
	EventHandlers []*EventHandlerConfig  `json:"event_handlers,omitempty" yaml:"event_handlers,omitempty"`
	InitialData   map[string]interface{} `json:"initial_data,omitempty" yaml:"initial_data,omitempty"`
	UpdateRate    time.Duration          `json:"update_rate,omitempty" yaml:"update_rate,omitempty"`
	Active        bool                   `json:"active" yaml:"active"`
}

// SensorConfig represents the configuration for a sensor
type SensorConfig struct {
	Name           string                 `json:"name" yaml:"name"`
	Type           string                 `json:"type" yaml:"type"`
	Description    string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	UpdateInterval time.Duration          `json:"update_interval,omitempty" yaml:"update_interval,omitempty"`
	Enabled        bool                   `json:"enabled" yaml:"enabled"`
}

// MemoryConfig represents the configuration for memory system
type MemoryConfig struct {
	Type           string                 `json:"type" yaml:"type"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	MaxEvents      int                    `json:"max_events,omitempty" yaml:"max_events,omitempty"`
	RetentionTime  time.Duration          `json:"retention_time,omitempty" yaml:"retention_time,omitempty"`
	PersistentPath string                 `json:"persistent_path,omitempty" yaml:"persistent_path,omitempty"`
}

// EventHandlerConfig represents the configuration for event handlers
type EventHandlerConfig struct {
	Name       string                 `json:"name" yaml:"name"`
	Type       string                 `json:"type" yaml:"type"`
	EventType  string                 `json:"event_type" yaml:"event_type"`
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Priority   int                    `json:"priority,omitempty" yaml:"priority,omitempty"`
	Enabled    bool                   `json:"enabled" yaml:"enabled"`
}

// Validate validates the behavior tree configuration
func (btc *BehaviorTreeConfig) Validate() error {
	if btc.Name == "" {
		return fmt.Errorf("behavior tree name is required")
	}

	if btc.Root == nil {
		return fmt.Errorf("behavior tree root node is required")
	}

	return btc.Root.Validate()
}

// Validate validates the node configuration
func (nc *NodeConfig) Validate() error {
	if nc.Name == "" {
		return fmt.Errorf("node name is required")
	}

	if nc.Type == "" {
		return fmt.Errorf("node type is required")
	}

	// Validate children
	for i, child := range nc.Children {
		if err := child.Validate(); err != nil {
			return fmt.Errorf("child node %d validation failed: %w", i, err)
		}
	}

	// Validate single child (for decorators)
	if nc.Child != nil {
		if err := nc.Child.Validate(); err != nil {
			return fmt.Errorf("child node validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates the agent configuration
func (ac *AgentConfig) Validate() error {
	if ac.ID == "" {
		return fmt.Errorf("agent ID is required")
	}

	if ac.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	if ac.BehaviorTree != nil {
		if err := ac.BehaviorTree.Validate(); err != nil {
			return fmt.Errorf("behavior tree validation failed: %w", err)
		}
	}

	// Validate sensors
	for i, sensor := range ac.Sensors {
		if err := sensor.Validate(); err != nil {
			return fmt.Errorf("sensor %d validation failed: %w", i, err)
		}
	}

	// Validate event handlers
	for i, handler := range ac.EventHandlers {
		if err := handler.Validate(); err != nil {
			return fmt.Errorf("event handler %d validation failed: %w", i, err)
		}
	}

	return nil
}

// Validate validates the sensor configuration
func (sc *SensorConfig) Validate() error {
	if sc.Name == "" {
		return fmt.Errorf("sensor name is required")
	}

	if sc.Type == "" {
		return fmt.Errorf("sensor type is required")
	}

	return nil
}

// Validate validates the event handler configuration
func (ehc *EventHandlerConfig) Validate() error {
	if ehc.Name == "" {
		return fmt.Errorf("event handler name is required")
	}

	if ehc.Type == "" {
		return fmt.Errorf("event handler type is required")
	}

	if ehc.EventType == "" {
		return fmt.Errorf("event type is required")
	}

	return nil
}

// GetParameter retrieves a parameter value with type assertion
func (nc *NodeConfig) GetParameter(key string) (interface{}, bool) {
	if nc.Parameters == nil {
		return nil, false
	}

	value, exists := nc.Parameters[key]
	return value, exists
}

// GetStringParameter retrieves a string parameter
func (nc *NodeConfig) GetStringParameter(key string) (string, bool) {
	value, exists := nc.GetParameter(key)
	if !exists {
		return "", false
	}

	str, ok := value.(string)
	return str, ok
}

// GetIntParameter retrieves an int parameter
func (nc *NodeConfig) GetIntParameter(key string) (int, bool) {
	value, exists := nc.GetParameter(key)
	if !exists {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetFloatParameter retrieves a float64 parameter
func (nc *NodeConfig) GetFloatParameter(key string) (float64, bool) {
	value, exists := nc.GetParameter(key)
	if !exists {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

// GetBoolParameter retrieves a boolean parameter
func (nc *NodeConfig) GetBoolParameter(key string) (bool, bool) {
	value, exists := nc.GetParameter(key)
	if !exists {
		return false, false
	}

	b, ok := value.(bool)
	return b, ok
}

// GetDurationParameter retrieves a duration parameter
func (nc *NodeConfig) GetDurationParameter(key string) (time.Duration, bool) {
	value, exists := nc.GetParameter(key)
	if !exists {
		return 0, false
	}

	switch v := value.(type) {
	case string:
		duration, err := time.ParseDuration(v)
		return duration, err == nil
	case float64:
		return time.Duration(v), true
	case int:
		return time.Duration(v), true
	default:
		return 0, false
	}
}

// ToJSON converts the configuration to JSON
func (btc *BehaviorTreeConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(btc, "", "  ")
}

// FromJSON loads configuration from JSON
func (btc *BehaviorTreeConfig) FromJSON(data []byte) error {
	return json.Unmarshal(data, btc)
}

// Clone creates a deep copy of the configuration
func (btc *BehaviorTreeConfig) Clone() *BehaviorTreeConfig {
	data, _ := json.Marshal(btc)
	clone := &BehaviorTreeConfig{}
	json.Unmarshal(data, clone)
	return clone
}

// Clone creates a deep copy of the node configuration
func (nc *NodeConfig) Clone() *NodeConfig {
	data, _ := json.Marshal(nc)
	clone := &NodeConfig{}
	json.Unmarshal(data, clone)
	return clone
}
