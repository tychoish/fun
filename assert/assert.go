// Package assert provides an incredibly simple assertion framework,
// that relies on generics and simplicity. All assertions are "fatal"
// and cause the test to abort at the failure line (rather than
// continue on error).
package assert

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/intish"
)

// True causes a test to fail if the condition is false.
func True(t testing.TB, cond bool) {
	t.Helper()
	if !cond {
		t.Fatal("assertion failure")
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
		t.Fatalf("unequal: <%v> != <%v>", valOne, valTwo)
	}
}

// NotEqual causes a test to fail if two (comparable) values are not
// equal. Be aware that pointers to objects (including objects passed
// as interfaces implemented by pointers) will pass this test, even if
// their values are equal.
func NotEqual[T comparable](t testing.TB, valOne, valTwo T) {
	t.Helper()
	if valOne == valTwo {
		t.Fatalf("equal: <%v>", valOne)
	}
}

// Nil causes a test to fail if the value is not nil. This operation
// uses reflection, (unlike many in this package,) and correctly
// handles nil values assigned to interfaces (e.g. that they are nil.)
func Nil(t testing.TB, val any) {
	t.Helper()

	if _, ok := val.(error); ok {
		t.Error("use assert.NotError() for checking errors")
	}

	if !internal.IsNil(val) {
		t.Fatalf("value (type=%T), %v was expected to be nil", val, val)
	}
}

// NotNil causes a test to fail if the value is nil. This operation
// uses reflection, (unlike many in this package,) and correctly
// handles nil values assigned to interfaces (e.g. that they are nil.)
func NotNil(t testing.TB, val any) {
	t.Helper()

	if _, ok := val.(error); ok {
		t.Error("use assert.Error() for checking errors")
	}

	if internal.IsNil(val) {
		t.Fatalf("value (type=%T), was nil", val)
	}
}

// NilPtr asserts that the pointer value is nil. Use Nil (which uses
// reflection) for these pointer values as well maps, channels,
// slices, and interfaces.
func NilPtr[T any](t testing.TB, val *T) { t.Helper(); Equal(t, val, nil) }

// NotNilPtr asserts that the pointer value is not equal to nil. Use
// Nil (which uses reflection) for these pointer values as well maps,
// channels, slices, and interfaces.
func NotNilPtr[T any](t testing.TB, val *T) { t.Helper(); NotEqual(t, val, nil) }

// Zero fails a test if the value is not the zero-value for its type.
func Zero[T comparable](t testing.TB, val T) {
	t.Helper()

	var zero T
	if zero != val {
		t.Fatalf("expected zero for value of type %T <%v>", val, val)
	}
}

// NotZero fails a test if the value is the zero for its type.
func NotZero[T comparable](t testing.TB, val T) {
	t.Helper()
	var zero T
	if zero == val {
		t.Fatalf("expected non-zero for value of type %T", val)
	}
}

// Error fails the test if the error is nil.
func Error(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}

// NotError fails the test if the error is non-nil.
func NotError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Type fails the test if the type of the object doesn't match the
// specifier type provided.
func Type[T any](t testing.TB, obj any) {
	t.Helper()
	_, ok := obj.(T)
	if !ok {
		var tn T
		t.Fatalf("%s is not of type %T", obj, tn)
	}
}

// NotType fails the test when the type of the specifier matches the
// type of the object.
func NotType[T any](t testing.TB, obj any) {
	t.Helper()
	_, ok := obj.(T)
	if ok {
		var tn T
		t.Fatalf("%s is not of type %T", obj, tn)
	}
}

// ErrorIs is an assertion form of errors.Is, and fails the test if
// the error (or its wrapped values) are not equal to the target
// error.
func ErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error <%v>, is not <%v>", err, target)
	}
}

// NotErrorIs is an assertion form of !errors.Is, and fails the test if
// the error (or its wrapped values) are  equal to the target
// error.
func NotErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if errors.Is(err, target) {
		t.Fatalf("error <%v>, is <%v>", err, target)
	}
}

// Panic asserts that the function raises a panic.
func Panic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r == nil {
			t.Fatal("expected a panic but got none")
		}
	}()
	fn()
}

