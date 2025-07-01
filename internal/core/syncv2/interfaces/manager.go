package interfaces

import (
	"github.com/zeusync/zeusync/internal/core/syncv2"
	"github.com/zeusync/zeusync/internal/core/syncv2/metrics"
)

// Manager provides a simple interface for managing variables
type Manager[T any] interface {
	// Create creates a new variable with the given name and initial value
	Create(name string, initialValue T, options ...sync.Option) (sync.Variable[T], error)

	// Get retrieves an existing variable by name
	Get(name string) (sync.Variable[T], bool)

	// Delete removes a variable
	Delete(name string) bool

	// List returns all variable names
	List() []string

	// Stats returns usage statistics
	Stats() metrics.Metrics

	// Close closes the manager and all variables
	Close() error
}
