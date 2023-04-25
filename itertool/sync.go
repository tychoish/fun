package itertool

import (
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
)

// Synchronized produces wraps an existing iterator with one that is
// protected by a mutex. The underling implementation provides an
// Unwrap method.
//
// Even when synchronized in this manner, Iterators are generally not
// safe for concurrent access from multiple go routines, as Next() and
// Value() calls may interleave. The underlying implementation has a
// special case with fun.IterateOne which does make it safe for
// concurrent use, if the iterator is only accessed using
// fun.IterateOne.
func Synchronize[T any](in fun.Iterator[T]) fun.Iterator[T] {
	return adt.NewIterator(&sync.Mutex{}, in)
}
