package shard

import (
	"hash/maphash"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

var hashSeed = adt.NewOnce(func() maphash.Seed { return maphash.MakeSeed() })

var hasherPool *adt.Pool[*maphash.Hash]

const defaultSize = 32

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
	sh    adt.Once[[]shmap[K, V]]
	num   uint64
	clock atomic.Uint64
}

func (m *Map[K, V]) setNum(n int)                 { m.num = ft.Default(max(0, uint64(n)), defaultSize) }
func (m *Map[K, V]) init(n int) []shmap[K, V]     { m.setNum(n); return m.makeShards() }
func (m *Map[K, V]) makeShards() []shmap[K, V]    { return make([]shmap[K, V], m.num) }
func (m *Map[K, V]) defaultShards() []shmap[K, V] { return m.init(defaultSize) }

func (m *Map[K, V]) shards() dt.Slice[shmap[K, V]] { return m.sh.Call(m.defaultShards) }
func (m *Map[K, V]) shard(key K) *shmap[K, V]      { return m.shards().Ptr(int(m.shardID(key))) }
func (m *Map[K, V]) inc() *Map[K, V]               { m.clock.Add(1); return m }

func (m *Map[K, V]) shardPtrs() dt.Slice[*shmap[K, V]]      { return m.shards().Ptrs() }
func (m *Map[K, V]) shardIter() *fun.Iterator[*shmap[K, V]] { return m.shardPtrs().Iterator() }

type shmap[K comparable, V any] struct {
	data  adt.Map[K, *Versioned[V]]
	clock atomic.Uint64
}

func (sh *shmap[K, V]) read() *adt.Map[K, *Versioned[V]]  { return &sh.data }
func (sh *shmap[K, V]) write() *adt.Map[K, *Versioned[V]] { sh.clock.Add(1); return &sh.data }

func (sh *shmap[K, V]) Keys() *fun.Iterator[K]      { return sh.read().Keys() }
func (sh *shmap[K, V]) Set(it dt.Pair[K, V])        { sh.Store(it.Key, it.Value) }
func (sh *shmap[K, V]) Load(k K) (V, bool)          { v, ok := sh.read().Load(k); return v.Load(), ok }
func (sh *shmap[K, V]) Version() uint64             { return sh.clock.Load() }
func (sh *shmap[K, V]) Versioned(k K) *Versioned[V] { return ft.IgnoreSecond(sh.read().Load(k)) }

func (sh *shmap[K, V]) Atomic(k K) *adt.Atomic[V] {
	v := sh.Versioned(k)
	return ft.WhenDo(v != nil, v.Atomic)
}

func (sh *shmap[K, V]) Store(k K, v V) {
	mp := sh.write()
	if val, ok := mp.Load(k); ok {
		val.Set(v)
		mp.Store(k, val)
	} else {
		mp.Store(k, Version(v))
	}
}

func (sh *shmap[K, V]) Fetch(k K) (out V, _ uint64, _ bool) {
	mp := sh.read()
	for {
		v := sh.clock.Load()
		d, ok := mp.Load(k)
		switch {
		case !ok:
			return out, 0, false
		case v == sh.clock.Load():
			return d.Load(), v, ok
		}
	}
}

func (m *Map[K, V]) SetupShards(n int)           { m.sh.Do(func() []shmap[K, V] { return m.init(n) }) }
func (m *Map[K, V]) Versioned(k K) *Versioned[V] { return m.shard(k).Versioned(k) }
func (m *Map[K, V]) Atomic(k K) *adt.Atomic[V]   { return m.shard(k).Atomic(k) }

func (m *Map[K, V]) shardID(key K) uint64 {
	h := hasherPool.Get()
	defer hasherPool.Put(h)

	maphash.WriteComparable(h, key)
	return h.Sum64() % m.num
}

// Store adds a key and value to the map, replacing any existing
// values as needed.
func (m *Map[K, V]) Store(key K, value V) { m.inc().shard(key).Store(key, value) }

func (m *Map[K, V]) Version() uint64 { return m.clock.Load() }

