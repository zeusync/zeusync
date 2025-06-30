package sync

type StateManage interface {
	CreateSnapshot() (Snapshot, error)
	RestoreSnapshot(snapshot Snapshot) error
	GetDelta(fromVersion uint64) (Delta, error)
	ApplyDelta(Delta) error

	SetIntersetAres(clientID string, area SpatialArea) error
	GetRelevantState(clientID string) (StateSubset, error)
}

type (
	Snapshot    interface{}
	SpatialArea struct{}
	StateSubset struct{}
)

type PermissionMask uint16

const (
	PermissionRead PermissionMask = 1 << iota
	PermissionWrite
	PermissionDelete
	PermissionMigrate
	PermissionAdmin
)
