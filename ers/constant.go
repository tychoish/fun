// Package ers provides some very basic error aggregating and handling
// tools, as a companion to erc.
//
// The packages are similar, though ers is smaller and has no
// dependencies outside of a few packages in the standard library,
// whereas erc is more tightly integrated into fun's ecosystem and
// programming model.
package ers

// Error is a type alias for building/declaring sentinel errors
// as constants.
//
// In addition to nil error interface values, the Empty string is,
// considered equal to nil errors for the purposes of Is(). errors.As
// correctly handles unwrapping and casting Error-typed error objects.
type Error string

func New(str string) error { return Error(str) }

// Error implements the error interface for ConstError.
func (e Error) Error() string { return string(e) }

// Satisfies the Is() interface without using reflection.
func (e Error) Is(err error) bool {
	switch {
	case err == nil && e == "":
		return e == ""
	case (err == nil) != (e == ""):
		return false
	default:
		switch x := err.(type) {
		case Error:
			return x == e
		default:
			return false
		}
	}
}
