package adt

import (
	"bytes"
	"iter"
	"sync"

	"github.com/tychoish/fun/irt"
)

// OrderedMap provides a map implementation that maintains insertion order.
// It has the same interface as stw.Map but iterates in insertion order.
// This implementation uses a mutex to ensure consistency between the
// underlying hash map and the insertion order list.
type OrderedMap[K comparable, V any] struct {
	mtx  sync.Mutex
	once sync.Once
	hash Map[K, *elem[irt.KV[K, V]]]
	list list[irt.KV[K, V]]
}

func (m *OrderedMap[K, V]) init()   { m.once.Do(m.doInit) }
func (m *OrderedMap[K, V]) doInit() { m.hash = Map[K, *elem[irt.KV[K, V]]]{} }

func (*OrderedMap[K, V]) zerov() (out V)       { return }
func (*OrderedMap[K, V]) with(mtx *sync.Mutex) { mtx.Unlock() }
func (m *OrderedMap[K, V]) lock() *sync.Mutex  { m.mtx.Lock(); m.init(); return &m.mtx }

// Len returns the length of the map.
func (m *OrderedMap[K, V]) Len() int { defer m.with(m.lock()); return m.hash.Len() }

// Check returns true if the value K is in the map.
func (m *OrderedMap[K, V]) Check(key K) bool { defer m.with(m.lock()); return m.hash.Check(key) }

// Get returns the value from the map. If the key is not present in the map,
// this returns the zero value for V.
func (m *OrderedMap[K, V]) Get(key K) (out V) { out, _ = m.Load(key); return }

// Load returns the value in the map for the key, and an "ok" value
// which is true if that item is present in the map.
func (m *OrderedMap[K, V]) Load(key K) (V, bool) {
	defer m.with(m.lock())
	elem, ok := m.hash.Load(key)
	if ok {
		return elem.Value().Value, true
	}
	return m.zerov(), false
}

// Delete removes a key from the map.
func (m *OrderedMap[K, V]) Delete(k K) {
	defer m.with(m.lock())
	entry, ok := m.hash.Load(k)
	if !ok {
		return
	}

	defer m.hash.Delete(k)

	if entry != nil {
		entry.Pop()
	}
}

// Set adds a key-value pair directly to the map. If the key already
// exists, it updates the value but does not change the insertion
// order. When the return value is true, the key existed in the map
// before the operation.
func (m *OrderedMap[K, V]) Set(key K, value V) bool {
	defer m.with(m.lock())
	return m.innerSet(key, value)
}

func (m *OrderedMap[K, V]) innerSet(key K, value V) bool {
	elem, ok := m.hash.Load(key)
	if !ok {
		elem = m.list.newElem().Set(irt.MakeKV(key, value))
		m.list.Back().PushBack(elem)
		m.hash.Set(key, elem)
		return ok
	}

	elem.Set(irt.MakeKV(key, value))
	return ok
}

// Ensure sets the provided key in the map to the zero value
// for the value type. If the key already exists, it is not modified.
func (m *OrderedMap[K, V]) Ensure(key K) bool {
	defer m.with(m.lock())
	if m.hash.Check(key) {
		return true
	}

	return m.innerSet(key, m.zerov())
}

// Store adds a key-value pair directly to the map. Alias for Add.
func (m *OrderedMap[K, V]) Store(k K, v V) { m.Set(k, v) }

// Extend adds a sequence of key-value pairs to the map.
func (m *OrderedMap[K, V]) Extend(seq iter.Seq2[K, V]) { irt.Apply2(seq, m.Store) }

// Iterator returns a standard Go iterator interface to the key-value
// pairs of the map in insertion order.
func (m *OrderedMap[K, V]) Iterator() iter.Seq2[K, V] {
	return irt.KVsplit(
		irt.WithMutex(
			irt.Keep(
				m.list.IteratorFront(),
				m.checkForElem,
			),
			&m.mtx,
		),
	)
}

// Keys provides an iterator over just the keys in the map in insertion order.
func (m *OrderedMap[K, V]) Keys() iter.Seq[K] { m.init(); return irt.First(irt.KVsplit(m.elems())) }

func (m *OrderedMap[K, V]) checkForElem(e irt.KV[K, V]) bool { return m.hash.Check(e.Key) }
func (m *OrderedMap[K, V]) elems() iter.Seq[irt.KV[K, V]] {
	return irt.WithMutex(m.list.IteratorFront(), &m.mtx)
}

// Values provides an iterator over just the values in the map in insertion order.
func (m *OrderedMap[K, V]) Values() iter.Seq[V] { return irt.Second(m.Iterator()) }

func (m *OrderedMap[K, V]) MarshalJSON() ([]byte, error) { return irt.MarshalJSON2(m.Iterator()) }
func (m *OrderedMap[K, V]) UnmarshalJSON(in []byte) error {
	for kv, err := range irt.UnmarshalJSON2[K, V](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		m.Store(kv.Key, kv.Value)
	}
	return nil
}
