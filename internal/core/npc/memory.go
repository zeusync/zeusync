package npc

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// BasicMemory implements a basic in-memory storage system
type BasicMemory struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	events    []MemoryEvent
	maxEvents int
	retention time.Duration
}

// NewBasicMemory creates a new basic memory instance
func NewBasicMemory() *BasicMemory {
	return &BasicMemory{
		data:      make(map[string]interface{}),
		events:    make([]MemoryEvent, 0),
		maxEvents: 1000,
		retention: time.Hour * 24, // 24 hours default retention
	}
}

// NewBasicMemoryWithConfig creates a new basic memory with configuration
func NewBasicMemoryWithConfig(maxEvents int, retention time.Duration) *BasicMemory {
	return &BasicMemory{
		data:      make(map[string]interface{}),
		events:    make([]MemoryEvent, 0),
		maxEvents: maxEvents,
		retention: retention,
	}
}

// Store saves data to memory
func (bm *BasicMemory) Store(key string, value interface{}) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.data[key] = value
	return nil
}

// Retrieve gets data from memory
func (bm *BasicMemory) Retrieve(key string) (interface{}, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	value, exists := bm.data[key]
	return value, exists
}

// Remember adds an experience/event to memory
func (bm *BasicMemory) Remember(event MemoryEvent) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate ID if not provided
	if event.ID == "" {
		event.ID = fmt.Sprintf("mem_%d_%s", event.Timestamp.UnixNano(), event.Type)
	}

	// Add event to memory
	bm.events = append(bm.events, event)

	// Clean up old events if necessary
	bm.cleanupEvents()

	return nil
}

// Recall retrieves experiences based on criteria
func (bm *BasicMemory) Recall(criteria MemoryCriteria) ([]MemoryEvent, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var results []MemoryEvent

	for _, event := range bm.events {
		if bm.matchesCriteria(event, criteria) {
			results = append(results, event)
		}
	}

	// Sort by timestamp (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// Apply limit if specified
	if criteria.Limit > 0 && len(results) > criteria.Limit {
		results = results[:criteria.Limit]
	}

	return results, nil
}

