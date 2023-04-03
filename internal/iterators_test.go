package internal

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
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
			if closerCalled {
				t.Error("closer called early")
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
				iter := &ChannelIterImpl[int]{Pipe: make(chan int)}
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
			iter := &MapIterImpl[int]{
				ChannelIterImpl: ChannelIterImpl[int]{
					Pipe:   makeClosedSlice([]int{1, 2, 3, 4}),
					Closer: func() { closerCalled = true },
				},
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
			if closerCalled {
				t.Error("closer called early")
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

			iter := &MapIterImpl[int]{
				ChannelIterImpl: ChannelIterImpl[int]{
					Pipe: makeClosedSlice([]int{1, 2, 3, 4}),
				},
			}
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
	t.Run("SendOne", func(t *testing.T) {
		t.Run("BlockingHelper", func(t *testing.T) {
			if Blocking(true) != SendModeBlocking {
				t.Error("blocking true is not blocking")
			}
			if Blocking(false) != SendModeNonBlocking {
				t.Error("blocking false is blocking")
			}
		})
		t.Run("Send", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int, 2)
			err := SendOne(ctx, Blocking(true), ch, 1)
			assert.NotError(t, err)
			assert.Equal(t, <-ch, 1)

			err = SendOne(ctx, Blocking(false), ch, 3)
			assert.NotError(t, err)
			assert.Equal(t, <-ch, 3)
		})
		t.Run("NonBlocking", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int)

			err := SendOne(ctx, Blocking(false), ch, 3)
			assert.NotError(t, err)
			close(ch)
			out, ok := <-ch
			assert.True(t, !ok)
			assert.Zero(t, out)
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ch := make(chan int)

			err := SendOne(ctx, Blocking(true), ch, 1)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			err = SendOne(ctx, Blocking(false), ch, 1)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			err = SendOne(ctx, SendMode(42), ch, 1)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
		})

	})

}
