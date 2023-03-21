package internal

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

func TestMerge(t *testing.T) {
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
}
