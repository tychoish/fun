package internal

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/testt"
)

func makeClosedSlice[T any](in []T) <-chan T {
	out := make(chan T, len(in))
	for i := range in {
		out <- in[i]
	}
	close(out)
	return out
}

func TestIterators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Slice", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			iter := NewSliceIter([]int{1, 2, 3, 4})
			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 4 || iter.Value() <= 0 {
					t.Error(iter.Value())
				}
			}
			if seen != 4 {
				t.Error(seen)
			}
			if iter.Close() != nil {
				t.Error(iter.Close())
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			iter := &SliceIterImpl[int]{}
			seen := 0
			for iter.Next(ctx) {
				seen++
			}
			if seen != 0 {
				t.Error(seen)
			}
			if iter.Close() != nil {
				t.Error(iter.Close())
			}
		})
	})
	t.Run("Channel", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			var closerCalled bool
			iter := &ChannelIterImpl[int]{
				Pipe:   makeClosedSlice([]int{1, 2, 3, 4}),
				Closer: func() { closerCalled = true },
			}
			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 4 || iter.Value() <= 0 {
					t.Error(iter.Value())
				}
			}
			if seen != 4 {
				t.Error(seen)
			}
			if iter.Close() != nil {
				t.Error(iter.Close())
			}
			if !closerCalled {
				t.Error("closer not called")
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			t.Run("Before", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				iter := &ChannelIterImpl[int]{}
				seen := 0
				for iter.Next(ctx) {
					seen++
				}
				if seen != 0 {
					t.Error(seen)
				}
				if iter.Close() != nil {
					t.Error(iter.Close())
				}
			})
			t.Run("During", func(t *testing.T) {
				iter := NewChannelIterator(make(chan int))
				seen := 0
				sig := make(chan struct{})
				go func() { defer close(sig); time.Sleep(5 * time.Millisecond); cancel() }()
				for iter.Next(ctx) {
					seen++
				}
				if seen != 0 {
					t.Error(seen)
				}
			})
		})
	})
	t.Run("Map", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var closerCalled bool
			iter := &ChannelIterImpl[int]{
				Pipe:   makeClosedSlice([]int{1, 2, 3, 4}),
				Closer: func() { closerCalled = true },
			}
			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 4 || iter.Value() <= 0 {
					t.Error(iter.Value())
				}
			}
			if seen != 4 {
				t.Error(seen)
			}
			if iter.Close() != nil {
				t.Error(iter.Close())
			}
			if !closerCalled {
				t.Error("closer not called")
			}
		})
		t.Run("Waiting", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := NewChannelIterator(makeClosedSlice([]int{1, 2, 3, 4}))

			seen := 0
			for iter.Next(ctx) {
				seen++
			}
			if seen != 4 {
				t.Error("unexpected", seen)
			}
			start := time.Now()
			iter.WG.Add(1)
			go func() {
				defer iter.WG.Done()
				time.Sleep(10 * time.Millisecond)
			}()
			if err := iter.Close(); err != nil {
				t.Error(err)
			}
			if time.Since(start) < 10*time.Millisecond {
				t.Error(time.Since(start))
			}
		})
	})
	t.Run("AlternateSyncIterator", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			counter := &atomic.Int64{}

			iter := SyncIterImpl[int]{
				Mtx: &sync.Mutex{},
				Iter: NewGeneratorIterator(func(ctx context.Context) (int, error) {
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

			iter := SyncIterImpl[int]{
				Mtx: &sync.Mutex{},
				Iter: NewGeneratorIterator(func(ctx context.Context) (int, error) {
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

	t.Run("ReadOne", func(t *testing.T) {
		t.Parallel()
		ch := make(chan string, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		ch <- "merlin"
		defer cancel()
		out, err := ReadOne(ctx, ch)
		if err != nil {
			t.Fatal(err)
		}
		if out != "merlin" {
			t.Fatal(out)
		}

		ch <- "merlin"
		cancel()
		seenCondition := false
		for i := 0; i < 10; i++ {
			t.Log(i)
			_, err = ReadOne(ctx, ch)
			if errors.Is(err, context.Canceled) {
				seenCondition = true
			}
			t.Log(err)

			select {
			case ch <- "merlin":
			default:
			}
		}
		if !seenCondition {
			t.Error("should have observed a context canceled")

		}
	})

	t.Run("NonBlockingReadOne", func(t *testing.T) {
		t.Parallel()
		t.Run("BlockingCompatibility", func(t *testing.T) {
			ch := make(chan string, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			ch <- "merlin"
			defer cancel()
			out, err := NonBlockingReadOne(ctx, ch)
			if err != nil {
				t.Fatal(err)
			}
			if out != "merlin" {
				t.Fatal(out)
			}

			ch <- "merlin"
			cancel()
			seenCondition := false
			for i := 0; i < 10; i++ {
				t.Log(i)
				_, err = NonBlockingReadOne(ctx, ch)
				if errors.Is(err, context.Canceled) {
					seenCondition = true
				}
				t.Log(err)

				select {
				case ch <- "merlin":
				default:
				}
			}
			if !seenCondition {
				t.Error("should have observed a context canceled")

			}
		})
		t.Run("NonBlocking", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan string)
			out, err := NonBlockingReadOne(ctx, ch)
			assert.Zero(t, out)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)
		})
		t.Run("Closed", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan string)
			close(ch)
			out, err := NonBlockingReadOne(ctx, ch)
			assert.Zero(t, out)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
		})
	})
	t.Run("Generator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("BasicOperation", func(t *testing.T) {
			var closed bool
			iter := NewGeneratorIterator(func(context.Context) (int, error) {
				return 1, nil
			})
			iter.Closer = func() { closed = true }

			if iter.Value() != 0 {
				t.Error("should initialize to zero")
			}
			if !iter.Next(ctx) {
				t.Error("should always iterate at least once")
			}
			if err := iter.Close(); err != nil {
				t.Error(err)
			}
			if !closed {
				t.Error("should have observed closed side effect")
			}
			if iter.Next(ctx) {
				t.Error("should not iterate after close")
			}
		})
		t.Run("RespectEOF", func(t *testing.T) {
			count := 0
			iter := NewGeneratorIterator(func(context.Context) (int, error) {
				count++
				if count > 10 {
					return 1000, io.EOF
				}
				return count, nil
			})

			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 10 {
					t.Error("unexpected value", iter.Value())
				}
			}

			if seen > count {
				t.Error(seen, "vs", "count")
			}
		})
		t.Run("PropogateErrors", func(t *testing.T) {
			count := 0
			expected := errors.New("kip")
			iter := SyncIterImpl[int]{
				Mtx: &sync.Mutex{},
				Iter: &GeneratorIterator[int]{
					Operation: func(context.Context) (int, error) {
						count++
						if count > 10 {
							return 1000, expected
						}
						return count, nil
					},
				},
			}
			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 10 {
					t.Error("unexpected value", iter.Value())
				}
			}

			if seen > count {
				t.Error(seen, "vs", "count")
			}
			if err := iter.Close(); !errors.Is(err, expected) {
				t.Error(err)

			}
		})

	})
}
