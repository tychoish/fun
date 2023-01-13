package fun

import (
	"errors"
	"strings"
	"testing"
)

func catcherIsEmpty(t *testing.T, catcher *ErrorCollector) {
	t.Helper()

	if catcher == nil {
		t.Fatal("test issue")
	}

	if catcher.Len() != 0 {
		t.Error("should have zero errors")
	}
	if catcher.HasErrors() {
		t.Error("should not have errors")
	}
	if catcher.Resolve() != nil {
		t.Error("should produce nil error")
	}
}

func catcherHasErrors(t *testing.T, expectedNum int, catcher *ErrorCollector) {
	t.Helper()

	if catcher == nil || expectedNum <= 0 {
		t.Fatal("test issue", catcher, expectedNum)
	}

	if catcher.Len() != expectedNum {
		t.Error("should have expected number of errors", expectedNum, catcher.Len())
	}
	if !catcher.HasErrors() {
		t.Error("should have errors")
	}
	if catcher.Resolve() == nil {
		t.Error("should produce an error")
	}
}

func TestErrors(t *testing.T) {
	const errval = "ERRO=42"
	t.Run("InitialState", func(t *testing.T) {
		catcher := &ErrorCollector{}
		catcherIsEmpty(t, catcher)
	})
	t.Run("AddNilErrors", func(t *testing.T) {
		catcher := &ErrorCollector{}
		catcher.Add(nil)
		catcherIsEmpty(t, catcher)
		var err error
		catcher.Add(err)
		catcherIsEmpty(t, catcher)
	})
	t.Run("SingleError", func(t *testing.T) {
		catcher := &ErrorCollector{}

		catcher.Add(errors.New(errval))
		catcherHasErrors(t, 1, catcher)

		err := catcher.Resolve()
		if err.Error() != errval {
			t.Error("unexpected error value:", err)
		}
	})
	t.Run("ErrorsAreCached", func(t *testing.T) {
		catcher := &ErrorCollector{}

		catcher.Add(errors.New(errval))
		catcherHasErrors(t, 1, catcher)

		err := catcher.Resolve()
		err2 := catcher.Resolve()
		if err.Error() != err2.Error() {
			t.Error("unexpected error value:", err)
		}
		if err != err2 {
			t.Error("should be different objects")
		}
	})
	t.Run("CacheRefreshesAsNeeded", func(t *testing.T) {
		catcher := &ErrorCollector{}
		catcher.Add(errors.New(errval))
		catcherHasErrors(t, 1, catcher)
		err := catcher.Resolve()
		catcher.Add(errors.New(errval))
		catcherHasErrors(t, 2, catcher)

		err2 := catcher.Resolve()
		if !strings.Contains(err2.Error(), err.Error()) {
			t.Error("errors should remain")
		}
	})
}
