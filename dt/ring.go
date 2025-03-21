package dt

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

const maxRingSize int = 4294967296
const defaultRingSize int = 1024

type Ring[T any] struct {
	size  int
	count int
	pos   int
	total uint64
	buf   struct {
		once sync.Once
		ring []T
		nils []*T // this could be a bitmap but that sounds fussy.
	}
}

func (r *Ring[T]) innerInit() {
	fun.Invariant.IsTrue(r.size < maxRingSize, "invalid size", r.size)

	r.size = ft.Default(max(2, r.size), 1024)
	r.buf.ring = make([]T, r.size)
	r.buf.nils = make([]*T, r.size)
}

func (r *Ring[T]) init()          { r.buf.once.Do(r.innerInit) }
func (r *Ring[T]) set(i int, v T) { r.init(); r.buf.ring[i] = v; r.buf.nils[i] = &r.buf.ring[i] }
func (r *Ring[T]) get(idx int) T  { r.init(); return r.buf.ring[idx] }

func (r *Ring[T]) Cap() int               { return r.size }
func (r *Ring[T]) Setup(size int)         { r.size = size }
func (r *Ring[T]) Len() int               { return r.count }
func (r *Ring[T]) Total() uint64          { return r.total }
func (r *Ring[T]) Tail() T                { return r.get(r.curentPos()) }
func (r *Ring[T]) Head() T                { return r.get(r.oldestPos()) }
func (r *Ring[T]) FIFO() *fun.Iterator[T] { return r.iterate(r.oldestPos(), r.after) }
func (r *Ring[T]) LIFO() *fun.Iterator[T] { return r.iterate(r.curentPos(), r.before) }

func (r *Ring[T]) Push(val T) {
	r.init()

	r.set(r.pos, val)

	r.pos = r.nextPos()

	r.total++
	if r.count < r.size {
		r.count++
	}
}

func (r *Ring[T]) nextPos() int       { return r.after(r.pos) }
func (r *Ring[T]) curentPos() int     { return r.pos }
func (r *Ring[T]) oldestPos() int     { return ft.IfDo(r.count < r.size, ft.Wrapper(0), r.nextPos) }
func (r *Ring[T]) after(idx int) int  { return (idx + 1) % r.size }
func (r *Ring[T]) before(idx int) int { return ((r.size * intish.Abs(idx)) + (idx - 1)) % r.size }

func (r *Ring[T]) iterate(from int, advance func(int) int) *fun.Iterator[T] {
	// var prev int
	var next int
	var current int

	var zero T
	next = from
	var count int

	return fun.NewProducer(func(ctx context.Context) (T, error) {
		if count == len(r.buf.ring) {
			return zero, io.EOF
		}
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		// advance the index.
		for {
			// prev = current
			current = next
			next = advance(current)

			if r.buf.nils[current] != nil {
				break
			}
			// current is nil and also if...
			if next == from && r.buf.nils[next] == nil {
				return zero, io.EOF
			}
			// otherwise skip to the next (this supports us popping)
		}
		fmt.Println(count, r.buf.ring[current], r.buf.ring[next], r.buf.ring)
		count++
		return r.buf.ring[current], nil
	}).Iterator()
}
