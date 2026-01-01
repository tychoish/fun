package shard

import (
	"fmt"
	"hash/maphash"
	"iter"
	"sync/atomic"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt/stw"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

var hashSeed = adt.NewOnce(func() maphash.Seed { return maphash.MakeSeed() })

var hashPool *adt.Pool[*maphash.Hash]

const (
	defaultSize = 32
)

func init() {
	hashPool = &adt.Pool[*maphash.Hash]{}
	hashPool.SetCleanupHook(func(h *maphash.Hash) *maphash.Hash { h.Reset(); return h })
	hashPool.SetConstructor(func() *maphash.Hash { h := &maphash.Hash{}; h.SetSeed(hashSeed.Resolve()); return h })
	hashPool.FinalizeSetup()
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
	imp   MapType
}

// Setup initializes the shard with non-default shard size and backing
// map implementation. Once the Map is initialized (e.g. after calling
// this function, modifying the map, or accessing the contents of the
// map, this function becomes a no-op.)
func (m *Map[K, V]) Setup(n int, mi MapType) { m.sh.Do(func() []sh[K, V] { return m.init(n, mi) }) }

func (m *Map[K, V]) init(n int, mi MapType) []sh[K, V] {
	// ONLY called within the sync.Once scope.

	m.num = ft.Default(uint64(max(0, n)), defaultSize)
	m.imp = ft.Default(mi, MapTypeDefault)
	return m.makeShards()
}

// String reports the type name, number of configured shards, and
// current version of the map.
func (m *Map[K, V]) String() string {
	m.sh.Do(m.defaultShards)

	var (
		k K
		v V
	)

	return fmt.Sprintf("ShardedMap[%T, %T]<%s> Shards(%d) Version(%d)", k, v, m.imp, m.num, m.clock.Load())
}

func (m *Map[K, V]) makeShards() []sh[K, V] {
	// ONLY called within the sync.Once scope.

	shards := make([]sh[K, V], m.num)
	for idx := range shards {
		shards[idx].data = shards[idx].makeVmap(m.imp)
	}
	return shards
}

func (m *Map[K, V]) defaultShards() []sh[K, V]                  { return m.init(defaultSize, MapTypeDefault) }
func (m *Map[K, V]) shards() stw.Slice[sh[K, V]]                { return m.sh.Call(m.defaultShards) }
func (m *Map[K, V]) shard(key K) *sh[K, V]                      { return m.shards().Ptr(int(m.shardID(key))) }
func (m *Map[K, V]) inc() *Map[K, V]                            { m.clock.Add(1); return m }
func to[T, O any](in func(T) O) fn.Converter[T, O]              { return fn.MakeConverter(in) }
func (m *Map[K, V]) s2ks() fn.Converter[*sh[K, V], iter.Seq[K]] { return to(m.shKeys) }
func (m *Map[K, V]) s2vs() fn.Converter[*sh[K, V], iter.Seq[V]] { return to(m.shValues) }
func (m *Map[K, V]) keyToItem() fn.Converter[K, MapItem[K, V]]  { return to(m.Fetch) }
func (*Map[K, V]) shKeys(sh *sh[K, V]) iter.Seq[K]              { return sh.keys() }
func (*Map[K, V]) shValues(sh *sh[K, V]) iter.Seq[V]            { return sh.values() }
func (m *Map[K, V]) shPtrs() stw.Slice[*sh[K, V]]               { return m.shards().Ptrs() }
func (m *Map[K, V]) shIter() iter.Seq[*sh[K, V]]                { return m.shPtrs().Iterator() }
func (m *Map[K, V]) keyItr() iter.Seq[iter.Seq[K]]              { return m.s2ks().Iterator(m.shIter()) }
func (m *Map[K, V]) valItr() iter.Seq[iter.Seq[V]]              { return m.s2vs().Iterator(m.shIter()) }
func (m *Map[K, V]) itemItr() iter.Seq[MapItem[K, V]]           { return m.keyToItem().Iterator(m.Keys()) }

func (m *Map[K, V]) shardID(key K) uint64 {
	h := hashPool.Get()
	defer hashPool.Put(h)

	maphash.WriteComparable(h, key)

	return h.Sum64() % m.num
}

// Store adds a key and value to the map, replacing any existing
// values as needed.
func (m *Map[K, V]) Store(key K, value V) { m.inc().shard(key).store(key, value) }

// Version returns the version for the entire sharded map.
func (m *Map[K, V]) Version() uint64 { return m.clock.Load() }

// Clocks returns a slice of the versions for the map and all of the shards. The first value is the "global" version.
func (m *Map[K, V]) Clocks() []uint64 {
	shards := m.shards()
	out := make([]uint64, 1+m.num)
	out[0] = m.clock.Load()
	for idx := range shards {
		out[idx+1] = shards[idx].clock.Load()
	}
	return out
}

// Keys returns a stream for all the keys in the map. Items are
// provdied from shards sequentially, and in the same sequence, but
// are randomized within the shard. The keys are NOT captured in a
// snapshot, so keys reflecting different logical moments will appear
// in the stream. No key will appear more than once.
func (m *Map[K, V]) Keys() iter.Seq[K] { return irt.Chain(m.KeysSharded()) }

// KeysSharded returns an iterator of iterators, with each iterator provided access to the keys of one of the
// map's underlying shard. Use sharded iterators to fan out a workload.
func (m *Map[K, V]) KeysSharded() iter.Seq[iter.Seq[K]] { return m.keyItr() }

// Values returns a stream for all of the keys in the map. Values
// are provided from shards sequentially, and always in the same
// sequences, but randomized within each shard. The values are NOT
// captured in a snapshot, so values reflecting different logical
// moments will appear in the stream.
func (m *Map[K, V]) Values() iter.Seq[V] { return irt.Chain(m.ValuesSharded()) }

// ValuesSharded returns an iterator of iterators, with each iterator provided access to the values of one of the
// map's underlying shard. Use sharded iterators to fan out a workload.
func (m *Map[K, V]) ValuesSharded() iter.Seq[iter.Seq[V]] { return m.valItr() }

// Items provides a stream over all items in the map. The
// MapItem type captures the version information and information about
// the sharded configuration.
func (m *Map[K, V]) Items() iter.Seq[MapItem[K, V]] { return irt.Keep(m.itemItr(), m.filter) }

// ItemsSharded provides an iterator holding the items of each shard's items. Use the sharded
// iterator to fan out workloads.
func (m *Map[K, V]) ItemsSharded() iter.Seq[iter.Seq[MapItem[K, V]]] {
	return irt.Convert(m.keyItr(), func(seq iter.Seq[K]) iter.Seq[MapItem[K, V]] {
		return irt.Keep(irt.Convert(seq, m.keyToItem()), m.filter)
	})
}

// Iterator provides an iterator over all items in the sharded map.
func (m *Map[K, V]) Iterator() iter.Seq2[K, V]   { return irt.With2(m.Items(), m.split) }
func (*Map[K, V]) split(mi MapItem[K, V]) (K, V) { return mi.Key, mi.Value }
func (*Map[K, V]) filter(mi MapItem[K, V]) bool  { return mi.Exists }

// MapItem wraps the value stored in a sharded map, with synchronized
// sharding and versioning information.
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

// Check returns true if the key exists in the map or false otherwise.
func (m *Map[K, V]) Check(key K) bool { return m.shard(key).read().Check(key) }

// Versioned returns the wrapped Versioned object which tracks the
// version (modification count) of the stored object.
func (m *Map[K, V]) Versioned(k K) *Versioned[V] { return ft.IgnoreSecond(m.shard(k).read().Load(k)) }
