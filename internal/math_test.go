package internal

import "testing"

func TestMath(t *testing.T) {
	if v := Abs(-100); v != 100 {
		t.Error(v)
	}
	if v := Abs(100); v != 100 {
		t.Error(v)
	}

	if v := Max(0, 100); v != 100 {
		t.Error(v)
	}
	if v := Max(-10, 100); v != 100 {
		t.Error(v)
	}

	if v := Max(100, 0); v != 100 {
		t.Error(v)
	}
	if v := Max(100, -10); v != 100 {
		t.Error(v)
	}

	if v := Min(-100, -10); v != -100 {
		t.Error(v)
	}
	if v := Min(-100, 0); v != -100 {
		t.Error(v)
	}
	if v := Min(-100, 100); v != -100 {
		t.Error(v)
	}

	if v := Min(-10, -100); v != -100 {
		t.Error(v)
	}
	if v := Min(0, -100); v != -100 {
		t.Error(v)
	}
	if v := Min(100, -100); v != -100 {
		t.Error(v)
	}

}
