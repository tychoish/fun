package shard

import (
	"fmt"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
)

type MapImplementation uint32

const (
	MapImplementationSyncMap MapImplementation = 0
	MapImplementationStdlib  MapImplementation = 1
	MapImplementationRWMutex MapImplementation = 2
	MapImplementationMutex   MapImplementation = 3
)

func (mi MapImplementation) String() string {
	switch mi {
	case MapImplementationSyncMap:
		return "adt.Map[K,V]"
	case MapImplementationStdlib:
		return "map[K]V{}"
	case MapImplementationRWMutex:
		return "struct { mtx sync.RWMutex, mp map[K]V }"
	case MapImplementationMutex:
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

type rwMutexMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data dt.Map[K, V]
}

func (mmp *rwMutexMap[K, V]) mutex() sync.Locker   { return &mmp.mu }
func (mmp *rwMutexMap[K, V]) rwmutex() sync.Locker { return mmp.mu.RLocker() }

func (mmp *rwMutexMap[K, V]) Load(k K) (V, bool) {
	defer adt.With(adt.Lock(mmp.rwmutex()))
	return mmp.data.Load(k)
}

func (mmp *rwMutexMap[K, V]) Store(k K, v V) {
	defer adt.With(adt.Lock(mmp.mutex()))
	mmp.data.Store(k, v)
}
func (mmp *rwMutexMap[K, V]) Delete(k K) {
	defer adt.With(adt.Lock(mmp.mutex()))
	mmp.data.Delete(k)

}
func (mmp *rwMutexMap[K, V]) Check(k K) bool {
	defer adt.With(adt.Lock(mmp.rwmutex()))
	return mmp.data.Check(k)
}
func (mmp *rwMutexMap[K, V]) Keys() *fun.Iterator[K] {
	defer adt.With(adt.Lock(mmp.rwmutex()))
	return mmp.data.Keys()
}
func (mmp *rwMutexMap[K, V]) Values() *fun.Iterator[V] {
	defer adt.With(adt.Lock(mmp.rwmutex()))
	return mmp.data.Values()
}

type mutexMap[K comparable, V any] struct {
	mu   sync.Mutex
	data dt.Map[K, V]
}

func (mmp *mutexMap[K, V]) mutex() sync.Locker { return &mmp.mu }

func (mmp *mutexMap[K, V]) Load(k K) (V, bool) {
	defer adt.With(adt.Lock(mmp.mutex()))
	return mmp.data.Load(k)
}

func (mmp *mutexMap[K, V]) Store(k K, v V) {
	defer adt.With(adt.Lock(mmp.mutex()))
	mmp.data.Store(k, v)
}
func (mmp *mutexMap[K, V]) Delete(k K) {
	defer adt.With(adt.Lock(mmp.mutex()))
	mmp.data.Delete(k)
}

func (mmp *mutexMap[K, V]) Check(k K) bool {
	defer adt.With(adt.Lock(mmp.mutex()))
	return mmp.data.Check(k)
}
func (mmp *mutexMap[K, V]) Keys() *fun.Iterator[K] {
	defer adt.With(adt.Lock(mmp.mutex()))
	return mmp.data.Keys()
}
func (mmp *mutexMap[K, V]) Values() *fun.Iterator[V] {
	defer adt.With(adt.Lock(mmp.mutex()))
	return mmp.data.Values()
}
