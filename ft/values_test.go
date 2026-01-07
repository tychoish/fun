package ft

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
)

func TestPtr(t *testing.T) {
	out := Ptr(123)
	assert.True(t, out != nil)
	check.Equal(t, *out, 123)

	var nptr *string //nolint:staticcheck

	var anyif any
	check.True(t, internal.IsNil(anyif))
	check.True(t, anyif == nil)
	anyif = nptr

	// interfaces holding nil values are not nil
	check.True(t, anyif != nil) //nolint:staticcheck
	// however...
	check.True(t, internal.IsNil(anyif))

	// this is gross, but we have a pointer (non-nil) to an object
	// that is a pointer, which is nil.
	var dptr *string
	st := Ptr(dptr)
	assert.True(t, st != nil)
	assert.True(t, *st == nil)
	assert.Type[**string](t, st)

	check.True(t, internal.IsPtr(dptr))
	check.True(t, internal.IsPtr(t))
	check.True(t, !(internal.IsPtr(DerefZ(dptr))))
	check.True(t, !(internal.IsPtr(3)))
	t.Run("Ref", func(t *testing.T) {
		var strptr *string
		assert.True(t, strptr == nil)
		assert.Equal(t, "", DerefZ(strptr))

		strptr = Ptr("")
		assert.True(t, strptr != nil)
		assert.Equal(t, "", Deref(strptr))
		assert.Equal(t, "", DerefZ(strptr))

		strptr = Ptr("hello")
		assert.True(t, strptr != nil)
		assert.Equal(t, "hello", Deref(strptr))
	})
}

func TestDefault(t *testing.T) {
	assert.Equal(t, Default(0, 42), 42)
	assert.Equal(t, Default(77, 42), 77)

	assert.Equal(t, Default("", "kip"), "kip")
	assert.Equal(t, Default("buddy", "kip"), "buddy")

	t.Run("IsZero", func(t *testing.T) {
		assert.True(t, !IsZero(100))
		assert.True(t, !IsZero(true))
		assert.True(t, !IsZero("hello world"))
		assert.True(t, !IsZero(time.Now()))
		assert.True(t, IsZero(0))
		assert.True(t, IsZero(false))
		assert.True(t, IsZero(""))
		assert.True(t, IsZero(time.Time{}))
	})
}

func TestContexts(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		var cc context.Context
		WithContextTimeoutCall(10*time.Millisecond, func(ctx context.Context) {
			assert.NotError(t, ctx.Err())
			cc = ctx
			time.Sleep(100 * time.Millisecond)
			assert.Error(t, ctx.Err())
			assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		})
		assert.ErrorIs(t, cc.Err(), context.DeadlineExceeded)
	})
	t.Run("ScopeTimeout", func(t *testing.T) {
		var cc context.Context
		WithContextTimeoutCall(10*time.Millisecond, func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
	t.Run("Scope", func(t *testing.T) {
		var cc context.Context
		WithContextCall(func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
	t.Run("Do", func(t *testing.T) {
		var cc context.Context
		assert.Equal(t, 42, WithContextDo(func(ctx context.Context) int {
			cc = ctx
			check.NotError(t, ctx.Err())
			return 42
		}))
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
}
