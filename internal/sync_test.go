package internal

import (
	"context"
	"testing"
)

func TestMnemonize(t *testing.T) {
	t.Run("Plain", func(t *testing.T) {
		const value string = "val"
		counter := 0
		op := func() string { counter++; return value }

		foo := Mnemonize(op)
		if counter != 0 {
			t.Error("should not call yet", counter)
		}
		if out := foo(); out != value {
			t.Error("wrong value", out, value)
		}
		if counter != 1 {
			t.Error("should call only once", counter)
		}
		for i := 0; i < 128; i++ {
			if out := foo(); out != value {
				t.Error("wrong value", out, value)
			}
			if counter != 1 {
				t.Error("should call only once", counter)
			}
		}
	})
	t.Run("PlainContext", func(t *testing.T) {
		const value string = "val"
		counter := 0
		op := func(insideCtx context.Context) string {
			if insideCtx != BackgroundContext {
				t.Error("different contexts")
			}
			counter++
			return value
		}

		foo := MnemonizeContext(op)
		if counter != 0 {
			t.Error("should not call yet", counter)
		}
		if out := foo(BackgroundContext); out != value {
			t.Error("wrong value", out, value)
		}
		if counter != 1 {
			t.Error("should call only once", counter)
		}
		for i := 0; i < 128; i++ {
			if out := foo(BackgroundContext); out != value {
				t.Error("wrong value", out, value)
			}
			if counter != 1 {
				t.Error("should call only once", counter)
			}
		}
	})

}
