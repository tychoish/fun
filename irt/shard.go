package irt

import (
	"context"
	"iter"
	"sync"
)

func ShardByHash[T any](ctx context.Context, num int64, seq iter.Seq[T], getHash func(T) int64) iter.Seq[iter.Seq[T]] {
	shards := makeShards(getHash)
	shards.init(num)

	populate := once(func() {
		go func() {
			defer shards.closeAll()

		NEXT:
			for value := range seq {
				for ch := range shards.getShardsFrom(getHash(value)) {
					select {
					case <-ctx.Done():
						return
					case ch <- value:
						continue NEXT
					}
				}
			}
			return
		}()
	})

	return Merge(Index(shards.iterator()), func(shardID int, ch chan T) iter.Seq[T] {
		populate()
		return func(yield func(T) bool) {
			defer shards.clean()
			defer shards.closeIdx(shardID)

			for recvAndYield(ctx, ch, yield) {
				continue
			}
		}
	})
}

type shardSet[T any] struct {
	mutex sync.Mutex
	chans []chan T
	count int64
	hashf func(T) int64
}

func makeShards[T any](hashf func(T) int64) *shardSet[T] { return &shardSet[T]{hashf: hashf} }
func (sh *shardSet[T]) lock() *sync.Mutex                { return lock(sh.mtx()) }
func (sh *shardSet[T]) mtx() *sync.Mutex                 { return &sh.mutex }

func (sh *shardSet[T]) init(size int64) {
	defer with(sh.lock())
	sh.count = size
	sh.chans = Collect(WhileOk(Perpetual2(ntimes(int(size), sh.makeCh))))
}

func (*shardSet[T]) makeCh() chan T                  { return make(chan T) }
func (*shardSet[T]) closeCh(in chan T)               { close(in) }
func (sh *shardSet[T]) closeIdx(idx int)             { defer with(sh.lock()); close(sh.chans[idx]) }
func (sh *shardSet[T]) withLock(op func() bool) bool { defer with(sh.lock()); return op() }
func (sh *shardSet[T]) clean()                       { defer with(sh.lock()); sh.chans, sh.count = sh.compact(), sh.len() }
func (sh *shardSet[T]) closeAll()                    { defer with(sh.lock()); Apply(sh.iterator(), sh.closeCh) }
func (sh *shardSet[T]) iterator() iter.Seq[chan T]   { return Remove(Slice(sh.chans), isNilChan) }
func (sh *shardSet[T]) compact() []chan T            { return Collect(sh.iterator(), 0, int(sh.count)) }
func (sh *shardSet[T]) len() int64                   { return int64(len(sh.chans)) }
func (sh *shardSet[T]) atIndex(index int64) chan T   { return sh.chans[index] }

func (sh *shardSet[T]) getShardsFrom(index int64) iter.Seq[chan T] {
	return func(yield func(chan T) bool) {
		for idx, ct := index%sh.count, int64(0); ct < sh.count; idx, ct = idx+1%sh.count, ct+1 {
			if sh.withLock(func() bool {
				if sh := sh.atIndex(idx); sh == nil {
					return false
				} else {
					yield(sh)
				}
				return true
			}) {
				return
			}
		}
	}
}
