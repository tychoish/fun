package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
)

func TestDistributor(t *testing.T) {
	// TODO: add specific unit tests, most of the distributor code
	// is tested via the broker.

	t.Run("Channel", func(t *testing.T) {
		t.Run("CloseSafety", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan string, 100)
			buf := DistributorChannel(ch)
			if err := buf.Push(ctx, "merlin"); err != nil {
				t.Error(err)
			}
			_, err := buf.Pop(ctx)
			if err != nil {
				t.Fatal(err)
			}
			close(ch)
			_, err = buf.Pop(ctx)
			if err == nil {
				t.Fatal("expected error")
			}
			errs := erc.Unwind(err)
			if len(errs) != 1 {
				t.Error(len(errs))
			}

			// repeat to make sure we don't accumulate
			_, err = buf.Pop(ctx)
			if err == nil {
				t.Fatal("expected error")
			}
			errs = erc.Unwind(err)
			if len(errs) != 1 {
				t.Error(len(errs))
			}

			// add another so that the rest of the test works
			err = buf.Push(ctx, "merlin")
			if err != nil {
				t.Error(err)
			}
			err = buf.Push(ctx, "kip")
			if err == nil {
				t.Error("expected error")
			}

			errs = erc.Unwind(err)
			if len(errs) != 1 {
				t.Error(len(errs))
			}
			err = buf.Push(ctx, "kip")
			if err == nil {
				t.Error("expected error")
			}
			errs = erc.Unwind(err)
			if len(errs) != 1 {
				t.Error(len(errs))
			}
		})
		t.Run("Cancelation", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			start := time.Now()

			ch := make(chan string, 1)
			ch <- "merlin"
			buf := DistributorChannel(ch)
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				err := buf.Push(ctx, "kip")
				if err == nil {
					t.Error("expected error")
				}
				errs := erc.Unwind(err)
				if len(errs) != 1 {
					t.Error(len(errs))
				}
			}()
			<-sig
			if time.Since(start) < 10*time.Millisecond {
				t.Error(time.Since(start))
			}
		})
	})
	t.Run("Iterator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queue := NewUnlimitedQueue[string]()
		_ = queue.Add("string")
		_ = queue.Add("strong")
		_ = queue.Add("strung")

		dist := DistributorQueue(queue)

		set := set.MakeNewOrdered[string]()
		seen := 0
		if queue.tracker.len() != 3 {
			t.Fatal(queue.tracker.len())
		}

		iter := DistributorIterator(dist)
		go func() {
			time.Sleep(10 * time.Millisecond)
			queue.Close()
		}()
		fun.Observe(ctx, iter, func(in string) { set.Add(in); seen++ })
		if iter.Next(ctx) {
			t.Error("iterator should be empty")
		}
		if set.Len() != 3 {
			t.Log(itertool.CollectSlice(ctx, (set.Iterator())))
			t.Error(set.Len())
		}
		if seen != 3 {
			t.Error(seen)
		}
		if queue.tracker.len() != 0 {
			t.Fatal(queue.tracker.len())
		}
		if iter.Value() != "" {
			t.Error(iter.Value())
		}
	})
}
