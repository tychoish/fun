package erc

import (
	"errors"
	"strings"
	"testing"

	"github.com/tychoish/fun/ers"
)

func TestErrorf(t *testing.T) {
	t.Run("NoArgs", func(t *testing.T) {
		err := Errorf("something went wrong")
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		var ce ers.Error
		if !errors.As(err, &ce) {
			t.Fatalf("expected ers.Error, got %T", err)
		}
		if string(ce) != "something went wrong" {
			t.Fatalf("unexpected message: %q", ce)
		}
	})

	t.Run("ArgsNoErrors", func(t *testing.T) {
		err := Errorf("x=%d y=%s", 5, "hello")
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		var ce ers.Error
		if !errors.As(err, &ce) {
			t.Fatalf("expected ers.Error, got %T", err)
		}
		if string(ce) != "x=5 y=hello" {
			t.Fatalf("unexpected message: %q", ce)
		}
	})

	t.Run("SingleTrailingWrapped", func(t *testing.T) {
		inner := errors.New("inner error")
		tmpl := "operation failed: %w"
		err := Errorf(tmpl, inner)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(err, inner) {
			t.Fatal("expected err to wrap inner")
		}
		msg := err.Error()
		if !strings.Contains(msg, "operation failed") {
			t.Fatalf("expected prefix in message, got: %q", msg)
		}
	})

	t.Run("SingleTrailingWrappedWithFormatArgs", func(t *testing.T) {
		inner := errors.New("inner error")
		tmpl := "x=%d operation failed: %w"
		err := Errorf(tmpl, 42, inner)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(err, inner) {
			t.Fatal("expected err to wrap inner")
		}
		msg := err.Error()
		if !strings.Contains(msg, "x=42") {
			t.Fatalf("expected formatted prefix in message, got: %q", msg)
		}
		if !strings.Contains(msg, "operation failed") {
			t.Fatalf("expected prefix text in message, got: %q", msg)
		}
	})

	t.Run("ErrorArgReferencedAsPercV", func(t *testing.T) {
		// When an error arg is formatted with %v rather than %w, the
		// implementation still detects it as an error (by type) and
		// wraps it individually. The format position is substituted with
		// a [0] placeholder rather than the error's inline text.
		inner := errors.New("inner error")
		err := Errorf("context: %v", inner)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(err, inner) {
			t.Fatal("expected err to wrap inner via errors.Is")
		}
		msg := err.Error()
		if !strings.Contains(msg, "[0]") {
			t.Fatalf("expected [0] placeholder in message, got: %q", msg)
		}
		// The raw error message should NOT appear inline in the prefix;
		// it is accessible only through the wrapped entry.
		if strings.Contains(msg, "context: inner error") {
			t.Fatalf("did not expect inline error text in prefix, got: %q", msg)
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		err1 := errors.New("first")
		err2 := errors.New("second")
		tmpl := "got %w and %w"
		err := Errorf(tmpl, err1, err2)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(err, err1) {
			t.Fatal("expected err to contain err1")
		}
		if !errors.Is(err, err2) {
			t.Fatal("expected err to contain err2")
		}
		msg := err.Error()
		if !strings.Contains(msg, "[0]") {
			t.Fatalf("expected [0] placeholder in message, got: %q", msg)
		}
		if !strings.Contains(msg, "[1]") {
			t.Fatalf("expected [1] placeholder in message, got: %q", msg)
		}
	})
}
