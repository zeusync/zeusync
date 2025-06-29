package vars

import (
	sc "sync"

	"github.com/zeusync/zeusync/internal/core/sync"
)

// var _ sync.TypedVariable[int] = (*Int)(nil)
type Int struct {
	mx  sc.RWMutex
	val sync.TypedVariable[int]
}
