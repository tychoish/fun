package ers

import (
	"bytes"
	"errors"

	"github.com/tychoish/fun/internal"
)

// Stack represents the error type returned by an ErrorCollector
// when it has more than one error. The implementation provides
// support for errors.Unwrap and errors.Is, and provides an Errors()
// method which returns a slice of the constituent errors for
// additional use.
type Stack struct {
	count int
	next  *Stack
	err   error
}

// Join takes a slice of errors and converts it into an *erc.Stack
// typed error.
func Join(errs ...error) error {
	s := &Stack{}
	s.append(errs...)

	switch s.count {
	case 0:
		return nil
	case 1:
		return s.err
	default:
		return s
	}
}

// Len returns the depth of the stack beneath the current level. This
// value isn't updated as additional errors are pushed onto the stack.
func (e *Stack) Len() int {
	if e == nil {
		return 0
	}
	return e.count
}

// Push adds an error to the current stack. If one stack is pushed
// onto another, the first stack's errors are unwound and pushed
// individually to the stack. If the error object implements the
// {Unwind() []error}, or {Unwrap() []error} interfaces, then these
// are also pushed individually to the stack, all other errors are
// added individually to the stack. The Stack.Is() and Stack.As
// implementations support identfying all errors included in the stack
// even if one of the individual errors is itself wrapped (e.g. one
// wrapped using fmt.Errorf and %w.)
//
// The stack object is not safe for concurrent access (use the error
// collector in the `erc` package for a higher level interface to the
// Stack object that includes a mutex,) but, Stack objects are
// implemented as a linked list and not modified after use, which
// may make them easier to reason about.
func (e *Stack) Push(err error) {
	switch werr := err.(type) {
	case nil:
		return
	case *Stack:
		// do stack separately, so we don't build the list and
		// can merge more effectively (we do throw away the
		// Stack wrapper objects for consistency with counts.)
		for werr != nil {
			e.Push(werr.err)
			werr = werr.next
		}
	case interface{ Unwind() []error }:
		// unwind over unwrap, to avoid
		for _, err := range werr.Unwind() {
			e.Push(err)
		}
	case interface{ Unwrap() []error }:
		for _, err := range werr.Unwrap() {
			e.Push(err)
		}
	default:
		// everything else includes normal unwrapped errors
		// and singly wrapped errors (e.g. with fmt.Errorf and
		// %w).
		e.next = &Stack{next: e.next, err: e.err}
		e.err = err
		e.count++
	}

}

func (e *Stack) append(errs ...error) {
	for _, err := range errs {
		e.Push(err)
	}
}

// Error produces the aggregated error strings from this method. If
// the error at the current layer is nil.
func (e *Stack) Error() string {
	if e.err == nil && e.next == nil {
		return "<nil>"
	}

	// TODO: pool buffers.
	buf := &bytes.Buffer{}

	prod := e.CheckProducer()

	for err, ok := prod(); ok; err, ok = prod() {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}

		buf.WriteString(err.Error())
	}

	return buf.String()
}

// Is calls errors.Is on the underlying error to provied compatibility
// with errors.Is, which takes advantage of this interface.
func (e *Stack) Is(err error) bool { return errors.Is(e.err, err) }

// As calls errors.As on the underlying error to provied compatibility
// with errors.As, which takes advantage of this interface.
func (e *Stack) As(target any) bool { return errors.As(e.err, target) }

// Unwrap returns the next iterator in the stack, and is compatible
// with errors.Unwrap. The error objects returned by unwrap are all
// Stack objects. When the
func (e *Stack) Unwrap() error {
	if e.next == nil || e.next.err == nil {
		return nil
	}
	return e.next
}

// Unwind returns a slice of the errors included in the Stack directly
// without wrapping them as Stacks. The ers.Unwind function *does*
// take advantage of this interface, although, in practice special
// cases the Stack type.
func (e *Stack) Unwind() []error {
	out := make([]error, 0, e.count)
	iter := &Stack{next: e}

	for {
		if iter.next == nil || iter.next.err == nil {
			break
		}
		iter = iter.next
		out = append(out, iter.err)
	}

	return out
}

// CheckProducer provides a pull-based iterator function for iterating
// through the errors (without Stack object wrappers,)
func (e *Stack) CheckProducer() func() (error, bool) {
	iter := &Stack{next: e}
	return func() (error, bool) {
		if iter.next == nil || iter.next.err == nil {
			return nil, false
		}

		iter = iter.next
		return iter.err, true
	}
}

// Unwind, is a special case of the fun.Unwind operation, that
// assembles the full "unwrapped" list of all component
// errors. Supports error implementations where the Unwrap() method
// returns either error or []error.
//
// If an error type implements interface{ Unwind() []error }, this
// takes precedence over Unwrap when unwinding errors, to better
// support the Stack type and others where the original error is
// nested within the unwrapped error objects.
func Unwind(in error) []error { return internal.Unwind(in) }

// Strings renders (using the Error() method) a slice of errors into a
// slice of their string values.
func Strings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for idx := range errs {
		if !OK(errs[idx]) {
			out = append(out, errs[idx].Error())
		}
	}

	return out
}
