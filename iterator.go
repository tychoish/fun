package fun

import (
	"context"
)

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
type Iterator[T any] interface {
	Next(context.Context) bool
	Close(context.Context) error
	Value() T
}
