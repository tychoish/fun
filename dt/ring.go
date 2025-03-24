package dt

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

const maxRingSize int = 4294967296
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

func (r *Ring[T]) init() { ft.WhenCall(r.buf.ring == nil, r.innerInit) }
func (r *Ring[T]) innerInit() {
	r.size = ft.Default(r.size, defaultRingSize)

	fun.Invariant.IsTrue(r.size < maxRingSize, "invalid size", r.size, "max:", maxRingSize)
	fun.Invariant.IsTrue(r.size >= 2, "invalid size", r.size, "(must be > 1)")

	r.buf.ring = make([]T, r.size)
	r.buf.nils = make([]*T, r.size)
}

func (r *Ring[T]) Setup(size int) { r.size = size }
func (r *Ring[T]) Cap() int       { return r.size }
func (r *Ring[T]) Len() int       { return r.count }
func (r *Ring[T]) Total() uint64  { return r.total }

func (r *Ring[T]) Head() T                { r.init(); return r.buf.ring[r.oldest()] }
func (r *Ring[T]) Tail() T                { r.init(); return r.buf.ring[r.before(r.pos)] }
func (r *Ring[T]) FIFO() *fun.Iterator[T] { r.init(); return r.iterate(r.oldest(), r.after) }
func (r *Ring[T]) LIFO() *fun.Iterator[T] { r.init(); return r.iterate(r.before(r.pos), r.before) }

func (*Ring[T]) zero() (out T)            { return out }
func (r *Ring[T]) offset(idx, by int) int { return intish.AbsMax(r.size, r.size*idx) + (idx + by) }
func (r *Ring[T]) oldest() int            { return ft.IfValue(r.count < r.size, 0, r.pos) }
func (r *Ring[T]) after(idx int) int      { return r.offset(idx, 1) % r.size }
func (r *Ring[T]) before(idx int) int     { return r.offset(idx, -1) % r.size }

func (r *Ring[T]) Push(val T) {
	r.init()

	r.buf.ring[r.pos] = val
	r.buf.nils[r.pos] = &r.buf.ring[r.pos]

	r.pos = r.after(r.pos)

	r.total++
	if r.count < r.size {
		r.count++
	}
}

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

func (r *Ring[T]) iterate(from int, advance func(int) int) *fun.Iterator[T] {
	var count int
	var current int

	next := from

	return fun.CheckProducer(func() (T, bool) {
		for {
			if count >= len(r.buf.ring) || (next == from && count > 0) {
				return r.zero(), false
			}

			current = next
			next = advance(current)

			count++

			if r.buf.nils[current] != nil {
				return r.buf.ring[current], true
			}
		}
	}).Iterator()
}
