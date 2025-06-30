package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeusync/zeusync/internal/core/sync"
)

func TestVariableManager(t *testing.T) {
	m := NewVariableManager()

	// Create a variable
	_, err := m.Create("myVar", 10)
	assert.NoError(t, err)

	// Get the variable
	myVar, ok := m.Get("myVar")
	assert.True(t, ok)

	// Check the value
	value, err := myVar.Get()
	assert.NoError(t, err)
	assert.Equal(t, 10, value)

	// Delete the variable
	err = m.Delete("myVar")
	assert.NoError(t, err)

	// Check that the variable is gone
	_, ok = m.Get("myVar")
	assert.False(t, ok)
}

func TestAutoOptimization(t *testing.T) {
	m := NewVariableManager()

	// Create a variable
	_, err := m.Create("myVar", 10, sync.WithMetrics(true))
	assert.NoError(t, err)

	// Enable auto-optimization
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.EnableAutoOptimization(ctx, 10*time.Millisecond)

	// Get the variable
	myVar, ok := m.Get("myVar")
	assert.True(t, ok)

	// Simulate some reads
	for i := 0; i < 100_000; i++ {
		_, _ = myVar.Get()
	}

	// Wait for auto-optimization to kick in
	time.Sleep(100 * time.Millisecond)

	t.Logf("Strategy: %d", myVar.GetStorageStrategy())

	// Check if the strategy has changed
	assert.NotEqual(t, sync.StrategyAtomic, myVar.GetStorageStrategy())
}
