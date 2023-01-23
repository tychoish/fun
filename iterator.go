// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Iterators, error handling, a
// native-feeling Set type, and a simple pub-sub framework for
// distributing messages in fan-out patterns.
package fun

import (
	"context"
)

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
//
// The itertool package provides a number of tools and paradigms for
// creating and processing Iterator objects. In general, Iterators
// cannot be safe for access from multiple concurrent goroutines,
// because it is impossible to synchronize calls to Next() and
// Value(); however, itertool.Range() and itertool.Split provide
// support for these workloads.
type Iterator[T any] interface {
	Next(context.Context) bool
	Close(context.Context) error
	Value() T
}
