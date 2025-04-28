// Package intish provides a collection of strongly type integer
// arithmetic operations, to make it possible to avoid floating point
// math for simple operations when desired.
package intish

import (
	"fmt"
)

// Numbers are the set of singed and unsinged integers as used by this package.
type Numbers interface {
	Signed | Unsigned
}

// Signed are all of the primitive signed integer types in go.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned are all of the primitive signed integer types in go.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Abs returns the absolute value of the integer.
func Abs[T Signed](in T) T {
	if in < 0 {
		in *= -1
	}
	return in
}

// Range orders two numbers and returns the pair as (lower, higher).
func Range[T Numbers](a, b T) (start T, end T) { return min(a, b), max(a, b) }

// Bounds returns the two arguments as a (min,max): values less than
// zero become zero.
func Bounds[T Numbers](a, b T) (minVal T, maxVal T) { return max(0, min(a, b)), max(0, max(a, b)) }

// AbsBounds resolves the absolute values of two numbers and then
// return the lower (absolute) value followed by the higher absolute
// value.
func AbsBounds[T Signed](a, b T) (minVal T, maxVal T) { return AbsMin(a, b), AbsMax(a, b) }

// AbsMax resolves the absolute value of both arguments and returns
// the larger value.
func AbsMax[T Signed](a, b T) T { return max(Abs(a), Abs(b)) }

// AbsMin resolves the absolute value of both arguments and returns
// the smaller value.
func AbsMin[T Signed](a, b T) T { return min(Abs(a), Abs(b)) }

// Diff returns the absolute value of the difference between two values.
func Diff[T Numbers](a, b T) T { return max(a, b) - min(a, b) }

// FloatMillis reverses, though potentially (often) not without some
// loss of fidelity, the operation of Millis.
func FloatMillis[T Signed](in T) float64 { return float64(in) / 1000 }

// Millis converts a float into a an integer that represents one
// thousandth of the units of the original.
//
// Millis will panic in the case of an overflow (wraparound).
func Millis[T Signed](in float64) T {
	milli := T(in * 1000)
	// check if we overflowed.
	if (in < 0) != (milli < 0) {
		panic(fmt.Sprintf("%.2f cannot be converted to millis, avoiding overflow", in))
	}

	return milli
}

// RoundToMultipleAwayFromZero rounds a value to the nearest multiple
// of the other number. The argument with the smaller absolute value
// is always the "multiple" and value with larger absolute value is
// rounded.
//
// The rounded always has a higher absolute value than the input
// value.
func RoundToMultipleAwayFromZero[T Signed](a, b T) T {
	multiple := AbsMin(a, b)
	maxVal := AbsMax(a, b)

	return (maxVal + multiple - (maxVal % multiple)) * roundedSign(multiple, a, b)
}

func roundedSign[T Signed](multiple, a, b T) T {
	switch {
	case a < 0 && b < 0:
		return -1
	case a < 0 && b > 0 && Abs(b) == multiple:
		return -1
	case a > 0 && b < 0 && Abs(a) == multiple:
		return -1
	default:
		return 1
	}

}

// RoundToMultipleTowardZero rounds a value to the nearest multiple of
// the other number. The argument with the smaller absolute value is
// always the "multiple" and value with larger absolute value is
// rounded.
//
// The rounded always has a lower absolute value than the input value.
func RoundToMultipleTowardZero[T Signed](a, b T) T {
	multiple := AbsMin(a, b)
	maxVal := AbsMax(a, b)

	return (maxVal - (maxVal % multiple)) * roundedSign(multiple, a, b)
}

// RoundToSmallestMultiple rounds to smaller numbers: The argument
// with the smaller absolute value is always the "multiple" and the
// "larger" is always the value that is rounded.
//
// The rounded value is always *smaller* than the input value.
func RoundToSmallestMultiple[T Signed](a, b T) T {
	return min(RoundToMultipleTowardZero(a, b), RoundToMultipleAwayFromZero(a, b))
}

// RoundToLargestMultiple rounds up to a larger value: The argument
// with the smaller absolute value is always the "multiple" and the
// "larger" is always the value that is rounded.
//
// The output value is always *larget* than the input value.
func RoundToLargestMultiple[T Signed](a, b T) T {
	return max(RoundToMultipleTowardZero(a, b), RoundToMultipleAwayFromZero(a, b))
}
