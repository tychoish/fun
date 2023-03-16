// GENERATED FILE FROM ASSERTION PACKAGE
package check

import (
	"errors"
	"strings"
	"testing"
)

// True causes a test to fail if the condition is false.
func True(t testing.TB, cond bool) {
	t.Helper()
	if !cond {
		t.Error("assertion failure")
	}
}

// Equal causes a test to fail if the two (comparable) values are not
// equal. Be aware that two different pointers and objects passed as
// interfaces that are implemented by pointer receivers are comparable
// as equal and will fail this assertion even if their *values* are
// equal.
func Equal[T comparable](t testing.TB, valOne, valTwo T) {
	t.Helper()
	if valOne != valTwo {
		t.Errorf("values unequal: <%v> != <%v>", valOne, valTwo)
	}
}

// NotEqual causes a test to fail if two (comparable) values are not
// equal. Be aware that pointers to objects (including objects passed
// as interfaces implemented by pointers) will pass this test, even if
// their values are equal.
func NotEqual[T comparable](t testing.TB, valOne, valTwo T) {
	t.Helper()
	if valOne == valTwo {
		t.Errorf("values equal: <%v>", valOne)
	}
}

func zeroOf[T any]() T { return *new(T) }

// Zero fails a test if the value is not the zero-value for its type.
func Zero[T comparable](t testing.TB, val T) {
	t.Helper()
	if zeroOf[T]() != val {
		t.Errorf("expected zero for value of type %T <%v>", val, val)
	}
}

// NotZero fails a test if the value is the zero for its type.
func NotZero[T comparable](t testing.TB, val T) {
	t.Helper()
	if zeroOf[T]() == val {
		t.Errorf("expected non-zero for value of type %T", val)
	}
}

// Error fails the test if the error is nil.
func Error(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected non-nil error")
	}
}

// NotError fails the test if the error is non-nil.
func NotError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Error(err)
	}
}

// ErrorIs is an assertion form of errors.Is, and fails the test if
// the error (or its wrapped values) are not equal to the target
// error.
func ErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("error <%v>, is not <%v>", err, target)
	}
}

// NotErrorIs is an assertion form of !errors.Is, and fails the test if
// the error (or its wrapped values) are  equal to the target
// error.
func NotErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if errors.Is(err, target) {
		t.Errorf("error <%v>, is <%v>", err, target)
	}
}

// Panic asserts that the function raises a panic.
func Panic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic but got none")
		}
	}()
	fn()
}

// NotPanic asserts that the function does not panic.
func NotPanic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Error("panic: ", r)
		}
	}()
	fn()
}

// PanicValue asserts that the function raises a panic and that the
// value, as returned by recover() is equal to the value provided.
func PanicValue[T comparable](t testing.TB, fn func(), value T) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected a panic but got none")
		}
		pval, ok := r.(T)
		if !ok {
			t.Errorf("panic [%v], not of expected type %T", r, zeroOf[T]())
		}
		Equal(t, pval, value)
	}()

	fn()
}

// Contains asserts that the item is in the slice provided. Empty or
// nil slices always cause failure.
func Contains[T comparable](t testing.TB, slice []T, item T) {
	t.Helper()
	if len(slice) == 0 {
		t.Error("slice was empty")
	}

	for _, it := range slice {
		if it == item {
			return
		}
	}

	t.Errorf("item <%v> is not in %v", item, slice)
}

// Contains asserts that the item is *not* in the slice provided. If
// the input slice is empty, this assertion will never error.
func NotContains[T comparable](t testing.TB, slice []T, item T) {
	t.Helper()

	for _, it := range slice {
		if it == item {
			t.Errorf("item <%v> is in %v", item, slice)
		}
	}
}

// Substring asserts that the substring is present in the string.
func Substring(t testing.TB, str, substr string) {
	t.Helper()
	if !strings.Contains(str, substr) {
		t.Errorf("expected %q to contain substring %q", str, substr)
	}
}

// NotSubstring asserts that the substring is not present in the
// outer string.
func NotSubstring(t testing.TB, str, substr string) {
	t.Helper()
	if strings.Contains(str, substr) {
		t.Errorf("expected %q to contain substring %q", str, substr)
	}
}

// Failing asserts that the specified test fails. This was required
// for validating the behavior of the assertion, and may be useful in
// your own testing.
func Failing[T testing.TB](t T, test func(T)) {
	t.Helper()
	sig := make(chan bool)
	go func() {
		defer close(sig)

		var tt testing.TB

		switch testing.TB(t).(type) {
		case *testing.T:
			tt = &testing.T{}
		case *testing.B:
			tt = &testing.B{}
		}

		test(tt.(T))

		if !tt.Failed() {
			sig <- true
		}
	}()

	if <-sig {
		t.Errorf("expected test to fail in %s", t.Name())
	}
}
