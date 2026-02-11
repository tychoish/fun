package erc

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

func TestPanics(t *testing.T) {
	t.Run("NilInput", func(t *testing.T) {
		err := ParsePanic(nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ErrorSlice", func(t *testing.T) {
		err := ParsePanic([]error{ers.New("one"), ers.WithTime(ers.New("two"))})
		if err == nil {
			t.Fatal("expected error")
		}
		check.Equal(t, 3, len(internal.Unwind(err)))
	})
	t.Run("ArbitraryObject", func(t *testing.T) {
		err := ParsePanic(t)
		if err == nil {
			t.Fatal("expected error")
		}

		check.Substring(t, err.Error(), "testing.T")
		if !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Error("not wrapped", err)
		}
	})

	t.Run("TwoErrors", func(t *testing.T) {
		err := ParsePanic(io.EOF)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, io.EOF) {
			t.Error("not EOF", err)
		}
		if !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Error("not wrapped", err)
		}
	})
	t.Run("NotErrorObject", func(t *testing.T) {
		err := ParsePanic("EOF")
		if err == nil {
			t.Fatal("expected error")
		}
		if errors.Is(err, io.EOF) {
			t.Error(err)
		}
		if !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Error("not wrapped", err)
		}
		if !strings.Contains(err.Error(), io.EOF.Error()) {
			t.Error(io.EOF.Error(), "NOT IN", err.Error())
		}
		if !strings.Contains(err.Error(), string(ers.ErrRecoveredPanic)) {
			t.Error(ers.ErrRecoveredPanic.Error(), "NOT IN", err.Error())
		}
	})
	t.Run("InvariantViolation", func(t *testing.T) {
		assert.True(t, ers.IsInvariantViolation(ers.ErrInvariantViolation))
		assert.True(t, ers.IsInvariantViolation(errors.Join(io.EOF, ers.Error("hello"), ers.ErrInvariantViolation)))
		assert.True(t, !ers.IsInvariantViolation(nil))
		assert.True(t, !ers.IsInvariantViolation(9001))
		assert.True(t, !ers.IsInvariantViolation(io.EOF))
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		ec := &Collector{}
		input := []any{nil, ers.Error("hi"), 1, true, "   ", "hi", func() string { return "boop" }, func() string { return "" }}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 2)

		var nerr error
		ec = &Collector{}
		input = []any{nil, ers.Error("hi"), func() error { return nil }, nerr, 2, false, "etc..."}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 2)

		ec = &Collector{}
		input = []any{ers.Error("hi"), ers.Error("hi"), ers.Error("hi"), ers.Error("hi"), time.Now()}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 5)
	})
	t.Run("InvariantError", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			assert.Equal[error](t, ers.ErrInvariantViolation, NewInvariantError())
			assert.ErrorIs(t, NewInvariantError(), ers.ErrInvariantViolation)
		})
		t.Run("SingleNil", func(t *testing.T) {
			assert.Equal[error](t, ers.ErrInvariantViolation, NewInvariantError(nil))
			assert.ErrorIs(t, NewInvariantError(nil), ers.ErrInvariantViolation)
		})
		t.Run("StringFunc", func(t *testing.T) {
			ts := time.Now()
			assert.ErrorIs(t, NewInvariantError(ts.String), ers.ErrInvariantViolation)
			assert.True(t, strings.Contains(NewInvariantError(ts.String).Error(), ts.String()))
		})
		t.Run("Stringer", func(t *testing.T) {
			ts := time.Now()
			assert.ErrorIs(t, NewInvariantError(ts), ers.ErrInvariantViolation)
			assert.True(t, strings.Contains(NewInvariantError(ts).Error(), ts.String()))
		})
		t.Run("One", func(t *testing.T) {
			vals := []any{"hello", time.Now(), ers.New("hello"), func() error { return errors.New("hello") }}
			for val := range slices.Values(vals) {
				err := NewInvariantError(val)
				check.Error(t, err)
				check.ErrorIs(t, err, ers.ErrInvariantViolation)
				check.Equal(t, 2, len(ers.Unwind(err)))
			}
		})
		t.Run("Many", func(t *testing.T) {
			input := []any{errors.New("hello"), ers.New("hello"), io.EOF}
			err := NewInvariantError(input...)
			check.ErrorIs(t, err, ers.ErrInvariantViolation)
			check.Equal(t, len(input)+1, len(ers.Unwind(err)))
		})
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := NewInvariantError("math is a construct")
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.True(t, ers.IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := NewInvariantError(err)

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := NewInvariantError(42, err)
			if !errors.Is(se, err) {
				t.Log("se", se)
				t.Log("err", err)
				t.FailNow()
			}
			if !strings.Contains(se.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("NoError", func(t *testing.T) {
			err := NewInvariantError(42)
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("CheckError", func(t *testing.T) {
			if ers.IsInvariantViolation(nil) {
				t.Error("nil error shouldn't read as invariant")
			}
			if ers.IsInvariantViolation(errors.New("foo")) {
				t.Error("arbitrary errors are not invariants")
			}
		})
		t.Run("ZeroFilterCases", func(t *testing.T) {
			err := NewInvariantError("", nil, nil, "")
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrInvariantViolation)
			es := ers.Unwind(err)
			check.Equal(t, len(es), 1)
		})
		t.Run("Future", func(t *testing.T) {
			t.Run("Single", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := NewInvariantError(op)
				check.Equal(t, count, 1)
				check.ErrorIs(t, err, ers.ErrInvalidInput)
			})
			t.Run("Multi", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := NewInvariantError(op, op, op, op, op, op, op, op)
				check.Equal(t, count, 8)
				check.ErrorIs(t, err, ers.ErrInvalidInput)

				errs := ers.Unwind(err)
				check.Equal(t, len(errs), 9)
				for idx := range errs {
					t.Log(idx, "/", len(errs), errs[idx])
				}
			})
		})
		t.Run("AlwaysErrors", func(t *testing.T) {
			assert.Error(t, NewInvariantError())
			assert.ErrorIs(t, NewInvariantError(), ers.ErrInvariantViolation)
		})
		t.Run("LongInvariant", func(t *testing.T) {
			err := NewInvariantError("math is a construct", "1 == 2")
			if err == nil {
				t.FailNow()
			}
			if !strings.Contains(err.Error(), "construct 1 == 2") {
				t.Error(err)
			}
		})
	})
}

