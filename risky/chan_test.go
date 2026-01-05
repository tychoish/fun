package risky_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/risky"
)

// this is public in the ft package which we're not importing here to avoid a cycle; using it in
// this test with the test case from ft to exercise the other modules.
func ContextErrorChannel(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		defer risky.Send(out, ctx.Err)
		risky.Wait(ctx.Done())
	}()

	return out
}

func TestChan(t *testing.T) {
	t.Run("ContextError", func(t *testing.T) {
		t.Run("TimeValidate", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			started := time.Now()
			err := <-ContextErrorChannel(ctx)
			if err == nil {
				t.Error("expected error")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Error(err)
			}
			ended := time.Now()
			rtime := ended.Sub(started)
			if rtime < 500*time.Millisecond {
				t.Errorf("runtime of %s, less than 500ms", rtime)
			}
			if rtime > 550*time.Millisecond {
				t.Errorf("runtime of %s, expected 500ms, with some buffer", rtime)
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			started := time.Now()
			err := <-ContextErrorChannel(ctx)
			if err == nil {
				t.Error("expected error")
			}
			if !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
			ended := time.Now()
			rtime := ended.Sub(started)
			if rtime > time.Millisecond {
				t.Error("unexpected rtime:", rtime)
			}
			if rtime == 0 {
				t.Error("unexpected rtime:", rtime)
			}
		})
	})

	t.Run("Recv", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
			case <-timer.C:
				t.Error("soft timeout")
				panic("bailout")
			}
		}()
		ch := make(chan int, 1)
		risky.Send(ch, func() int { return 42 })
		out := risky.Recv(ch)
		check.Equal(t, 42, out)
		close(ch)
		out = risky.Recv(ch)
		check.Zero(t, out)
	})
}
