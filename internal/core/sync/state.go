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

type PermissionMask uint8

const (
	PermissionRead  PermissionMask = 1 << iota
	PermissionWrite PermissionMask = 1 << iota
	PermissionCheck PermissionMask = 1 << iota

	PermissionAdmin = PermissionRead |
		PermissionWrite |
		PermissionCheck<<iota
)
