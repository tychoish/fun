// Package intish provides a collection of strongly type integer
// arithmetic operations, to make it possible to avoid floating point
// math for simple operations when desired.
package intish

import (
	"fmt"
)

// Numbers are the set of singed and unsinged integers, used by this package.
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
		in = in * -1
	}
	return in
}

// Min returns the lowest value.
func Min[T Numbers](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func AbsMax[T Signed](a, b T) T { return Max(Abs(a), Abs(b)) }
func AbsMin[T Signed](a, b T) T { return Min(Abs(a), Abs(b)) }

// Max returns the highest value.
func Max[T Numbers](a, b T) T {
	if a > b {
		return a
	}
	return b
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
	max := AbsMax(a, b)

	return (max + multiple - (max % multiple)) * roundedSign(multiple, a, b)
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
	max := AbsMax(a, b)

	return (max - (max % multiple)) * roundedSign(multiple, a, b)

}

// RoundToSmallestMultiple rounds to smaller numbers,
// The argument with the smaller absolute value is always the
// "multiple" and the "larger" is always the value that is rounded.
//
// The rounded value is always *smaller* than the input value.
func RoundToSmallestMultiple[T Signed](a, b T) T {
	return Min(RoundToMultipleTowardZero(a, b), RoundToMultipleAwayFromZero(a, b))
}

// The argument with the smaller absolute value is always the
// "multiple" and the "larger" is always the value that is rounded.
//
// The output value is always *larget* than the input value.
func RoundToLargestMultiple[T Signed](a, b T) T {
	return Max(RoundToMultipleTowardZero(a, b), RoundToMultipleAwayFromZero(a, b))
}

// Diff returns the absolute value of the difference between two values.
func Diff[T Numbers](a, b T) T {
	return Max(a, b) - Min(a, b)
}

// Millis converts a float into a an integer that represents one
// thousandth of the units of the original.
//
// Millis will panic in the case of an overflow (wraparound).
func Millis(in float64) int64 {
	milli := int64(in * 1000)
	// check if we overflowed.
	if (in < 0) != (milli < 0) {
		panic(fmt.Sprintf("%.2f cannot be converted to millis, avoiding overflow", in))
	}

	return milli
}

// FloatMillis reverses, though potentially (often) not without some
// loss of fidelity
func FloatMillis(in int64) float64 { return float64(in) / 1000 }
