package ft

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChannels(t *testing.T) {
	t.Run("Canceled", func(t *testing.T) {
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
}
