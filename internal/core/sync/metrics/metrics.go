package metrics

import (
	"reflect"
	sc "sync"
	"time"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.Variable = (*Wrapper)(nil)

// Wrapper is a decorator for a sync.Variable that adds metrics collection.
type Wrapper struct {
	variable sync.Variable
	metrics  *sync.VariableMetrics
	mu       sc.RWMutex
}

// NewMetricsWrapper creates a new MetricsWrapper.
func NewMetricsWrapper(variable sync.Variable) *Wrapper {
	return &Wrapper{
		variable: variable,
		metrics: &sync.VariableMetrics{
			CreatedAt: time.Now(),
		},
	}
}

// Get implements the sync.Variable interface.
func (m *Wrapper) Get() (any, error) {
	start := time.Now()
	value, err := m.variable.Get()
	latency := time.Since(start)

	m.mu.Lock()
	m.metrics.ReadCount++
	m.metrics.AvgReadLatency = (m.metrics.AvgReadLatency*time.Duration(m.metrics.ReadCount-1) + latency) / time.Duration(m.metrics.ReadCount)
	m.metrics.LastAccessed = time.Now()
	if err != nil {
		m.metrics.ErrorCount++
	}
	m.mu.Unlock()

	return value, err
}

// Set implements the sync.Variable interface.
func (m *Wrapper) Set(value any) error {
	start := time.Now()
	err := m.variable.Set(value)
	latency := time.Since(start)

	m.mu.Lock()
	m.metrics.WriteCount++
	m.metrics.AvgWriteLatency = (m.metrics.AvgWriteLatency*time.Duration(m.metrics.WriteCount-1) + latency) / time.Duration(m.metrics.WriteCount)
	m.metrics.LastModified = time.Now()
	if err != nil {
		m.metrics.ErrorCount++
	}
	m.mu.Unlock()

	return err
}

// GetMetrics implements the sync.Variable interface.
func (m *Wrapper) GetMetrics() sync.VariableMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.metrics
}

// GetType implements the sync.Variable interface.
func (m *Wrapper) GetType() reflect.Type {
	return m.variable.GetType()
}

// GetHistory implements the sync.Variable interface.
func (m *Wrapper) GetHistory() []sync.HistoryEntry {
	return m.variable.GetHistory()
}

// Close implements the sync.Variable interface.
func (m *Wrapper) Close() error {
	return m.variable.Close()
}

// Size implements the sync.Variable interface.
func (m *Wrapper) Size() int64 {
	return m.variable.Size()
}

// IsDirty implements the sync.Variable interface.
func (m *Wrapper) IsDirty() bool {
	return m.variable.IsDirty()
}

// MarkClean implements the sync.Variable interface.
func (m *Wrapper) MarkClean() {
	m.variable.MarkClean()
}

// GetVersion implements the sync.Variable interface.
func (m *Wrapper) GetVersion() uint64 {
	return m.variable.GetVersion()
}

// GetDelta implements the sync.Variable interface.
func (m *Wrapper) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return m.variable.GetDelta(sinceVersion)
}

// ApplyDelta implements the sync.Variable interface.
func (m *Wrapper) ApplyDelta(deltas ...sync.Delta) error {
	return m.variable.ApplyDelta(deltas...)
}

// SetConflictResolver implements the sync.Variable interface.
func (m *Wrapper) SetConflictResolver(resolver sync.ConflictResolver) {
	m.variable.SetConflictResolver(resolver)
}

// OnChange implements the sync.Variable interface.
func (m *Wrapper) OnChange(f func(oldValue, newValue any)) {
	m.variable.OnChange(f)
}

// OnConflict implements the sync.Variable interface.
func (m *Wrapper) OnConflict(f func(local, remote any) any) {
	m.variable.OnConflict(f)
}

// GetPermissions implements the sync.Variable interface.
func (m *Wrapper) GetPermissions() sync.PermissionMask {
	return m.variable.GetPermissions()
}

// SetPermissions implements the sync.Variable interface.
func (m *Wrapper) SetPermissions(mask sync.PermissionMask) {
	m.variable.SetPermissions(mask)
}

// GetStorageStrategy implements the sync.Variable interface.
func (m *Wrapper) GetStorageStrategy() sync.StorageStrategy {
	return m.variable.GetStorageStrategy()
}

// CanMigrateTo implements the sync.Variable interface.
func (m *Wrapper) CanMigrateTo(strategy sync.StorageStrategy) bool {
	return m.variable.CanMigrateTo(strategy)
}
