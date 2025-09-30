package cmp

import (
	"testing"
	"time"
)

type userOrderable struct {
	val int
}

func (u userOrderable) LessThan(in userOrderable) bool { return u.val < in.val }

func TestCmp(t *testing.T) {
	t.Run("Native", func(t *testing.T) {
		for idx, b := range []bool{
			LessThanNative(1, 2),
			LessThanNative(0, 40),
			LessThanNative(-1, 0),
			LessThanNative(1.5, 1.9),
			LessThanNative(440, 9001),
			LessThanNative("abc", "abcd"),
		} {
			if !b {
				t.Error(idx, "expected true")
			}
		}

		for idx, b := range []bool{
			LessThanNative(0, -2),
			LessThanNative(2.1, 1.9),
			LessThanNative(999440, 9001),
			LessThanNative("zzzz", "aaa"),
		} {
			if b {
				t.Error(idx, "expected false")
			}
		}
	})
	t.Run("Reversed", func(t *testing.T) {
		for idx, b := range []bool{
			Reverse(LessThanNative[int])(1, 2),
			Reverse(LessThanNative[int])(0, 40),
			Reverse(LessThanNative[int])(-1, 0),
			Reverse(LessThanNative[float64])(1.5, 1.9),
			Reverse(LessThanNative[uint])(440, 9001),
			Reverse(LessThanNative[string])("abc", "abcd"),
		} {
			if b {
				t.Error(idx, "expected not true (false)")
			}
		}

		for idx, b := range []bool{
			Reverse(LessThanNative[int8])(0, -2),
			Reverse(LessThanNative[float32])(2.1, 1.9),
			Reverse(LessThanNative[uint64])(999440, 9001),
			Reverse(LessThanNative[string])("zzzz", "aaa"),
		} {
			if !b {
				t.Error(idx, "expected not false (true)")
			}
		}
	})
	t.Run("Time", func(t *testing.T) {
		if LessThanTime(time.Now(), time.Now().Add(-time.Hour)) {
			t.Error("the past should not be before the future")
		}
		if LessThanTime(time.Now().Add(365*24*time.Hour), time.Now()) {
			t.Error("the future should be after the present")
		}
	})
	t.Run("Custom", func(t *testing.T) {
		if !LessThanCustom(userOrderable{1}, userOrderable{199}) {
			t.Error("custom error")
		}
		if LessThanCustom(userOrderable{1000}, userOrderable{199}) {
			t.Error("custom error")
		}
	})
	t.Run("Function", func(t *testing.T) {
		anyCmp := LessThanConverter(func(in any) int { return in.(int) })

		if !anyCmp(any(1), any(900)) {
			t.Error("custom error")
		}
		if anyCmp(any(1000), any(199)) {
			t.Error("custom error")
		}
	})
}
