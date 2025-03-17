package shard

import (
	"fmt"
	"hash/maphash"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

var hashSeed = adt.NewOnce(func() maphash.Seed { return maphash.MakeSeed() })

var hasherPool *adt.Pool[*maphash.Hash]

var numWorkers = fun.WorkerGroupConfNumWorkers

const (
	defaultSize                    = 32
	defaultShMap MapImplementation = MapImplementationSyncMap
)

func init() {
	hasherPool = &adt.Pool[*maphash.Hash]{}
	hasherPool.SetCleanupHook(func(h *maphash.Hash) *maphash.Hash { h.Reset(); return h })
	hasherPool.SetConstructor(func() *maphash.Hash { h := &maphash.Hash{}; h.SetSeed(hashSeed.Resolve()); return h })
	hasherPool.FinalizeSetup()
}

// Map behaves like a simple map but divides the contents of
// the map between a number of shards. The outer map itself is
// versioned (e.g. a vector clock which records the number of global
// modification operations against the map), as well as each
// shard.
//
// All version numbers are strictly increasing and are incremented
// before the modification: no-op modifications (as in setting a key
// to itself) do increment the version numbers. Operations that return
// versions, will not report stale data or version numbers.
//
// Use these versions to determine if the data you have is
// (potentially) stale. The versions, of course, track number of modifications
//
// There are no fully synchronized global operations--counts and
// iteration would require exclusive access to all of the data and the
// implementation doesn't contain provisions for this.
type Map[K comparable, V any] struct {
	sh    adt.Once[[]sh[K, V]]
	clock atomic.Uint64
	num   uint64
	imp   MapImplementation
}

// Setup initializes the shard with non-default shard size and backing map implementation.
func (m *Map[K, V]) Setup(n int, mi MapImplementation) {
	m.sh.Do(func() []sh[K, V] { return m.init(n, mi) })
}

func (m *Map[K, V]) init(n int, mi MapImplementation) []sh[K, V] {
	m.num = ft.Default(max(0, uint64(n)), defaultSize)
	m.imp = ft.Default(mi, MapImplementationSyncMap)
	return m.makeShards()
}

// String reports the type name, number of configured shards, and
// current version of the map.
func (m *Map[K, V]) String() string {
	return fmt.Sprintf("ShardedMap<%s> Shards(%d) Version(%d)", m.imp, m.num, m.clock.Load())
}

func (m *Map[K, V]) makeShards() []sh[K, V] {
	shards := make([]sh[K, V], m.num)
	for idx := range shards {
		shards[idx].data = shards[idx].makeVmap(m.imp)
	}
	return shards
}

func (m *Map[K, V]) defaultShards() []sh[K, V]                      { return m.init(defaultSize, defaultShMap) }
func (m *Map[K, V]) shards() dt.Slice[sh[K, V]]                     { return m.sh.Call(m.defaultShards) }
func (m *Map[K, V]) shard(key K) *sh[K, V]                          { return m.shards().Ptr(int(m.shardID(key))) }
func (m *Map[K, V]) inc() *Map[K, V]                                { m.clock.Add(1); return m }
func to[T, O any](in func(T) O) fun.Transform[T, O]                 { return fun.Converter(in) }
func (m *Map[K, V]) sk() fun.Transform[*sh[K, V], *fun.Iterator[K]] { return to(m.shKeys) }
func (m *Map[K, V]) sv() fun.Transform[*sh[K, V], *fun.Iterator[V]] { return to(m.shVals) }
func (m *Map[K, V]) vp() fun.Transform[*sh[K, V], *fun.Iterator[V]] { return to(m.shValsp) }
func (m *Map[K, V]) keyToItem() fun.Transform[K, MapItem[K, V]]     { return to(m.Fetch) }
func (*Map[K, V]) shKeys(sh *sh[K, V]) *fun.Iterator[K]             { return sh.keys() }
func (*Map[K, V]) shVals(sh *sh[K, V]) *fun.Iterator[V]             { return sh.values() }
func (m *Map[K, V]) shValsp(sh *sh[K, V]) *fun.Iterator[V]          { return sh.valuesp(m.num) }
func (m *Map[K, V]) shPtrs() dt.Slice[*sh[K, V]]                    { return m.shards().Ptrs() }
func (m *Map[K, V]) shIter() *fun.Iterator[*sh[K, V]]               { return m.shPtrs().Iterator() }
func (m *Map[K, V]) keyItr() *fun.Iterator[*fun.Iterator[K]]        { return m.sk().Process(m.shIter()) }
func (m *Map[K, V]) valItr() *fun.Iterator[*fun.Iterator[V]]        { return m.sv().Process(m.shIter()) }
func (m *Map[K, V]) valItrp() *fun.Iterator[*fun.Iterator[V]]       { return m.vp().Process(m.shIter()) }
func (m *Map[K, V]) popt() fun.OptionProvider[*fun.WorkerGroupConf] { return numWorkers(int(m.num)) }

func (m *Map[K, V]) shardID(key K) uint64 {
	h := hasherPool.Get()
	defer hasherPool.Put(h)

	maphash.WriteComparable(h, key)
	return h.Sum64() % m.num
}

// Store adds a key and value to the map, replacing any existing
// values as needed.
func (m *Map[K, V]) Store(key K, value V) { m.inc().shard(key).store(key, value) }

// Version returns the version for the entire sharded map.
func (m *Map[K, V]) Version() uint64 { return m.clock.Load() }

// Clocks returns a slice of the versions for the map and all of the shards. The first value is the "global" version
func (m *Map[K, V]) Clocks() []uint64 {
	shards := m.shards()
	out := make([]uint64, 1+m.num)
	out[0] = m.clock.Load()
	for idx := range shards {
		out[idx+1] = shards[idx].clock.Load()
	}
	return out
}

// Keys returns an iterator for all the keys in the map. Items are
// provdied from shards sequentially, and in the same sequence, but
// are randomized within the shard. The keys are NOT captured in a
// snapshot, so keys reflecting different logical moments will appear
// in the iterator. No key will appear more than once.
func (m *Map[K, V]) Keys() *fun.Iterator[K] { return fun.FlattenIterators(m.keyItr()) }

// Values returns an iterator for all of the keys in the map. Values
// are provided from shards sequentially, and always in the same
// sequences, but randomized within each shard. The values are NOT
// captured in a snapshot, so values reflecting different logical
// moments will appear in the iterator.
func (m *Map[K, V]) Values() *fun.Iterator[V] { return fun.FlattenIterators(m.valItr()) }

// Iterator provides an iterator over all items in the map. The
// MapItem type captures the version information and information about
// the sharded configuration.
func (m *Map[K, V]) Iterator() *fun.Iterator[MapItem[K, V]] { return m.keyToItem().Process(m.Keys()) }

// ParallelIterator provides an iterator that resolves MapItems in
// parallel, which may be useful in avoiding slow iteration with
// highly contended mutexes. Additionally, because items are processed
// concurrently, items are presented in fully arbitrary order. It's
// possible that the iterator could return some items where
// MapItem.Exists is false if items are deleted during iteration.
func (m *Map[K, V]) ParallelIterator() *fun.Iterator[MapItem[K, V]] {
	return fun.Map(m.Keys(), m.keyToItem(), m.popt())
}

// ParallelValues returns an iterator over all values in the sharded map. Because items are processed
// concurrently, items are presented in fully arbitrary order.
func (m *Map[K, V]) ParallelValues() *fun.Iterator[V] { return fun.MergeIterators(m.valItrp()) }

// ParallelValues provides an iterator over the Values in a sharded map in
// parallel, which may be useful in avoiding slow iteration with
// highly contended mutexes. Additionally, because items are processed
// concurrently, items are presented in fully arbitrary order.
type MapItem[K comparable, V any] struct {
	Exists        bool
	GlobalVersion uint64
	ShardVersion  uint64
	Version       uint64
	ShardID       uint64
	NumShards     uint64
	Key           K
	Value         V
}

// Fetch returns an item from the sharded map, reporting all of the
// relevant version numbers.
func (m *Map[K, V]) Fetch(k K) MapItem[K, V] {
	shards := m.shards()
	it := MapItem[K, V]{Key: k, ShardID: m.shardID(k), NumShards: uint64(len(shards))}

	for {
		it.GlobalVersion = m.clock.Load()
		it.Value, it.Version, it.ShardVersion, it.Exists = shards[it.ShardID].fetch(k)
		if it.GlobalVersion == m.clock.Load() {
			return it
		}

	}
}

// Load retrieves the value from the map. The semantics are the same
// as for maps in go: if the value does not exist it always returns
// the zero value for the type, while the second value indicates if
// the key was present in the map.
func (m *Map[K, V]) Load(key K) (V, bool) { return m.shard(key).load(key) }

// Delete removes a key--and its corresponding value--from the map, if
// it exists.
func (m *Map[K, V]) Delete(key K) { m.inc().shard(key).write().Delete(key) }

// Set adds a key and value to the map from a Pair.
func (m *Map[K, V]) Set(it dt.Pair[K, V]) { m.Store(it.Key, it.Value) }

// Check returns true if the key exists in the map or false otherwise.
func (m *Map[K, V]) Check(key K) bool { return m.shard(key).read().Check(key) }

// Versioned returns the wrapped Versioned object which tracks the
// version (modification count) of the stored object.
func (m *Map[K, V]) Versioned(k K) *Versioned[V] { return ft.IgnoreSecond(m.shard(k).read().Load(k)) }
