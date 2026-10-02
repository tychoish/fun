package pubsub

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestDequeIteratorOrder(t *testing.T) {
	for name, tt := range map[string]struct {
		iter func(*Deque[int], context.Context) []int
		want []int
	}{
		"Front": {func(dq *Deque[int], ctx context.Context) []int { return slices.Collect(dq.IteratorFront(ctx)) }, []int{1, 2, 3}},
		"Back":  {func(dq *Deque[int], ctx context.Context) []int { return slices.Collect(dq.IteratorBack(ctx)) }, []int{3, 2, 1}},
		"WaitFront": {func(dq *Deque[int], ctx context.Context) []int {
			return slices.Collect(dq.IteratorWaitFront(ctx))
		}, []int{1, 2, 3}},
		"WaitBack": {func(dq *Deque[int], ctx context.Context) []int {
			return slices.Collect(dq.IteratorWaitBack(ctx))
		}, []int{3, 2, 1}},
	} {
		t.Run(name, func(t *testing.T) {
			dq := NewUnlimitedDeque[int]()
			for i := 1; i <= 3; i++ {
				if err := dq.PushBack(i); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if got := tt.iter(dq, ctx); !slices.Equal(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
