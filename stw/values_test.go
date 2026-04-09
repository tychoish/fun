package stw

import (
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

func TestCheckOr(t *testing.T) {
	isPositive := func(n int) bool { return n > 0 }
	isEven := func(n int) bool { return n%2 == 0 }

	check.True(t, CheckOr(isPositive, isEven)(2))
	check.True(t, CheckOr(isPositive, isEven)(3))
	check.True(t, CheckOr(isPositive, isEven)(-2))
	check.True(t, !CheckOr(isPositive, isEven)(-3))
	check.True(t, !CheckOr[int]()(0))

	// short-circuits: second op never called if first returns true
	called := false
	second := func(int) bool { called = true; return false }
	CheckOr(isPositive, second)(1)
	check.True(t, !called)
}

func TestCheckAnd(t *testing.T) {
	isPositive := func(n int) bool { return n > 0 }
	isEven := func(n int) bool { return n%2 == 0 }

	check.True(t, CheckAnd(isPositive, isEven)(4))
	check.True(t, !CheckAnd(isPositive, isEven)(3))
	check.True(t, !CheckAnd(isPositive, isEven)(-2))
	check.True(t, CheckAnd[int]()(0))

	// short-circuits: second op never called if first returns false
	called := false
	second := func(int) bool { called = true; return true }
	CheckAnd(isEven, second)(3)
	check.True(t, !called)
}

func TestDerefOrZ(t *testing.T) {
	t.Run("NilPointer", func(t *testing.T) {
		var p *int
		assert.Equal(t, 0, DerefOrZ(p))
	})
	t.Run("NonNilPointer", func(t *testing.T) {
		v := 42
		assert.Equal(t, 42, DerefOrZ(&v))
	})
	t.Run("ZeroValuePointer", func(t *testing.T) {
		v := 0
		assert.Equal(t, 0, DerefOrZ(&v))
	})
	t.Run("NoArgs", func(t *testing.T) {
		assert.Equal(t, 0, DerefOrZ[int]())
	})
	t.Run("FirstNonNilWins", func(t *testing.T) {
		a := 1
		b := 2
		assert.Equal(t, 1, DerefOrZ(&a, &b))
	})
	t.Run("SkipsLeadingNils", func(t *testing.T) {
		v := 99
		assert.Equal(t, 99, DerefOrZ[int](nil, nil, &v))
	})
	t.Run("AllNil", func(t *testing.T) {
		assert.Equal(t, "", DerefOrZ[string](nil, nil))
	})
	t.Run("String", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", DerefOrZ(&s))
	})
	t.Run("ZeroStringPointer", func(t *testing.T) {
		s := ""
		assert.Equal(t, "", DerefOrZ((*string)(nil), &s))
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
