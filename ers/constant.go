package ers

import "github.com/tychoish/fun/internal"

// Error is a type alias for building/declaring sentinel errors
// as constants.
type Error string

// Error implements the error interface for ConstError.
func (e Error) Error() string { return string(e) }

func (e Error) Is(err error) bool {
	switch {
	case err == nil && e == "":
		return e == ""
	case err == nil || e == "":
		return false
	case internal.IsType[Error](err):
		return err == e
	default:
		return err.Error() == string(e)
	}
}

func (e Error) As(out any) bool {
	switch val := out.(type) {
	case *Error:
		*val = e
		return true
	case *string:
		*val = string(e)
		return true
	}
	return false
}