func TestPanic(t *testing.T) {
	t.Run("InvariantOk", func(t *testing.T) {
		t.Run("TrueCondition", func(t *testing.T) {
			assert.NotPanic(t, func() {
				InvariantOk(true)
			})
		})
		t.Run("FalseCondition", func(t *testing.T) {
			var err error
			defer func() {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ers.ErrInvariantViolation)
				assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
				assert.True(t, ers.IsInvariantViolation(err))
			}()
			defer func() { err = ParsePanic(recover()) }()

			InvariantOk(false)
		})
		t.Run("TrueExpression", func(t *testing.T) {
			assert.NotPanic(t, func() {
				InvariantOk(1 == 1)
			})
		})
		t.Run("FalseExpression", func(t *testing.T) {
			var err error
			defer func() {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ers.ErrInvariantViolation)
				assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
				assert.True(t, ers.IsInvariantViolation(err))
			}()
			defer func() { err = ParsePanic(recover()) }()

			InvariantOk(1 == 2)
		})
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("NilError", func(t *testing.T) {
			assert.NotPanic(t, func() {
				Invariant(nil)
			})
		})
		t.Run("NonNilError", func(t *testing.T) {
			root := errors.New("test error")

			var err error

			defer func() {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ers.ErrInvariantViolation)
				assert.ErrorIs(t, err, root)
				assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
				assert.True(t, ers.IsInvariantViolation(err))
			}()
			defer func() { err = ParsePanic(recover()) }()

			Invariant(root)
		})
	})
	t.Run("Must", func(t *testing.T) {
		assert.Panic(t, func() { Must("32", errors.New("whoop")) })
		assert.Panic(t, func() { MustOk("32", false) })
		assert.NotPanic(t, func() { check.Equal(t, "32", Must("32", nil)) })
		assert.NotPanic(t, func() { check.Equal(t, "32", MustOk("32", true)) })
	})
}
