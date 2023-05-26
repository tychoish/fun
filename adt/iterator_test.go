package adt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestIterator(t *testing.T) {
	t.Run("Semantics", func(t *testing.T) {
		ctx := testt.Context(t)
		mp := &Map[string, int]{}
		mp.Store("one", 1)
		mp.Store("two", 2)
		mp.Store("three", 3)
		mp.Store("four", 4)

		iter := NewIterator(&sync.Mutex{}, mp.Iterator())

		seen := 0
		for iter.Next(ctx) {
			seen++
			item := iter.Value()
			if item.Value > 4 || item.Value <= 0 {
				t.Error(item)
			}
		}
		uiter := fun.Unwrap(iter)
		if uiter == iter {
			t.Error("unwrap should not be the same")
		}
		if seen != 4 {
			t.Error(seen)
		}
		if iter.Close() != nil {
			t.Error(iter.Close())
		}
	})
	t.Run("ReadOne", func(t *testing.T) {
		ctx := testt.Context(t)
		mp := &Map[string, int]{}
		for i := 0; i < 1000; i++ {
			mp.Store(fmt.Sprint("key=", i), i)
		}
		iter := NewIterator(&sync.Mutex{}, mp.Values()).(syncIterImpl[int])
		wg := &sync.WaitGroup{}
		count := &atomic.Int64{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					it, err := iter.ReadOne(ctx)
					if errors.Is(err, io.EOF) {
						return
					}
					count.Add(1)
					check.NotError(t, err)
					check.True(t, it < 1000)
					check.True(t, it >= 0)
					time.Sleep(time.Millisecond)
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, count.Load(), 1000)
	})
	t.Run("AlternateSyncIterator", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			counter := &atomic.Int64{}

			iter := syncIterImpl[int]{
				mtx: &sync.Mutex{},
				iter: internal.NewGeneratorIterator(func(ctx context.Context) (int, error) {
					if err := ctx.Err(); err != nil {
						return 0, err
					} else if val := counter.Add(1); val > 64 {
						return 0, io.EOF
					} else {
						return int(val), nil
					}
				}),
			}
			for {
				val, err := iter.ReadOne(ctx)
				testt.Log(t, err, val)
				if err != nil {
					assert.Equal(t, val, 0)
					break
				} else {
					assert.True(t, val >= 1 && val < 65)
				}
			}
			assert.True(t, counter.Load() > 2)
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			counter := &atomic.Int64{}

			iter := syncIterImpl[int]{
				mtx: &sync.Mutex{},
				iter: internal.NewGeneratorIterator(func(ctx context.Context) (int, error) {
					if err := ctx.Err(); err != nil {
						return 0, err
					} else if val := counter.Add(1); val > 64 {
						return 0, io.EOF
					} else {
						return int(val), nil
					}
				}),
			}
			for {
				val, err := iter.ReadOne(ctx)
				testt.Log(t, err, val)
				if err != nil {
					assert.Equal(t, val, 0)
					break
				} else {
					assert.True(t, val >= 1 && val < 65)
				}
			}
			assert.True(t, counter.Load() == 0)

		})
	})
}
