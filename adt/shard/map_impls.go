package shard

import (
	"fmt"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
)

// MapType is the enum that allows users to configure what map
// implementation backs the sharded Map. There are (in theory)
// performance trade offs for different map implementations based on
// the workload and usage patterns.
type MapType uint32

const (
	// MapTypeDefault is an alias for the default map type that's
	// used if the sharded map is not configured with a specific backing type.
	MapTypeDefault MapType = MapTypeSync
	// MapTypeSync is a map based on the sync.Map (via the
	// adt.Map[K,V]) type. This map implementation is optimized
	// for append-heavy/append-only workloads.
	MapTypeSync MapType = 1
	// MapTypeMutex is a standard library map (map[K]V) with all
	// access protected with a sync.Mutex, ensuring exclusive
	// access to all operations. May perform better than the
	// SyncMap for write-heavy workloads with frequent
	// modifications of existing keys.
	MapTypeMutex MapType = 2
	// MapTypeMutex is a standard library map (map[K]V) with all
	// access protected with a sync.RWMutex, ensuring exclusive
	// access for write operations and concurrent access for read
	// operations. Will perform better for read-heavy workloads,
	// with write workloads that are skewed towards modifications
	// rather than additions.
	MapTypeRWMutex MapType = 3
	// MapTypeStdlib is a very minimal wrapper on top of a
	// standard library map (map[K]V). This is not safe for
	// concurrent writes, and is primarily useful for
	// benchmarking.
	MapTypeStdlib MapType = 4
)

func (mi MapType) String() string {
	switch mi {
	case MapTypeSync:
		return "adt.Map[K,V]"
	case MapTypeStdlib:
		return "map[K]V{}"
	case MapTypeRWMutex:
		return "struct { mtx sync.RWMutex, mp map[K]V }"
	case MapTypeMutex:
		return "struct { mtx sync.Mutex, mp map[K]V }"
	default:
		return fmt.Sprintf("invalid<%d>", mi)
	}
}

type oomap[K comparable, V any] interface {
	Load(K) (V, bool)
	Store(K, V)
	Delete(K)
	Check(K) bool
	Keys() *fun.Iterator[K]
	Values() *fun.Iterator[V]
}

type vmap[K comparable, V any] oomap[K, *Versioned[V]]

type rmtxMap[K comparable, V any] struct {
	mu sync.RWMutex
	d  dt.Map[K, V]
}

func withR(m *sync.RWMutex)                 { m.RUnlock() }
func lockR(m *sync.RWMutex) *sync.RWMutex   { m.RLock(); return m }
func withW(m *sync.RWMutex)                 { m.Unlock() }
func lockW(m *sync.RWMutex) *sync.RWMutex   { m.Lock(); return m }
func (m *rmtxMap[K, V]) Store(k K, v V)     { defer withW(lockW(&m.mu)); m.d.Store(k, v) }
func (m *rmtxMap[K, V]) Delete(k K)         { defer withW(lockW(&m.mu)); m.d.Delete(k) }
func (m *rmtxMap[K, V]) Load(k K) (V, bool) { defer withR(lockR(&m.mu)); return m.d.Load(k) }
func (m *rmtxMap[K, V]) Check(k K) bool     { defer withR(lockR(&m.mu)); return m.d.Check(k) }

func (m *rmtxMap[K, V]) Keys() *fun.Iterator[K] {
	defer withR(lockR(&m.mu))

	return m.d.Keys().Producer().WithLock(m.mu.RLocker()).Iterator()
}

func (m *rmtxMap[K, V]) Values() *fun.Iterator[V] {
	defer withR(lockR(&m.mu))

	return m.d.Values().Producer().WithLock(m.mu.RLocker()).Iterator()
}

type mtxMap[K comparable, V any] struct {
	mu sync.Mutex
	d  dt.Map[K, V]
}

func with(m *sync.Mutex)                   { m.Unlock() }
func lock(m *sync.Mutex) *sync.Mutex       { m.Lock(); return m }
func (m *mtxMap[K, V]) Store(k K, v V)     { defer with(lock(&m.mu)); m.d.Store(k, v) }
func (m *mtxMap[K, V]) Delete(k K)         { defer with(lock(&m.mu)); m.d.Delete(k) }
func (m *mtxMap[K, V]) Load(k K) (V, bool) { defer with(lock(&m.mu)); return m.d.Load(k) }
func (m *mtxMap[K, V]) Check(k K) bool     { defer with(lock(&m.mu)); return m.d.Check(k) }

func (m *mtxMap[K, V]) Keys() *fun.Iterator[K] {
	defer with(lock(&m.mu))

	return m.d.Keys().Producer().WithLock(&m.mu).Iterator()
}

func (m *mtxMap[K, V]) Values() *fun.Iterator[V] {
	defer with(lock(&m.mu))

	return m.d.Values().Producer().WithLock(&m.mu).Iterator()
}
