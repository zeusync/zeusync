package sync

type StateManage interface {
	CreateSnapshot() (Snapshot, error)
	RestoreSnapshot(snapshot Snapshot) error
	GetDelta(fromVersion uint64) (Delta, error)
	ApplyDelta(Delta) error

	SetIntersetAres(clientID string, area SpatialArea) error
	GetRelevantState(clientID string) (StateSubset, error)
}

type Snapshot interface{}
type Delta interface{}
type SpatialArea struct{}
type StateSubset struct{}
