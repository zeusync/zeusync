package sync

// Manager provides a simple interface for managing variables
type Manager[T any] interface {
	// Create creates a new variable with the given name and initial value
	Create(name string, initialValue T, options ...Option) (Variable[T], error)

	// Get retrieves an existing variable by name
	Get(name string) (Variable[T], bool)

	// Delete removes a variable
	Delete(name string) bool

	// List returns all variable names
	List() []string

	// Stats returns usage statistics
	Stats() Metrics

	// Close closes the manager and all variables
	Close() error
}
