package npcv2

import (
	"bytes"
	"encoding/gob"
	"sync"
)

// simpleMemory keeps an in-memory list of decision records with binary (gob) persistence.
type simpleMemory struct {
	mu   sync.RWMutex
	list []DecisionRecord
}

// NewMemory creates a new memory implementation.
func NewMemory() Memory { return &simpleMemory{list: make([]DecisionRecord, 0, 128)} }

func (m *simpleMemory) AppendDecision(rec DecisionRecord) {
	m.mu.Lock()
	m.list = append(m.list, rec)
	m.mu.Unlock()
}

func (m *simpleMemory) History() []DecisionRecord {
	m.mu.RLock()
	cp := make([]DecisionRecord, len(m.list))
	copy(cp, m.list)
	m.mu.RUnlock()
	return cp
}

func (m *simpleMemory) Reset() {
	m.mu.Lock()
	m.list = m.list[:0]
	m.mu.Unlock()
}

func (m *simpleMemory) Save() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(m.list); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *simpleMemory) Load(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dec := gob.NewDecoder(bytes.NewReader(b))
	return dec.Decode(&m.list)
}