// NotPanic asserts that the function does not panic.
func NotPanic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if r := recover(); r != nil {
			t.Fatal("panic: ", r)
		}
	}()
	fn()
}

// PanicValue asserts that the function raises a panic and that the
// value, as returned by recover() is equal to the value provided.
func PanicValue[T comparable](t testing.TB, fn func(), value T) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Fatal("expected a panic but got none")
		}
		pval, ok := r.(T)
		if !ok {
			t.Fatalf("panic [%v], not of expected type %T", r, value)
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
		t.Fatal("slice was empty")
	}

	for _, it := range slice {
		if it == item {
			return
		}
	}

	t.Fatalf("item <%v> is not in %v", item, slice)
}

// Contains asserts that the item is *not* in the slice provided. If
// the input slice is empty, this assertion will never error.
func NotContains[T comparable](t testing.TB, slice []T, item T) {
	t.Helper()

	for _, it := range slice {
		if it == item {
			t.Fatalf("item <%v> is in %v", item, slice)
		}
	}
}

// EqualItems compares the values in two slices and creates an error
// if all items are not equal.
func EqualItems[T comparable](t testing.TB, one, two []T) {
	t.Helper()
	if len(one) != len(two) {
		t.Fatalf("slices are of different lengths [%d vs %d]", len(one), len(two))
	}

	for idx := range one {
		if one[idx] != two[idx] {
			t.Fatalf("items at index %d [%v vs %v] are not equal", idx, one[idx], two[idx])
		}
	}
}

// EqualItems compares the values in two slices and creates a failure
// if all items are not equal
func NotEqualItems[T comparable](t testing.TB, one, two []T) {
	t.Helper()

	for idx := range one {
		if one[idx] == two[idx] {
			continue
		}
		return
	}
	t.Fatal("slices do not have identical items")
}

// Substring asserts that the substring is present in the string.
func Substring(t testing.TB, str, substr string) {
	t.Helper()
	if !strings.Contains(str, substr) {
		t.Fatalf("expected %q to contain substring %q", str, substr)
	}
}

// NotSubstring asserts that the substring is not present in the
// outer string.
func NotSubstring(t testing.TB, str, substr string) {
	t.Helper()
	if strings.Contains(str, substr) {
		t.Fatalf("expected %q to contain substring %q", str, substr)
	}
}

// MaxRuntime runs an operation and asserts that the operations
// runtime was less than the provided duration.
func MaxRuntime(t testing.TB, dur time.Duration, op func()) {
	t.Helper()
	start := time.Now()
	op()
	ranFor := time.Since(start)
	if ranFor > dur {
		t.Fatalf("operation ran for %s, greater than expected %s", ranFor, dur)
	}
}

// MinRuntime runs an operation and asserts that the operations
// runtime was greater than the provided duration.
func MinRuntime(t testing.TB, dur time.Duration, op func()) {
	t.Helper()
	start := time.Now()
	op()
	ranFor := time.Since(start)
	if dur > ranFor {
		t.Fatalf("operation ran for %s, less than expected %s", ranFor, dur)
	}
}

// Runtime asserts that the function will execute for less than the
// absolute difference of the two durations provided. The absolute
// difference between the durations is use to max
func Runtime(t testing.TB, min, max time.Duration, op func()) {
	t.Helper()
	start := time.Now()
	op()
	ranFor := time.Since(start)

	if intish.Min(min, max) > ranFor || intish.Max(min, max) < ranFor {
		t.Log(intish.Min(min, max) > ranFor, "||", intish.Max(min, max) < ranFor)
		t.Fatalf("operation ran for %s which is not between %s and %s",
			ranFor, intish.Min(min, max), intish.Max(min, max),
		)
	}

}

// Failing asserts that the specified test fails. This was required
// for validating the behavior of the assertion, and may be useful in
// your own testing.
func Failing[T testing.TB](t T, test func(T)) {
	t.Helper()
	sig := make(chan bool)
	go func() {
		t.Helper()
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
		t.Fatalf("expected test to fail in %s", t.Name())
	}
}
