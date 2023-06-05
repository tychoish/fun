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
	t.Run("ReadOneEOF", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		ch := make(chan string, 1)
		close(ch)

		_, err := ReadOne(ctx, ch)
		if !errors.Is(err, io.EOF) {
			t.Fatal(err)
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
}
