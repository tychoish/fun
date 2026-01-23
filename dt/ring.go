package dt

import (
	"bytes"
	"iter"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

// the maximum size for the ring: given that we divide to find the
// index in the ring, if this is the maximum size of the ring, it
// means we can't.
const maxRingSize int64 = 4294967296

const defaultRingSize int = 1024

// Ring is a simple generic ring-buffer implemented on top of a slice/array
// with a few conveniences: there are forward and backward iterators;
// you can pop items from the "end" (oldest) in the buffer. The
// Total() method maintains a count of the total number of items added
// to the buffer.
//
// Operations on the Ring are NOT safe for concurrent use from
// multiple go routines.
type Ring[T any] struct {
	size  int
	count int
	pos   int
	total uint64
	buf   struct {
		ring []T
		nils []*T
	}
}

// Setup sets the size of the ring buffer and initializes the buffer,
// if the buffer hasn't been used. Using the buffer initializes it with a size of 1024.
func (r *Ring[T]) Setup(size int) {
	if r.size == 0 {
		r.size = size
	}
	r.init()
}

func (r *Ring[T]) init() {
	if r.buf.ring == nil {
		r.innerInit()
		return
	}
}

func (r *Ring[T]) innerInit() {
	r.size = stw.Default(r.size, defaultRingSize)

	erc.Invariant(ers.Whenf(int64(r.size) > maxRingSize, "invalid size (%d) max: %d", r.size, maxRingSize))
	erc.Invariant(ers.Whenf(r.size < 2, "invalid size %d (must be > 1)", r.size))

	r.buf.ring = make([]T, r.size)
	r.buf.nils = make([]*T, r.size)
}

func (*Ring[T]) zero() (out T)            { return out }
func (r *Ring[T]) hasWrapped() bool       { return int(r.total) > r.size }
func (r *Ring[T]) offset(idx, by int) int { return (r.size * max(1, stw.Abs(idx))) + (idx + by) }
func (r *Ring[T]) after(idx int) int      { return r.offset(idx, 1) % r.size }
func (r *Ring[T]) before(idx int) int     { return r.offset(idx, -1) % r.size }

func (r *Ring[T]) oldest() int {
	if r.hasWrapped() {
		return r.pos
	}
	return 0
}

// Cap returns the capacity of the ring buffer. This is either the size passed to Setup() or the default 1024.
func (r *Ring[T]) Cap() int { return r.size }

// Len returns the number of elements in the buffer. This is, some
// number between the capacity and 0. It is decremented when items are
// popped from the buffer, but only incremented when a previously empty
// position is filled.
func (r *Ring[T]) Len() int { return r.count }

// Total returns the number of elements that have ever been added to
// the buffer. This number is never decremented.
func (r *Ring[T]) Total() uint64 { return r.total }

// Head returns the oldest element in the buffer.
func (r *Ring[T]) Head() T { r.init(); return r.buf.ring[r.oldest()] }

// Tail returns the newest (or most recently added) element in the buffer.
func (r *Ring[T]) Tail() T { r.init(); return r.buf.ring[r.before(r.pos)] }

// Push adds an element to the buffer in the next position,
// potentially overwriting the oldest element in the buffer once the
// buffer is full.
//
// Elements are always pushed to the "next" position in the buffer,
// even if elements are removed using Pop().
func (r *Ring[T]) Push(val T) {
	r.init()

	r.total++
	if r.count < r.size {
		r.count++
	}

	r.buf.ring[r.pos] = val
	r.buf.nils[r.pos] = &r.buf.ring[r.pos]

	r.pos = r.after(r.pos)
}

// Pop returns the oldest element in the buffer to the caller. If the
// buffer is empty, then Pop() returns nil.
//
// The returned value is (effectively) owned by the caller of Pop() and
// is independent of the value stored in the ring.
func (r *Ring[T]) Pop() *T {
	r.init()

	for idx := r.oldest(); r.count > 0; idx = r.after(idx) {
		if r.buf.nils[idx] == nil {
			continue
		}

		item := r.buf.ring[idx]
		r.buf.ring[idx] = r.zero()
		r.buf.nils[idx] = nil
		r.count--
		return &item
	}

	return nil
}

// FIFO returns an iterator that begins at the first (oldest; Head) element
// and iterates forward to the current or most recently added element
// in the buffer.
func (r *Ring[T]) FIFO() iter.Seq[T] { r.init(); return r.iterate(r.oldest(), r.after) }

// LIFO returns the element that was most recently added to buffer and
// iterates backwords to the oldest element in the buffer.
func (r *Ring[T]) LIFO() iter.Seq[T] { r.init(); return r.iterate(r.before(r.pos), r.before) }

// PopFIFO returns a FIFO iterator that consumes elements in the
// buffer, starting with the oldest element in the buffer and moving
// through all elements. The iterator is exhusted when the buffer is empty.
func (r *Ring[T]) PopFIFO() iter.Seq[T] { return irt.UntilNil(irt.Generate(r.Pop)) }

// MarshalJSON encodes the ring as a JSON array in FIFO order.
func (r *Ring[T]) MarshalJSON() ([]byte, error) { return irt.MarshalJSON(r.FIFO()) }

// UnmarshalJSON decodes a JSON array and pushes each element into the ring.
func (r *Ring[T]) UnmarshalJSON(in []byte) error {
	for kv, err := range irt.UnmarshalJSON[T](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		r.Push(kv)
	}
	return nil
}

func (r *Ring[T]) iterate(from int, advance func(int) int) iter.Seq[T] {
	var current int
	var count int
	next := from

	return func(yield func(T) bool) {
		for r.count > 0 && (count == 0 || next != from) {
			current = next
			next = advance(current)

			count++

			if r.buf.nils[current] != nil && !yield(r.buf.ring[current]) {
				return
			}
		}
	}
}
