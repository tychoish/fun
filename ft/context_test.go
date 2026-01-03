package ft_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestWithContext(t *testing.T) {
	t.Run("TimeoutCall", func(t *testing.T) {
		t.Run("TimeoutOccurs", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextTimeoutCall(10*time.Millisecond, func(ctx context.Context) {
				capturedCtx = ctx
				assert.NotError(t, ctx.Err())
				time.Sleep(50 * time.Millisecond)
				assert.Error(t, ctx.Err())
				assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.DeadlineExceeded)
		})

		t.Run("NoTimeoutBeforeDeadline", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextTimeoutCall(100*time.Millisecond, func(ctx context.Context) {
				capturedCtx = ctx
				assert.NotError(t, ctx.Err())
			})
			// Context should be canceled after function returns
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})

		t.Run("ContextHasDeadline", func(t *testing.T) {
			ft.WithContextTimeoutCall(50*time.Millisecond, func(ctx context.Context) {
				deadline, ok := ctx.Deadline()
				check.True(t, ok)
				check.True(t, time.Until(deadline) <= 50*time.Millisecond)
			})
		})
	})

	t.Run("Call", func(t *testing.T) {
		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextCall(func(ctx context.Context) {
				capturedCtx = ctx
				assert.NotError(t, ctx.Err())
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})

		t.Run("ContextActiveInsideCall", func(t *testing.T) {
			ft.WithContextCall(func(ctx context.Context) {
				check.NotError(t, ctx.Err())
				select {
				case <-ctx.Done():
					t.Error("context should not be canceled inside call")
				default:
					// Expected
				}
			})
		})
	})

	t.Run("CallOk", func(t *testing.T) {
		t.Run("ReturnsTrue", func(t *testing.T) {
			result := ft.WithContextCallOk(func(ctx context.Context) bool {
				check.NotError(t, ctx.Err())
				return true
			})
			check.True(t, result)
		})

		t.Run("ReturnsFalse", func(t *testing.T) {
			result := ft.WithContextCallOk(func(ctx context.Context) bool {
				check.NotError(t, ctx.Err())
				return false
			})
			check.True(t, !result)
		})

		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextCallOk(func(ctx context.Context) bool {
				capturedCtx = ctx
				return true
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})
	})

	t.Run("CallErr", func(t *testing.T) {
		t.Run("ReturnsNil", func(t *testing.T) {
			err := ft.WithContextCallErr(func(ctx context.Context) error {
				check.NotError(t, ctx.Err())
				return nil
			})
			check.NotError(t, err)
		})

		t.Run("ReturnsError", func(t *testing.T) {
			expectedErr := errors.New("test error")
			err := ft.WithContextCallErr(func(ctx context.Context) error {
				check.NotError(t, ctx.Err())
				return expectedErr
			})
			check.ErrorIs(t, err, expectedErr)
		})

		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextCallErr(func(ctx context.Context) error {
				capturedCtx = ctx
				return nil
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})
	})

	t.Run("Do", func(t *testing.T) {
		t.Run("ReturnsValue", func(t *testing.T) {
			result := ft.WithContextDo(func(ctx context.Context) int {
				check.NotError(t, ctx.Err())
				return 42
			})
			check.Equal(t, result, 42)
		})

		t.Run("ReturnsString", func(t *testing.T) {
			result := ft.WithContextDo(func(ctx context.Context) string {
				check.NotError(t, ctx.Err())
				return "hello"
			})
			check.Equal(t, result, "hello")
		})

		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextDo(func(ctx context.Context) int {
				capturedCtx = ctx
				return 100
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})
	})

	t.Run("DoOk", func(t *testing.T) {
		t.Run("ReturnsValueAndTrue", func(t *testing.T) {
			val, ok := ft.WithContextDoOk(func(ctx context.Context) (int, bool) {
				check.NotError(t, ctx.Err())
				return 42, true
			})
			check.Equal(t, val, 42)
			check.True(t, ok)
		})

		t.Run("ReturnsValueAndFalse", func(t *testing.T) {
			val, ok := ft.WithContextDoOk(func(ctx context.Context) (string, bool) {
				check.NotError(t, ctx.Err())
				return "test", false
			})
			check.Equal(t, val, "test")
			check.True(t, !ok)
		})

		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextDoOk(func(ctx context.Context) (int, bool) {
				capturedCtx = ctx
				return 10, true
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})
	})

	t.Run("DoErr", func(t *testing.T) {
		t.Run("ReturnsValueNoError", func(t *testing.T) {
			val, err := ft.WithContextDoErr(func(ctx context.Context) (int, error) {
				check.NotError(t, ctx.Err())
				return 42, nil
			})
			check.Equal(t, val, 42)
			check.NotError(t, err)
		})

		t.Run("ReturnsValueAndError", func(t *testing.T) {
			expectedErr := errors.New("test error")
			val, err := ft.WithContextDoErr(func(ctx context.Context) (string, error) {
				check.NotError(t, ctx.Err())
				return "result", expectedErr
			})
			check.Equal(t, val, "result")
			check.ErrorIs(t, err, expectedErr)
		})

		t.Run("ContextCanceledAfterReturn", func(t *testing.T) {
			var capturedCtx context.Context
			ft.WithContextDoErr(func(ctx context.Context) (int, error) {
				capturedCtx = ctx
				return 10, nil
			})
			assert.ErrorIs(t, capturedCtx.Err(), context.Canceled)
		})
	})
}

