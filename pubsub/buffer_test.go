package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/erc"
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
}
