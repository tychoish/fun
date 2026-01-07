package shard

import (
	"sync/atomic"

	"github.com/tychoish/fun/adt"
)

// Versioned is a wrapper type that tracks the modification count for
// the wrapped value.
type Versioned[T any] struct {
	value adt.Atomic[T]
	clock atomic.Uint64
}

// Version constructs a new versioned object initialized with the given value.
// The initial version number will be 1. Uninitialized objects have version zero.
func Version[T any](in T) *Versioned[T]       { o := &Versioned[T]{}; o.Set(in); return o }
func (vv *Versioned[T]) innerLoad() T         { return vv.value.Load() }
func (vv *Versioned[T]) innerVersion() uint64 { return vv.clock.Load() }

// Ok returns true if this versioned object is not nil, false otherwise.
// This is a nil-safe way to check if the versioned object exists.
func (vv *Versioned[T]) Ok() bool { return vv != nil }

// Version returns the current version number of this object.
// Returns 0 if the object is nil. The version increments each time Set() is called.
func (vv *Versioned[T]) Version() (v uint64) {
	if vv != nil {
		v = vv.innerVersion()
	}
	return
}

// Load returns the current value stored in this versioned object.
// Returns the zero value if the object is nil. This operation is safe for concurrent access.
func (vv *Versioned[T]) Load() (out T) {
	if vv != nil {
		out = vv.innerLoad()
	}
	return
}

// Set stores a new value and increments the version number.
// This operation is safe for concurrent access and will atomically update both the value and version.
func (vv *Versioned[T]) Set(newValue T) { vv.clock.Add(1); vv.value.Store(newValue) }

// Fetch atomically retrieves both the current value and its version number.
// This method uses a retry loop to ensure the value and version are consistent,
// protecting against races where the value changes between reading the value and version.
// Returns the value and its corresponding version number.
func (vv *Versioned[T]) Fetch() (T, uint64) {
	for {
		version := vv.Version()
		val := vv.Load()
		if version == vv.Version() {
			return val, version
		}
	}
}
