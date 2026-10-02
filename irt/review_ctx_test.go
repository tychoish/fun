package irt

import (
	"context"
	"testing"
)

func TestCancelledContextBeatsReadyChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("sendTo", func(t *testing.T) {
		for range 200 {
			if sendTo(ctx, 1, make(chan int, 1)) {
				t.Fatal("sendTo succeeded on a cancelled ctx")
			}
		}
	})
	t.Run("recieveFrom", func(t *testing.T) {
		for range 200 {
			ch := make(chan int, 1)
			ch <- 1
			if _, ok := recieveFrom(ctx, ch); ok {
				t.Fatal("recieveFrom succeeded on a cancelled ctx")
			}
		}
	})
}
