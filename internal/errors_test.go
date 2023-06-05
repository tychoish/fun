package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

func TestMerge(t *testing.T) {
	t.Run("Underlying", func(t *testing.T) {
		e1 := &errorTest{val: 100}
		e2 := &errorTest{val: 200}

		err := &MergedError{Current: e1, Wrapped: e2}

		if !errors.Is(err, e1) {
			t.Error("shold be er1", err, e1)
		}

		if !errors.Is(err, e2) {
			t.Error("shold be er2", err, e2)
		}
		cp := &errorTest{}
		if !errors.As(err, &cp) {
			t.Error("should err as", err, cp)
		}
		if cp.val != e1.val {
			t.Error(cp.val)
		}
		if !strings.Contains(err.Error(), "100") {
			t.Error(err)
		}
		if !strings.Contains(err.Error(), "200") {
			t.Error(err)
		}
		ue := errors.Unwrap(err)
		if ue != e2 {
			t.Error(ue)
		}

	})
	t.Run("MergeErrors", func(t *testing.T) {
		t.Run("Both", func(t *testing.T) {
			e1 := &errorTest{val: 100}
			e2 := &errorTest{val: 200}

			err := MergeErrors(e1, e2)

			if err == nil {
				t.Fatal("should be an error")
			}
			if !errors.Is(err, e1) {
				t.Error("shold be er1", err, e1)
			}

			if !errors.Is(err, e2) {
				t.Error("shold be er2", err, e2)
			}
			cp := &errorTest{}
			if !errors.As(err, &cp) {
				t.Error("should err as", err, cp)
			}
			if cp.val != e1.val {
				t.Error(cp.val)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := MergeErrors(e1, nil)
			if err != e1 {
				t.Error(err, e1)
			}
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := MergeErrors(nil, e1)
			if err != e1 {
				t.Error(err, e1)
			}
		})
		t.Run("Neither", func(t *testing.T) {
			err := MergeErrors(nil, nil)
			if err != nil {
				t.Error(err)
			}
		})
	})
}

func TestParsePanic(t *testing.T) {
	base := errors.New("funtime")
	t.Run("NilInput", func(t *testing.T) {
		err := ParsePanic(nil, base)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("TwoErrors", func(t *testing.T) {
		err := ParsePanic(io.EOF, base)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, io.EOF) {
			t.Error("not EOF", err)
		}
		if !errors.Is(err, base) {
			t.Error("not wrapped", err)
		}
	})
	t.Run("NotErrorObject", func(t *testing.T) {
		err := ParsePanic("EOF", base)
		if err == nil {
			t.Fatal("expected error")
		}
		if errors.Is(err, io.EOF) {
			t.Error("is EOF", err)
		}
		if !errors.Is(err, base) {
			t.Error("not wrapped", err)
		}
		if err.Error() != "EOF: funtime" {
			t.Error(err)
		}
	})
}

func TestTerminatingError(t *testing.T) {
	if err := io.EOF; !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := context.Canceled; !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := context.DeadlineExceeded; !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}

	if err := fmt.Errorf("weird: %w", io.EOF); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := fmt.Errorf("weird: %w", context.Canceled); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := fmt.Errorf("weird: %w", context.DeadlineExceeded); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}

	other := errors.New("even")
	if err := MergeErrors(other, io.EOF); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := MergeErrors(other, context.Canceled); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}
	if err := MergeErrors(other, context.DeadlineExceeded); !IsTerminatingError(err) {
		t.Error("expected terminating error:", err)
	}

	if err := other; IsTerminatingError(err) {
		t.Error("not expecting terminating error:", err)

	}
	if IsTerminatingError(nil) {
		t.Error("not expecting terminating error:", nil)
	}
}
