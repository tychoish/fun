package adt

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/seq"
)

type Sequence[T any] interface {
	Iterator() fun.Iterator[T]
	Add(T)
	Remove(int) error
}

// func DistributorSequence[T any](dist pubsub.Distributor[T]) Sequence[T] {}

func ChannelSequence[T any](ch <-chan T) Sequence[T] {
	// return DistributorSequence(pubsub.DistributorChannel(ch))
	return nil
}

// func DequeSequence[T any](dq pubsub.Deque[T]) Sequence[T] {}
// func QueueSequence[T any](dq pubsub.Queue[T]) Sequence[T] {}

func ListSequence[T any](lst *seq.List[T]) Sequence[T] { return nil }
func SliceSequence[T any](sl []T) Sequence[T]          { return nil }
