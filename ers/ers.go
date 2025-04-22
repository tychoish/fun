package ers

import (
	"context"
	"fmt"
	"io"
)

// When constructs an ers.Error-typed error value IF the conditional
// is true, and returns nil otherwise.
func When(cond bool, err error) error {
	if !cond {
		return nil
	}

	return err
}

// Whenf constructs an error (using fmt.Errorf) IF the conditional is
// true, and returns nil otherwise.
func Whenf(cond bool, tmpl string, args ...any) error {
	if !cond {
		return nil
	}

	return fmt.Errorf(tmpl, args...)
}

// IsError returns true when the error is non-nill. Provides the
// inverse of Ok().
func IsError(err error) bool { return !IsOk(err) }

// IsOk returns true when the error is nil, and false otherwise. It
// should always be inlined, and mostly exists for clarity at call
// sites in bool/IsOk check relevant contexts.
func IsOk(err error) bool {
	switch e := err.(type) {
	case nil:
		return true
	case interface{ Ok() bool }:
		return e.Ok()
	default:
		return false
	}
}

// IsExpiredContext checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func IsExpiredContext(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration, or
// io.EOF.)
func IsTerminating(err error) bool { return Is(err, io.EOF, ErrCurrentOpAbort, ErrContainerClosed) }
