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

func TestErrorln_NoArgs(t *testing.T) {
	if err := Errorln(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestErrorln_OnlyNonErrorArgs(t *testing.T) {
	err := Errorln("foo", "bar", 123)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !ers.IsError(err) || err.Error() != "foo bar 123" {
		t.Errorf(`unexpected error: %T %q`, err, err.Error())
	}
}

func TestErrorln_OnlyErrorArgs(t *testing.T) {
	e1 := errors.New("err1")
	e2 := ers.Error("err2")
	err := Errorln(e1, e2)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	str := err.Error()
	if !(strings.Contains(str, "[0]") && strings.Contains(str, "[1]")) {
		t.Errorf("got %q, want both '[0]' and '[1]'", str)
	}
	if !strings.Contains(str, "err1") || !strings.Contains(str, "err2") {
		t.Errorf("missing error messages in error: %q", str)
	}
}

func TestErrorln_MixedArgs(t *testing.T) {
	e1 := errors.New("err1")
	err := Errorln("prefix", e1, "middle", 42)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	s := err.Error()
	if !strings.Contains(s, "prefix") || !strings.Contains(s, "middle") || !strings.Contains(s, "42") {
		t.Errorf("missing non-error args in error: %q", s)
	}
	if !strings.Contains(s, "[1]") || !strings.Contains(s, "err1") {
		t.Errorf("did not tag error with '[1]': %q", s)
	}
}

func TestErrorln_InterleavedArgs(t *testing.T) {
	e1 := errors.New("err1")
	e2 := errors.New("err2")
	err := Errorln("start", e1, "mid", e2, "end")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	s := err.Error()
	for _, part := range []string{"start", "mid", "end", "err1", "err2", "[1]", "[3]"} {
		if !strings.Contains(s, part) {
			t.Errorf("missing %q in %q", part, s)
		}
	}
}
