package npcv2

import (
	"bytes"
	"encoding/gob"
	"sort"
	"strings"
	"sync"
)

// bbMap is a thread-safe map-based blackboard implementation.
type bbMap struct {
	mu     sync.RWMutex
	data   map[string]any
	prefix string // for namespaces, empty for root
	root   *bbMap // pointer to root map for namespaced views
}

// NewBlackboard creates a new root blackboard.
func NewBlackboard() Blackboard {
	m := &bbMap{data: make(map[string]any)}
	m.root = m
	return m
}

func (b *bbMap) fullKey(key string) string {
	if b.prefix == "" {
		return key
	}
	return b.prefix + ":" + key
}

func (b *bbMap) Get(key string) (any, bool) {
	bb := b.root
	full := b.fullKey(key)
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	v, ok := bb.data[full]
	return v, ok
}

func (b *bbMap) Set(key string, value any) {
	bb := b.root
	full := b.fullKey(key)
	bb.mu.Lock()
	bb.data[full] = value
	bb.mu.Unlock()
}

func (b *bbMap) Delete(key string) {
	bb := b.root
	full := b.fullKey(key)
	bb.mu.Lock()
	delete(bb.data, full)
	bb.mu.Unlock()
}

func (b *bbMap) Namespace(ns string) Blackboard {
	if strings.Contains(ns, ":") {
		// sanitize nested prefix usage by replacing ':' with '_'
		ns = strings.ReplaceAll(ns, ":", "_")
	}
	return &bbMap{root: b.root, prefix: ns}
}

func (b *bbMap) Keys() []string {
	bb := b.root
	bb.mu.RLock()
	keys := make([]string, 0, len(bb.data))
	for k := range bb.data {
		keys = append(keys, k)
	}
	bb.mu.RUnlock()
	sort.Strings(keys)
	if b.prefix == "" {
		return keys
	}
	// filter by prefix and strip it
	res := make([]string, 0)
	pref := b.prefix + ":"
	for _, k := range keys {
		if strings.HasPrefix(k, pref) {
			res = append(res, strings.TrimPrefix(k, pref))
		}
	}
	return res
}

func (b *bbMap) MarshalBinary() ([]byte, error) {
	bb := b.root
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(bb.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *bbMap) UnmarshalBinary(data []byte) error {
	bb := b.root
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.data == nil {
		bb.data = make(map[string]any)
	}
	dec := gob.NewDecoder(bytes.NewReader(data))
	return dec.Decode(&bb.data)
}
