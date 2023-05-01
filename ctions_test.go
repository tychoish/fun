package fun

import (
	"context"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestWhen(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		out := WhenDo(true, func() int { return 100 })
		check.Equal(t, out, 100)

		out = WhenDo(false, func() int { return 100 })
		check.Equal(t, out, 0)
	})
	t.Run("Call", func(t *testing.T) {
		called := false
		WhenCall(true, func() { called = true })
		check.True(t, called)

		called = false
		WhenCall(false, func() { called = true })
		check.True(t, !called)
	})
}

func TestSend(t *testing.T) {
	t.Run("Send", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int, 2)
		err := Blocking(ch).Send(ctx, 1)
		assert.NotError(t, err)
		assert.Equal(t, <-ch, 1)

		err = NonBlocking(ch).Send(ctx, 3)
		assert.NotError(t, err)
		assert.Equal(t, <-ch, 3)
	})
	t.Run("ClosedSend", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int)
		close(ch)
		err := Blocking(ch).Send(ctx, 43)
		assert.ErrorIs(t, err, io.EOF)
	})
	t.Run("NonBlocking", func(t *testing.T) {
		t.Run("Send", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int)

			err := NonBlocking(ch).Send(ctx, 3)
			assert.ErrorIs(t, err, ErrSkippedBlockingSend)
			close(ch)
			out, ok := <-ch
			assert.True(t, !ok)
			assert.Zero(t, out)
		})
		t.Run("Check", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int, 1)

			sent := NonBlocking(ch).Check(ctx, 3)
			assert.True(t, sent)
			out, ok := <-ch
			assert.True(t, ok)
			assert.Equal(t, 3, out)
		})
		t.Run("Check", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int, 1)

			NonBlocking(ch).Ignore(ctx, 3)
			out, ok := <-ch
			assert.True(t, ok)
			assert.Equal(t, 3, out)
		})
	})
	t.Run("Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan int)

		err := Blocking(ch).Send(ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		err = NonBlocking(ch).Send(ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		assert.True(t, !NonBlocking(ch).Check(ctx, 1))

		err = (Send[int]{ch: ch}).Send(ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, io.EOF)
		err = (Send[int]{mode: 42, ch: ch}).Send(ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestContains(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		assert.True(t, Contains(1, []int{12, 3, 44, 1}))
	})
	t.Run("NotExists", func(t *testing.T) {
		assert.True(t, !Contains(1, []int{12, 3, 44}))
	})
}
