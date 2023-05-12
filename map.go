package fun

import (
	"context"
	"sync"

	"github.com/tychoish/fun/internal"
)

// Map is just a generic type wrapper around a map, mostly for the
// purpose of being able to interact with Pair[K,V] objects and
// Iterators.
type Map[K comparable, V any] map[K]V

// Mapify provides a constructor that will produce a fun.Map without
// specifying types.
func Mapify[K comparable, V any](in map[K]V) Map[K, V] { return in }

// Pairs exports a map a Pairs object, which is an alias for a slice of
// Pair objects.
func (m Map[K, V]) Pairs() Pairs[K, V] { return MakePairs(m) }

// Add adds a key value pair directly to the map.
func (m Map[K, V]) Add(k K, v V) { m[k] = v }

// AddPair adds a Pair object to the map.
func (m Map[K, V]) AddPair(p Pair[K, V]) { m.Add(p.Key, p.Value) }

// Append adds a sequence of Pair objects to the map.
func (m Map[K, V]) Append(pairs ...Pair[K, V]) { m.Extend(pairs) }

// Len returns the length. It is equivalent to len(Map), but is
// provided for consistency.
func (m Map[K, V]) Len() int { return len(m) }

// Extend adds a sequence of Pairs to the map.
func (m Map[K, V]) Extend(pairs Pairs[K, V]) {
	for _, pair := range pairs {
		m.AddPair(pair)
	}
}

// ConsumeMap adds all the keys from the input map the map.
func (m Map[K, V]) ConsumeMap(in Map[K, V]) {
	for k, v := range in {
		m[k] = v
	}
}

// ConsumeSlice adds a slice of values to the map, using the provided
// function to generate the key for the value. Existing values in the
// map are overridden.
func (m Map[K, V]) ConsumeSlice(in []V, keyf func(V) K) {
	for idx := range in {
		value := in[idx]
		m[keyf(value)] = value
	}
}

// Consume adds items to the map from an iterator of Pair
// objects. Existing values for K are always overwritten.
func (m Map[K, V]) Consume(ctx context.Context, iter Iterator[Pair[K, V]]) {
	InvariantMust(Observe(ctx, iter, func(in Pair[K, V]) { m.AddPair(in) }))
}

// ConsumeValues adds items to the map, using the function to generate
// the keys for the values.
func (m Map[K, V]) ConsumeValues(ctx context.Context, iter Iterator[V], keyf func(V) K) {
	InvariantMust(Observe(ctx, iter, func(in V) { m[keyf(in)] = in }))
}

// Iterator converts a map into an iterator of fun.Pair objects. The
// iterator is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the iterator, cancel the context that you pass to the
// iterator OR call iterator.Close().
//
// To use this iterator the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other iterator processing tools
// (generators, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func (m Map[K, V]) Iterator() Iterator[Pair[K, V]] {
	iter := &internal.GeneratorIterator[Pair[K, V]]{}
	synciter := &internal.SyncIterImpl[Pair[K, V]]{
		Iter: iter,
		Mtx:  &sync.Mutex{},
	}

	once := internal.MnemonizeContext(func(ctx context.Context) <-chan Pair[K, V] {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)

		// setting this means that iter.Close() will
		// release this thread
		func() {
			synciter.Mtx.Lock()
			defer synciter.Mtx.Unlock()
			iter.Closer = cancel
		}()

		// now we make the pipe
		out := make(chan Pair[K, V])
		worker := WorkerFunc(func(ctx context.Context) error {
			defer close(out)
			for k, v := range m {
				if !Blocking(out).Check(ctx, MakePair(k, v)) {
					break
				}
			}
			return nil // worker
		})

		// worker.Safe makes this panic safe.and
		// ensures the context is fully canceled no
		// matter what
		go func() {
			defer cancel()
			synciter.Mtx.Lock()
			defer synciter.Mtx.Unlock()

			iter.Error = worker.Safe(ctx)
		}()

		return out
	})

	iter.Operation = func(ctx context.Context) (Pair[K, V], error) {
		pipe := once(ctx)
		return ReadOne(ctx, pipe)
	}

	return iter
}

// Keys provides an iterator over just the keys in the map.
func (m Map[K, V]) Keys() Iterator[K] { return PairKeys(m.Iterator()) }

// Values provides an iterator over just the values in the map.
func (m Map[K, V]) Values() Iterator[V] { return PairValues(m.Iterator()) }
