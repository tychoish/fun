package pubsub

import (
	"context"
	"errors"
	"testing"
)

// Deque mirrors the Queue precedence rule pinned in
// errorprecedence_queue_test.go: closed/draining before ctx
// cancellation, except for Shutdown's own precedence of ctx, then
// drain, then close.

func TestDequeErrorPrecedence(t *testing.T) {
	t.Run("WaitPopFront", func(t *testing.T) { testDequeWaitPopPrecedence(t, (*Deque[int]).WaitPopFront) })
	t.Run("WaitPopBack", func(t *testing.T) { testDequeWaitPopPrecedence(t, (*Deque[int]).WaitPopBack) })

	t.Run("Drain", func(t *testing.T) {
		cases := []struct {
			name      string
			closed    bool
			cancelled bool
			wantErr   error
		}{
			{"open_live_empty", false, false, nil},
			{"open_cancelled_empty", false, true, context.Canceled},
			{"closed_live_empty", true, false, ErrQueueClosed},
			{"closed_cancelled_empty", true, true, ErrQueueClosed}, // the already-closed+empty gap
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dq := NewUnlimitedDeque[int]()
				if tc.closed {
					if err := dq.Close(); err != nil {
						t.Fatal(err)
					}
				}
				ctx := context.Background()
				if tc.cancelled {
					ctx = cancelled()
				}
				err := dq.Drain(ctx)
				if tc.wantErr == nil {
					if err != nil {
						t.Fatalf("got err=%v", err)
					}
					return
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v, want %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("Shutdown", func(t *testing.T) {
		t.Run("cancelled_ctx_open_deque_stays_open", func(t *testing.T) {
			dq := NewUnlimitedDeque[int]()
			if err := dq.Shutdown(cancelled()); !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v", err)
			}
			if err := dq.PushBack(1); err != nil {
				t.Fatalf("deque should still be open: %v", err)
			}
			if err := dq.Close(); err != nil {
				t.Fatalf("explicit close should still work: %v", err)
			}
		})
		t.Run("already_closed_reports_closed_regardless_of_ctx", func(t *testing.T) {
			for _, cancel := range []bool{false, true} {
				dq := NewUnlimitedDeque[int]()
				if err := dq.Close(); err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				if cancel {
					ctx = cancelled()
				}
				if err := dq.Shutdown(ctx); !errors.Is(err, ErrQueueClosed) {
					t.Fatalf("cancel=%v: got %v, want ErrQueueClosed", cancel, err)
				}
			}
		})
	})
}

func testDequeWaitPopPrecedence(t *testing.T, op func(*Deque[int], context.Context) (int, error)) {
	t.Helper()
	cases := []struct {
		name      string
		closed    bool
		cancelled bool
		withItem  bool
		wantErr   error
	}{
		{"open_live_item", false, false, true, nil},
		{"open_cancelled_item", false, true, true, context.Canceled}, // unchanged: ctx wins over ready item
		{"closed_live_empty", true, false, false, ErrQueueClosed},
		{"closed_cancelled_empty", true, true, false, ErrQueueClosed}, // closed wins over ctx
		{"closed_live_withitem", true, false, true, ErrQueueClosed},
		{"closed_cancelled_withitem", true, true, true, ErrQueueClosed}, // closed wins over ctx
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dq := NewUnlimitedDeque[int]()
			if tc.withItem {
				if err := dq.PushBack(1); err != nil {
					t.Fatal(err)
				}
			}
			if tc.closed {
				if err := dq.Close(); err != nil {
					t.Fatal(err)
				}
			}

			ctx := context.Background()
			if tc.cancelled {
				ctx = cancelled()
			}

			_, err := op(dq, ctx)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("got err=%v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