type MapItem[K comparable, V any] struct {
	Exists        bool
	GlobalVersion uint64
	ShardVersion  uint64
	ShardID       uint64
	NumShards     uint
	Key           K
	Value         V
}

// Fetch returns an item from the sharded map, reporting all of the
// relevant version numbers.
func (m *Map[K, V]) Fetch(k K) MapItem[K, V] {
	shards := m.shards()
	it := MapItem[K, V]{Key: k, ShardID: m.shardID(k), NumShards: uint(len(shards))}

	for {
		it.GlobalVersion = m.clock.Load()
		it.Value, it.ShardVersion, it.Exists = shards[it.ShardID].Fetch(k)
		if it.GlobalVersion == m.clock.Load() {
			return it
		}

	}
}

func (m *Map[K, V]) shardToKeyIter() fun.Transform[*shmap[K, V], *fun.Iterator[K]] {
	return fun.Converter(func(i *shmap[K, V]) *fun.Iterator[K] { return i.Keys() })
}
func (m *Map[K, V]) keyToItem() fun.Transform[K, MapItem[K, V]] {
	return fun.Converter(func(k K) MapItem[K, V] { return m.Fetch(k) })
}
func (m *Map[K, V]) popts() fun.OptionProvider[*fun.WorkerGroupConf] {
	return fun.WorkerGroupConfNumWorkers(int(m.num))
}

func (m *Map[K, V]) Keys() *fun.Iterator[K] {
	return fun.FlattenIterators(m.shardToKeyIter().Process(m.shardIter()))
}
func (m *Map[K, V]) ParallelKeys() *fun.Iterator[K] {
	return fun.FlattenIterators(m.shardToKeyIter().ProcessParallel(m.shardIter(), m.popts()))
}
func (m *Map[K, V]) ParallelIterator() *fun.Iterator[MapItem[K, V]] {
	return m.keyToItem().ProcessParallel(fun.FlattenIterators(m.shardToKeyIter().ProcessParallel(m.shardIter(), m.popts())))
}
func (m *Map[K, V]) Iterator() *fun.Iterator[MapItem[K, V]] {
	return m.keyToItem().Process(m.Keys())
}

// Load retrieves the value from the map. The semantics are the same
// as for maps in go: if the value does not exist it always returns
// the zero value for the type, while the second value indicates if
// the key was present in the map.
func (m *Map[K, V]) Load(key K) (V, bool) { return m.shard(key).Load(key) }

// Delete removes a key--and its corresponding value--from the map, if
// it exists.
func (m *Map[K, V]) Delete(key K) { m.inc().shard(key).write().Delete(key) }

// Set adds a key and value to the map from a Pair.
func (m *Map[K, V]) Set(it dt.Pair[K, V]) { m.Store(it.Key, it.Value) }

// Check returns true if the key exists in the map or false otherwise.
func (m *Map[K, V]) Check(key K) bool { return m.shard(key).read().Check(key) }

type Versioned[T any] struct {
	value adt.Atomic[T]
	clock atomic.Uint64
}

func Version[T any](in T) *Versioned[T]              { o := &Versioned[T]{}; o.Set(in); return o }
func (vv *Versioned[T]) inc()                        { vv.clock.Add(1) }
func (vv *Versioned[T]) innerAtomic() *adt.Atomic[T] { return ft.Ptr(vv.value) }
func (vv *Versioned[T]) innerLoad() T                { return vv.value.Load() }
func (vv *Versioned[T]) innerVersion() uint64        { return vv.clock.Load() }

func (vv *Versioned[T]) Atomic() *adt.Atomic[T] { return ft.WhenDo(vv != nil, vv.innerAtomic) }
func (vv *Versioned[T]) Version() uint64        { return ft.WhenDo(vv != nil, vv.innerVersion) }
func (vv *Versioned[T]) Load() T                { return ft.WhenDo(vv != nil, vv.innerLoad) }
func (vv *Versioned[T]) Set(newValue T)         { vv.inc(); vv.value.Store(newValue) }
func (vv *Versioned[T]) Fetch() (T, uint64) {
	for {
		version := vv.clock.Load()
		val := vv.value.Load()
		if version == vv.clock.Load() {
			return val, version
		}
	}
}
