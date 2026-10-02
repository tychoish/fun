package irt

import (
	"context"
	"iter"
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

func TestCollectManyArgsReview(t *testing.T) {
	if c := cap(Collect(Slice([]int{}), 0, 0, 5)); c != 5 {
		t.Fatalf("cap = %d, want 5", c)
	}
	if c := cap(Collect(Slice([]int{}), 0, 0, 0, 7, 0)); c != 7 {
		t.Fatalf("cap = %d, want 7", c)
	}
	if c := cap(Collect(Slice([]int{}), 0, 0, -3)); c != 0 {
		t.Fatalf("cap = %d, want 0", c)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for two non-zero args")
			}
		}()
		Collect(Slice([]int{}), 0, 2, 0, 3)
	}()
}

func TestChain2NilInnerReview(t *testing.T) {
	in := Slice([]iter.Seq2[int, int]{nil, Zip(Slice([]int{1}), Slice([]int{2})), nil})
	out := Collect2(Chain2(in))
	if len(out) != 1 || out[1] != 2 {
		t.Fatalf("got %v", out)
	}
}
