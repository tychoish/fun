package erc

import (
	"errors"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/ers"
)

func TestCollections(t *testing.T) {
	t.Run("Merge", func(t *testing.T) {
		t.Run("Both", func(t *testing.T) {
			e1 := &errorTest{val: 100}
			e2 := &errorTest{val: 200}

			err := Join(e1, e2)

			if err == nil {
				t.Error("should be an error")
			}
			if !ers.Is(err, e1) {
				t.Error("err IS er1")
				t.Log("err:", err)
				t.Log("er1:", e1)
			}

			if !errors.Is(err, e2) {
				t.Error("shold be er2", err, "=<=>=", e2)
			}
			cp := &errorTest{}
			if !errors.As(err, &cp) {
				t.Logf("%T, %s", cp, cp)
				t.Logf("%T, %s", err, err)
				t.Error(cp, "<=<=>=>", err)
				t.Error("should err as", err, cp)
			}
			if cp.val != e2.val {
				t.Error(cp.val, "=<=>=", e1.val)
				t.Log(cp)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := ers.Join(e1, nil)

			t.Cleanup(func() {
				if t.Failed() {
					t.Log(err)
					t.Logf("%T", err)
				}
			})

			assert.ErrorIs(t, err, e1)
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := ers.Join(nil, e1)
			assert.ErrorIs(t, err, e1)
		})
		t.Run("Neither", func(t *testing.T) {
			err := ers.Join(nil, nil)
			assert.NotError(t, err)
		})
	})
	t.Run("Collapse", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			if err := ers.Join(); err != nil {
				t.Error("should be nil", err)
			}
		})
		t.Run("One", func(t *testing.T) {
			const e ers.Error = "forty-two"
			err := ers.Join(e)
			if !errors.Is(err, e) {
				t.Error(err, e)
			}
		})
		t.Run("Many", func(t *testing.T) {
			const e0 ers.Error = "forty-two"
			const e1 ers.Error = "forty-three"
			err := ers.Join(e0, e1)
			if !errors.Is(err, e1) {
				t.Error(err, e1)
			}
			if !errors.Is(err, e0) {
				t.Error(err, e0)
			}
			t.Log(err)
			errs := ers.Unwind(err)
			if len(errs) != 2 {
				t.Error(errs)
			}
		})
	})
	t.Run("RecoverHookErrorSlice", func(t *testing.T) {
		ec := new(Collector)
		assert.NotPanic(t, func() {
			defer RecoverHook(ec, nil)
			panic([]error{ers.ErrImmutabilityViolation, ers.ErrInvalidInput})
		})
		err := ec.Resolve()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		assert.ErrorIs(t, err, ers.ErrImmutabilityViolation)
		assert.Equal(t, len(ers.Unwind(err)), 3)
	})
}