// matchesCriteria checks if an event matches the given criteria
func (bm *BasicMemory) matchesCriteria(event MemoryEvent, criteria MemoryCriteria) bool {
	// Check type
	if criteria.Type != "" && event.Type != criteria.Type {
		return false
	}

	// Check tags
	if len(criteria.Tags) > 0 {
		hasTag := false
		for _, criteriaTag := range criteria.Tags {
			for _, eventTag := range event.Tags {
				if eventTag == criteriaTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// Check time range
	if criteria.TimeRange != nil {
		if event.Timestamp.Before(criteria.TimeRange.Start) ||
			event.Timestamp.After(criteria.TimeRange.End) {
			return false
		}
	}

	// Check importance
	if criteria.Importance != nil && event.Importance < *criteria.Importance {
		return false
	}

	// Check data criteria
	if len(criteria.Data) > 0 {
		for key, expectedValue := range criteria.Data {
			if eventValue, exists := event.Data[key]; !exists || eventValue != expectedValue {
				return false
			}
		}
	}

	return true
}

// cleanupEvents removes old events based on retention policy
func (bm *BasicMemory) cleanupEvents() {
	now := time.Now()
	cutoff := now.Add(-bm.retention)

	// Remove events older than retention time
	validEvents := make([]MemoryEvent, 0, len(bm.events))
	for _, event := range bm.events {
		if event.Timestamp.After(cutoff) {
			validEvents = append(validEvents, event)
		}
	}

	// If still too many events, keep only the most recent ones
	if len(validEvents) > bm.maxEvents {
		// Sort by timestamp (most recent first)
		sort.Slice(validEvents, func(i, j int) bool {
			return validEvents[i].Timestamp.After(validEvents[j].Timestamp)
		})
		validEvents = validEvents[:bm.maxEvents]
	}

	bm.events = validEvents
}

// Forget removes data from memory
func (bm *BasicMemory) Forget(key string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.data, key)
	return nil
}

// ForgetEvent removes a specific event from memory
func (bm *BasicMemory) ForgetEvent(eventID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for i, event := range bm.events {
		if event.ID == eventID {
			bm.events = append(bm.events[:i], bm.events[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("event with ID %s not found", eventID)
}

// Clear clears all memory
func (bm *BasicMemory) Clear() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.data = make(map[string]interface{})
	bm.events = make([]MemoryEvent, 0)
	return nil
}

// Save exports memory to persistent storage
func (bm *BasicMemory) Save() ([]byte, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	memoryState := struct {
		Data      map[string]interface{} `json:"data"`
		Events    []MemoryEvent          `json:"events"`
		MaxEvents int                    `json:"max_events"`
		Retention time.Duration          `json:"retention"`
	}{
		Data:      bm.data,
		Events:    bm.events,
		MaxEvents: bm.maxEvents,
		Retention: bm.retention,
	}

	return json.MarshalIndent(memoryState, "", "  ")
}

// Load imports memory from persistent storage
func (bm *BasicMemory) Load(data []byte) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	var memoryState struct {
		Data      map[string]interface{} `json:"data"`
		Events    []MemoryEvent          `json:"events"`
		MaxEvents int                    `json:"max_events"`
		Retention time.Duration          `json:"retention"`
	}

	if err := json.Unmarshal(data, &memoryState); err != nil {
		return fmt.Errorf("failed to unmarshal memory state: %w", err)
	}

	bm.data = memoryState.Data
	bm.events = memoryState.Events
	bm.maxEvents = memoryState.MaxEvents
	bm.retention = memoryState.Retention

	// Clean up old events after loading
	bm.cleanupEvents()

	return nil
}

// GetStats returns memory statistics
func (bm *BasicMemory) GetStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return map[string]interface{}{
		"data_entries": len(bm.data),
		"event_count":  len(bm.events),
		"max_events":   bm.maxEvents,
		"retention":    bm.retention.String(),
		"memory_usage": bm.calculateMemoryUsage(),
	}
}

// calculateMemoryUsage estimates memory usage (simplified)
func (bm *BasicMemory) calculateMemoryUsage() string {
	// This is a simplified calculation
	dataSize := len(bm.data) * 64     // Rough estimate per entry
	eventSize := len(bm.events) * 256 // Rough estimate per event
	totalBytes := dataSize + eventSize

	if totalBytes < 1024 {
		return fmt.Sprintf("%d B", totalBytes)
	} else if totalBytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(totalBytes)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(totalBytes)/(1024*1024))
	}
}

// GetEventsByType returns all events of a specific type
func (bm *BasicMemory) GetEventsByType(eventType string) []MemoryEvent {
	criteria := MemoryCriteria{
		Type: eventType,
	}

	events, _ := bm.Recall(criteria)
	return events
}

// GetRecentEvents returns the most recent events
func (bm *BasicMemory) GetRecentEvents(limit int) []MemoryEvent {
	criteria := MemoryCriteria{
		Limit: limit,
	}

	events, _ := bm.Recall(criteria)
	return events
}

// GetEventsByTag returns all events with a specific tag
func (bm *BasicMemory) GetEventsByTag(tag string) []MemoryEvent {
	criteria := MemoryCriteria{
		Tags: []string{tag},
	}

	events, _ := bm.Recall(criteria)
	return events
}

// GetImportantEvents returns events above a certain importance threshold
func (bm *BasicMemory) GetImportantEvents(minImportance float64) []MemoryEvent {
	criteria := MemoryCriteria{
		Importance: &minImportance,
	}

	events, _ := bm.Recall(criteria)
	return events
}

// SetRetentionPolicy updates the memory retention policy
func (bm *BasicMemory) SetRetentionPolicy(maxEvents int, retention time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.maxEvents = maxEvents
	bm.retention = retention

	// Clean up with new policy
	bm.cleanupEvents()
}
