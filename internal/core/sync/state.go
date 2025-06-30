package sync

// StateManage defines the interface for managing the state of synchronized variables.
// This includes creating and restoring snapshots, as well as handling deltas.
type StateManage interface {
	// CreateSnapshot creates a snapshot of the current state.
	CreateSnapshot() (Snapshot, error)
	// RestoreSnapshot restores the state from a snapshot.
	RestoreSnapshot(snapshot Snapshot) error
	// GetDelta returns the changes since a specific version.
	GetDelta(fromVersion uint64) (Delta, error)
	// ApplyDelta applies a set of changes to the state.
	ApplyDelta(Delta) error

	// SetIntersetAres sets the area of interest for a client.
	SetIntersetAres(clientID string, area SpatialArea) error
	// GetRelevantState returns the relevant state for a client based on their area of interest.
	GetRelevantState(clientID string) (StateSubset, error)
}

// Snapshot represents a snapshot of the state.
type Snapshot interface{}

// SpatialArea defines a spatial area of interest.
type SpatialArea struct{}

// StateSubset represents a subset of the state.
type StateSubset struct{}

// PermissionMask defines a bitmask for access permissions.
type PermissionMask uint16

const (
	// PermissionRead allows reading the variable.
	PermissionRead PermissionMask = 1 << iota
	// PermissionWrite allows writing to the variable.
	PermissionWrite
	// PermissionDelete allows deleting the variable.
	PermissionDelete
	// PermissionMigrate allows migrating the variable.
	PermissionMigrate
	// PermissionAdmin gives full access to the variable.
	PermissionAdmin
)
