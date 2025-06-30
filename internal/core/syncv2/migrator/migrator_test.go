package migrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeusync/zeusync/internal/core/sync"
	"github.com/zeusync/zeusync/internal/core/sync/factory"
)

func TestMigrator(t *testing.T) {
	f := factory.NewVariableFactory()
	migrator := NewMigrator(f)

	// Create a variable with the atomic strategy
	atomicVar, err := f.CreateWithStrategy(sync.StrategyAtomic, 10)
	assert.NoError(t, err)

	// Migrate to the mutex strategy
	mutexVar, err := migrator.Migrate(atomicVar, sync.StrategyMutex)
	assert.NoError(t, err)

	// Check the new strategy
	assert.Equal(t, sync.StrategyMutex, mutexVar.GetStorageStrategy())

	// Check the value
	value, err := mutexVar.Get()
	assert.NoError(t, err)
	assert.Equal(t, 10, value)
}
