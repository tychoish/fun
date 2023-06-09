package ers

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Wrap produces a wrapped error if the err is non-nil, wrapping the
// error with the provided annotation. When the error is nil, Wrap
// returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrap" but
// taking advantage of newer standard library error wrapping.
func Wrap(err error, annotation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", annotation, err)
}

// Wrapf produces a wrapped error, if the error is non-nil, with a
// formated wrap annotation. When the error is nil, Wrapf does not
// build an error and returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrapf" but
// taking advantage of newer standard library error wrapping.
func Wrapf(err error, tmpl string, args ...any) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", fmt.Sprintf(tmpl, args...), err)
}

// ContextExpired checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func ContextExpired(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

func Ok(err error) bool { return err == nil }

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration a)
func IsTerminating(err error) bool {
	return Is(err, io.EOF, context.Canceled, context.DeadlineExceeded)
}

// Is returns true if the error is one of the target errors, (or one
// of it's constituent (wrapped) errors is a target error. ers.Is uses
// errors.Is.
func Is(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// Filter takes an error and returns nil if the error is nil, or if
// the error (or one of its wrapped errors,) is in the exclusion list.
func Filter(err error, exclusions ...error) error {
	if Ok(err) || Is(err, exclusions...) {
		return nil
	}
	return err
}
