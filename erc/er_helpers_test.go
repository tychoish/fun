package erc

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
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
				t.Error("cp.val", cp.val, "e1.val", e1.val, "e2", e2.val)
				t.Log(cp, cp.val == e2.val)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Join(e1, nil)

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
			err := Join(nil, e1)
			assert.ErrorIs(t, err, e1)
		})
		t.Run("Neither", func(t *testing.T) {
			err := Join(nil, nil)
			assert.NotError(t, err)
		})
	})
	t.Run("Collapse", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			if err := Join(); err != nil {
				t.Error("should be nil", err)
			}
		})
		t.Run("One", func(t *testing.T) {
			const e ers.Error = "forty-two"
			err := Join(e)
			if !errors.Is(err, e) {
				t.Error(err, e)
			}
		})
		t.Run("Many", func(t *testing.T) {
			const e0 ers.Error = "forty-two"
			const e1 ers.Error = "forty-three"
			err := Join(e0, e1)
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
			defer ec.WithRecoverHook(nil)
			panic([]error{ers.ErrImmutabilityViolation, ers.ErrInvalidInput})
		})
		err := ec.Resolve()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		assert.ErrorIs(t, err, ers.ErrImmutabilityViolation)
		assert.Equal(t, len(ers.Unwind(err)), 3)
	})
	t.Run("Wrapped", func(t *testing.T) {
		t.Run("Errorf", func(t *testing.T) {

			err := errors.New("base")
			for i := 0; i < 100; i++ {
				err = fmt.Errorf("wrap %d: %w", i, err)
			}

			errs := internal.Unwind(err)
			if len(errs) != 101 {
				t.Error(len(errs))
			}
			if errs[100].Error() != "base" {
				t.Error(errs[100])
			}
			check.Equal(t, 101, len(internal.Unwind(err)))
		})
		t.Run("Wrapf", func(t *testing.T) {
			base := errors.New("base")
			err := base
			for i := 0; i < 100; i++ {
				err = Wrapf(err, "iter=%d", i)
				t.Log(err)
				assert.Equal(t, err.(*Collector).Len(), 2)
			}

			assert.ErrorIs(t, err, base)
			assert.True(t, strings.HasPrefix(err.Error(), "base: iter=0: iter=1"))
		})
		t.Run("Wrap", func(t *testing.T) {
			base := errors.New("base")
			err := base
			for i := 0; i < 100; i++ {
				err = Wrap(err, "annotation")
				assert.Equal(t, err.(*Collector).Len(), 2)
			}

			assert.ErrorIs(t, err, base)
			assert.True(t, strings.HasPrefix(err.Error(), "base: annotation: annotation"))
		})
		t.Run("WrapfNil", func(t *testing.T) {
			assert.NotError(t, Wrapf(nil, "foo %s", "bar"))
		})
		t.Run("WrapNil", func(t *testing.T) {
			assert.NotError(t, Wrap(nil, "foo"))
		})

	})
}