func TestWrapWithContext(t *testing.T) {
	t.Run("Call", func(t *testing.T) {
		t.Run("WrapsFunction", func(t *testing.T) {
			var called atomic.Bool
			var receivedCtx context.Context

			ctx := context.Background()
			wrapped := ft.WrapWithContextCall(ctx, func(c context.Context) {
				called.Store(true)
				receivedCtx = c
			})

			// Function not called yet
			check.True(t, !called.Load())

			// Call wrapped function
			wrapped()

			// Function was called
			check.True(t, called.Load())
			check.Equal(t, receivedCtx, ctx)
		})

		t.Run("CapturesContext", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var capturedCtx context.Context
			wrapped := ft.WrapWithContextCall(ctx, func(c context.Context) {
				capturedCtx = c
			})

			wrapped()

			// Should have received the same context
			check.Equal(t, capturedCtx, ctx)
		})

		t.Run("MultipleInvocations", func(t *testing.T) {
			var count atomic.Int32
			ctx := context.Background()

			wrapped := ft.WrapWithContextCall(ctx, func(c context.Context) {
				count.Add(1)
			})

			wrapped()
			wrapped()
			wrapped()

			check.Equal(t, count.Load(), int32(3))
		})
	})

	t.Run("Do", func(t *testing.T) {
		t.Run("WrapsAndReturnsValue", func(t *testing.T) {
			ctx := context.Background()
			wrapped := ft.WrapWithContextDo(ctx, func(c context.Context) int {
				return 42
			})

			result := wrapped()
			check.Equal(t, result, 42)
		})

		t.Run("CapturesContext", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var capturedCtx context.Context
			wrapped := ft.WrapWithContextDo(ctx, func(c context.Context) string {
				capturedCtx = c
				return "hello"
			})

			result := wrapped()

			check.Equal(t, result, "hello")
			check.Equal(t, capturedCtx, ctx)
		})

		t.Run("ReturnsPointer", func(t *testing.T) {
			ctx := context.Background()
			expected := &struct{ Value int }{Value: 100}

			wrapped := ft.WrapWithContextDo(ctx, func(c context.Context) *struct{ Value int } {
				return expected
			})

			result := wrapped()
			check.Equal(t, result, expected)
			check.Equal(t, result.Value, 100)
		})
	})

	t.Run("DoOk", func(t *testing.T) {
		t.Run("WrapsAndReturnsValueAndTrue", func(t *testing.T) {
			ctx := context.Background()
			wrapped := ft.WrapWithContextDoOk(ctx, func(c context.Context) (int, bool) {
				return 42, true
			})

			val, ok := wrapped()
			check.Equal(t, val, 42)
			check.True(t, ok)
		})

		t.Run("WrapsAndReturnsValueAndFalse", func(t *testing.T) {
			ctx := context.Background()
			wrapped := ft.WrapWithContextDoOk(ctx, func(c context.Context) (string, bool) {
				return "test", false
			})

			val, ok := wrapped()
			check.Equal(t, val, "test")
			check.True(t, !ok)
		})

		t.Run("CapturesContext", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var capturedCtx context.Context
			wrapped := ft.WrapWithContextDoOk(ctx, func(c context.Context) (int, bool) {
				capturedCtx = c
				return 10, true
			})

			val, ok := wrapped()

			check.Equal(t, val, 10)
			check.True(t, ok)
			check.Equal(t, capturedCtx, ctx)
		})
	})

	t.Run("DoErr", func(t *testing.T) {
		t.Run("WrapsAndReturnsValueNoError", func(t *testing.T) {
			ctx := context.Background()
			wrapped := ft.WrapWithContextDoErr(ctx, func(c context.Context) (int, error) {
				return 42, nil
			})

			val, err := wrapped()
			check.Equal(t, val, 42)
			check.NotError(t, err)
		})

		t.Run("WrapsAndReturnsValueAndError", func(t *testing.T) {
			ctx := context.Background()
			expectedErr := errors.New("test error")

			wrapped := ft.WrapWithContextDoErr(ctx, func(c context.Context) (string, error) {
				return "result", expectedErr
			})

			val, err := wrapped()
			check.Equal(t, val, "result")
			check.ErrorIs(t, err, expectedErr)
		})

		t.Run("CapturesContext", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var capturedCtx context.Context
			wrapped := ft.WrapWithContextDoErr(ctx, func(c context.Context) (int, error) {
				capturedCtx = c
				return 10, nil
			})

			val, err := wrapped()

			check.Equal(t, val, 10)
			check.NotError(t, err)
			check.Equal(t, capturedCtx, ctx)
		})

		t.Run("WithCanceledContext", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			wrapped := ft.WrapWithContextDoErr(ctx, func(c context.Context) (int, error) {
				return 0, c.Err()
			})

			_, err := wrapped()
			check.ErrorIs(t, err, context.Canceled)
		})
	})
}
