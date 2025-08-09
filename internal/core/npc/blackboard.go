package npc

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Blackboard represents the central data storage for AI agents
// It stores all shared data that can be accessed by different modules
type Blackboard struct {
	mu   sync.RWMutex
	data map[string]interface{}

	// Metadata for tracking changes
	lastUpdated map[string]time.Time
	version     int64
}

// NewBlackboard creates a new blackboard instance
func NewBlackboard() *Blackboard {
	return &Blackboard{
		data:        make(map[string]interface{}),
		lastUpdated: make(map[string]time.Time),
		version:     0,
	}
}

// Set stores a value in the blackboard
func (bb *Blackboard) Set(key string, value interface{}) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.data[key] = value
	bb.lastUpdated[key] = time.Now()
	bb.version++
}

// Get retrieves a value from the blackboard
func (bb *Blackboard) Get(key string) (interface{}, bool) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	value, exists := bb.data[key]
	return value, exists
}

// GetString retrieves a string value from the blackboard
func (bb *Blackboard) GetString(key string) (string, bool) {
	value, exists := bb.Get(key)
	if !exists {
		return "", false
	}

	str, ok := value.(string)
	return str, ok
}

// GetInt retrieves an int value from the blackboard
func (bb *Blackboard) GetInt(key string) (int, bool) {
	value, exists := bb.Get(key)
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

// GetFloat retrieves a float64 value from the blackboard
func (bb *Blackboard) GetFloat(key string) (float64, bool) {
	value, exists := bb.Get(key)
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

// GetBool retrieves a boolean value from the blackboard
func (bb *Blackboard) GetBool(key string) (bool, bool) {
	value, exists := bb.Get(key)
	if !exists {
		return false, false
	}

	b, ok := value.(bool)
	return b, ok
}

// Has checks if a key exists in the blackboard
func (bb *Blackboard) Has(key string) bool {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	_, exists := bb.data[key]
	return exists
}

// Delete removes a key from the blackboard
func (bb *Blackboard) Delete(key string) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	delete(bb.data, key)
	delete(bb.lastUpdated, key)
	bb.version++
}

// Keys returns all keys in the blackboard
func (bb *Blackboard) Keys() []string {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	keys := make([]string, 0, len(bb.data))
	for key := range bb.data {
		keys = append(keys, key)
	}
	return keys
}

// GetVersion returns the current version of the blackboard
func (bb *Blackboard) GetVersion() int64 {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	return bb.version
}

// GetLastUpdated returns the last update time for a key
func (bb *Blackboard) GetLastUpdated(key string) (time.Time, bool) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	t, exists := bb.lastUpdated[key]
	return t, exists
}

// Clear removes all data from the blackboard
func (bb *Blackboard) Clear() {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.data = make(map[string]interface{})
	bb.lastUpdated = make(map[string]time.Time)
	bb.version++
}

// ToJSON exports the blackboard data to JSON
func (bb *Blackboard) ToJSON() ([]byte, error) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	export := struct {
		Data        map[string]interface{} `json:"data"`
		LastUpdated map[string]time.Time   `json:"last_updated"`
		Version     int64                  `json:"version"`
	}{
		Data:        bb.data,
		LastUpdated: bb.lastUpdated,
		Version:     bb.version,
	}

	return json.Marshal(export)
}

// FromJSON imports blackboard data from JSON
func (bb *Blackboard) FromJSON(data []byte) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	var export struct {
		Data        map[string]interface{} `json:"data"`
		LastUpdated map[string]time.Time   `json:"last_updated"`
		Version     int64                  `json:"version"`
	}

	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to unmarshal blackboard data: %w", err)
	}

	bb.data = export.Data
	bb.lastUpdated = export.LastUpdated
	bb.version = export.Version

	return nil
}

// Clone creates a deep copy of the blackboard
func (bb *Blackboard) Clone() *Blackboard {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	clone := NewBlackboard()

	// Deep copy data
	for key, value := range bb.data {
		clone.data[key] = value
	}

	// Copy metadata
	for key, t := range bb.lastUpdated {
		clone.lastUpdated[key] = t
	}

	clone.version = bb.version

	return clone
}
