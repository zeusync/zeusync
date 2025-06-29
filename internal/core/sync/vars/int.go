package vars

import (
	"encoding/gob"

	"github.com/zeusync/zeusync/internal/core/sync"
)

var _ sync.TypedVariable[int] = (*Int)(nil)

type Int struct {
	root sync.TypedVariable[int]
}

func NewInt(initialValue int, maxHistory uint8) *Int {
	gob.Register(initialValue)

	return &Int{
		root: NewMutexTyped(initialValue, maxHistory),
	}
}

func (v *Int) Get() (int, error) {
	return v.root.Get()
}

func (v *Int) Set(newValue int) error {
	return v.root.Set(newValue)
}

func (v *Int) IsDirty() bool {
	return v.root.IsDirty()
}

func (v *Int) MarkClean() {
	v.root.MarkClean()
}

func (v *Int) GetDelta(sinceVersion uint64) ([]sync.Delta, error) {
	return v.root.GetDelta(sinceVersion)
}

func (v *Int) ApplyDelta(delta ...sync.Delta) error {
	return v.root.ApplyDelta(delta...)
}

func (v *Int) SetConflictResolver(resolver sync.ConflictResolver) {
	v.root.SetConflictResolver(resolver)
}

func (v *Int) GetVersion() uint64 {
	return v.root.GetVersion()
}

func (v *Int) OnChange(eventHandler func(oldValue int, newValue int)) {
	v.root.OnChange(eventHandler)
}

func (v *Int) OnConflict(eventHandler func(local int, remote int) int) {
	v.root.OnConflict(eventHandler)
}

func (v *Int) GetPermissionMask() sync.PermissionMask {
	return v.root.GetPermissionMask()
}

func (v *Int) SetPermissionMask(mask sync.PermissionMask) {
	v.root.SetPermissionMask(mask)
}

func (v *Int) GetHistory() []sync.HistoryEntry {
	return v.root.GetHistory()
}
