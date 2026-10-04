package pubsub

import (
	"context"
	"errors"
	"testing"
)

// These tests pin the decided uniform error-precedence rule for Queue's
// blocking operations: closed/draining is reported before a context
// cancellation error. Draining only gates push-side operations; a
// draining queue still lets consumers pop. Shutdown is the one
// exception with its own precedence (ctx, then drain, then close): on
// an open queue a cancelled ctx wins and Shutdown leaves the queue
// open, but on an already-closed queue Shutdown (like Drain) reports
// ErrQueueClosed regardless of ctx state, even with zero items left.
//
// On an open queue, a cancelled ctx still wins over a ready item or
// slot (the Stop()-keeps-backlog guarantee, pinned separately by
// TestBrokerStopKeepsBacklog): that part is unchanged and re-pinned
// here alongside the new closed/draining cases.

func TestQueueErrorPrecedence(t *testing.T) {
	t.Run("WaitPop", func(t *testing.T) {
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
				q := NewUnlimitedQueue[int]()
				if tc.withItem {
					if err := q.Push(1); err != nil {
						t.Fatal(err)
					}
				}
				if tc.closed {
					if err := q.Close(); err != nil {
						t.Fatal(err)
					}
				}

				ctx := context.Background()
				if tc.cancelled {
					ctx = cancelled()
				}

				_, err := q.WaitPop(ctx)
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
				q := NewUnlimitedQueue[int]()
				if tc.closed {
					if err := q.Close(); err != nil {
						t.Fatal(err)
					}
				}
				ctx := context.Background()
				if tc.cancelled {
					ctx = cancelled()
				}
				err := q.Drain(ctx)
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
		t.Run("cancelled_ctx_open_queue_stays_open", func(t *testing.T) {
			q := NewUnlimitedQueue[int]()
			if err := q.Shutdown(cancelled()); !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v", err)
			}
			// the queue must still be open and usable.
			if err := q.Push(1); err != nil {
				t.Fatalf("queue should still be open: %v", err)
			}
			if err := q.Close(); err != nil {
				t.Fatalf("explicit close should still work: %v", err)
			}
		})
		t.Run("already_closed_reports_closed_regardless_of_ctx", func(t *testing.T) {
			for _, cancel := range []bool{false, true} {
				q := NewUnlimitedQueue[int]()
				if err := q.Close(); err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				if cancel {
					ctx = cancelled()
				}
				// zero items remain; this is the already-closed+empty gap.
				if err := q.Shutdown(ctx); !errors.Is(err, ErrQueueClosed) {
					t.Fatalf("cancel=%v: got %v, want ErrQueueClosed", cancel, err)
				}
			}
		})
	})
}
