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

// AsStack takes an error and converts it to a stack if possible, if
// the error is an ers.Stack then this is a passthrough, and errors
// that implement {Unwind() []error} or {Unwrap() []error}, though
// preferring Unwind, are added individually to the stack.
//
// For errors that provide the Unwind/Unwrap method, if these methods
// return empty slices of errors, then AsStack will return nil.
func AsStack(err error) *Stack {
	switch et := err.(type) {
	case nil:
		break
	case *Stack:
		return et
	case interface{ Unwind() []error }:
		if errs := et.Unwind(); len(errs) > 0 {
			st := &Stack{}
			st.Add(errs...)
			return st
		}
	case interface{ Unwrap() []error }:
		if errs := et.Unwrap(); len(errs) > 0 {
			st := &Stack{}
			st.Add(errs...)
			return st
		}
	default:
		st := &Stack{}
		st.Push(err)
		return st
	}

	// there's a nil error if
	return nil
}

// Join takes a slice of errors and converts it into an *erc.Stack
// typed error. This operation has several advantages relative to
// using errors.Join(): if you call ers.Join repeatedly on the same
// error set of errors the resulting error is convertable
func Join(errs ...error) error { st := Stack{}; st.Add(errs...); return st.Resolve() }

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
		// unwind over unwrap, given that Unwind is our
		// interface and unwrap is the stdlib and some folks
		// may implement both with different semantics.
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

// Handler provides a fun.Handler[error] typed function (though
// because ers is upstream of the root-fun package, it is not
// explicitly typed as such.) which will Add errors to the stack.
func (e *Stack) Handler() func(err error) { return e.Push }

// Future provides a fun.Future[error] typed function (though
// because ers is upstream of the root-fun package, it is not
// explicitly typed as such.) which will resolve the stack.
func (e *Stack) Future() func() error { return e.Resolve }

// Resolve, mirroring the interface of erc.Collector, returns the
// error (always a Stack object containing the aggregate errors,) in
// the case that stack object contains errors, and nil otherwise.
func (e *Stack) Resolve() error {
	switch {
	case e == nil || e.count == 0:
		return nil
	case e.count == 1:
		return e.err
	default:
		return e
	}
}

// Ok returns true if the Stack object contains no errors and false
// otherwise.
func (e *Stack) Ok() bool { return e == nil || (e.err == nil && e.next == nil) }

// Add is a thin wrapper around Push, that adds each error supplied as
// an argument individually to the stack.
//
// This leads to a curios, semantic: using the Unwrap() method (and
// casting; but not the unwind method!), the values returned for each
// layer are also *ers.Stack values: if you were to call Add any of
// these objects, you would end up with sort of tree-like assortment
// of objects which may yield surprising result. At the same time, if
// you have a reference to one of these stack objects, future calls to
// Add() on the "outer" stack will have no bearing on "inner" Stack
// objects.
func (e *Stack) Add(errs ...error) {
	for _, err := range errs {
		e.Push(err)
	}
}

// Error produces the aggregated error strings from this method. If
// the error at the current layer is nil.
func (e *Stack) Error() string {
	if e.Ok() {
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
// through the errors (without Stack object wrappers.)  The output
// function yields errors: the boolean return
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
		if !Ok(errs[idx]) {
			out = append(out, errs[idx].Error())
		}
	}

	return out
}
