package shard

import (
	"sync/atomic"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ft"
)

// Versioned is a wrapper type that tracks the modification count for
// the wrapped value.
type Versioned[T any] struct {
	value adt.Atomic[T]
	clock atomic.Uint64
}

// Version constructs a new versioned object. Uninitialized objects
// have version zero, while all objects that have values set have
// version 1.
func Version[T any](in T) *Versioned[T]       { o := &Versioned[T]{}; o.Set(in); return o }
func (vv *Versioned[T]) innerLoad() T         { return vv.value.Load() }
func (vv *Versioned[T]) innerVersion() uint64 { return vv.clock.Load() }

func (vv *Versioned[T]) Ok() bool        { return vv != nil }
func (vv *Versioned[T]) Version() uint64 { return ft.WhenDo(vv != nil, vv.innerVersion) }
func (vv *Versioned[T]) Load() T         { return ft.WhenDo(vv != nil, vv.innerLoad) }
func (vv *Versioned[T]) Set(newValue T)  { vv.clock.Add(1); vv.value.Store(newValue) }

func (vv *Versioned[T]) Fetch() (T, uint64) {
	for {
		version := vv.Version()
		val := vv.Load()
		if version == vv.Version() {
			return val, version
		}
	}
}
