package internal

import (
	"testing"
)

func TestPtr(t *testing.T) {
	var nptr *string
	var err error

	if !IsNil(nptr) {
		t.Error("should be nil")
	}
	if !IsNil(err) {
		t.Error("should be nil")
	}
	if IsNil(4) {
		t.Error("value can't be nil")
	}
	if IsNil(t) {
		t.Error("test value is always non-nil")
	}

	var anyif any
	if !IsNil(anyif) {
		t.Error("test is not nil")
	}

	// interfaces holding nil values are not nil..
	// and therefore:
	if anyif != nil {
		t.Error("unexpected comparison result")
	}
	anyif = nptr
	if anyif == nil { //nolint:staticcheck
		t.Error("unexpected comparison result")
	}
	// however...
	if !IsNil(anyif) {
		t.Error("helper should behave expectedly")
	}

	var dptr *string
	if !IsPtr(dptr) {
		t.Error("unexpected")
	}
	if IsPtr(3) {
		t.Error("unexpected pointer check")
	}
}
