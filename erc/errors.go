// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"iter"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Collector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors which can be resolved as a single error. The
// constituent errors (and flattened, in the case of wrapped errors),
// are an *erc.List object, which can be introspected as needed.
//
// Internally Collectors use a linked-list implementation to avoid
// pre-allocation concerns and limitations.
type Collector struct {
	mu   sync.Mutex
	list list
}

// AsCollector takes an error and converts it to a list if possible, if
// the error is an erc.List then this is a passthrough, and errors
// that implement {Unwind() []error} or {Unwrap() []error}, though
// preferring Unwind, are added individually to the list.
//
// For errors that provide the Unwind/Unwrap method, if these methods
// return empty slices of errors, then AsCollector will return nil.
func AsCollector(err error) *Collector {
	switch et := err.(type) {
	case nil:
		return nil
	case *Collector:
		return et
	case *list:
		return &Collector{list: *et}
	case interface{ Unwind() []error }:
		if errs := et.Unwind(); len(errs) > 0 {
			st := &Collector{}
			st.Add(errs...)
			return st
		}
		return nil
	case interface{ Unwrap() []error }:
		if errs := et.Unwrap(); len(errs) > 0 {
			st := &Collector{}
			st.Add(errs...)
			return st
		}
		return nil
	default:
		st := &Collector{}
		st.Push(err)
		return st
	}
}

///////////////////////////////////////////////////////////////////////
//
// Collector constructors and setup methods
//
///////////////////////////////////////////////////////////////////////

// Join takes a slice of errors and converts it into an *erc.List
// typed error. This operation has several advantages relative to
// using errors.Join(): if you call erc.Join repeatedly on the same
// error set of errors the resulting error is convertable
func Join(errs ...error) error { st := &Collector{}; st.Add(errs...); return st.Err() }

// with/lock are internal helpers to avoid twiddling the pointer to
// the mutex.
func (*Collector) with(m *sync.Mutex)   { internal.With(m) }
func (ec *Collector) lock() *sync.Mutex { return internal.Lock(&ec.mu) }

// SetFilter sets (or overrides) the current filter on the
// collector. Errors errors collected by the filter are passed to the
// filter function. Filters can observe nil errors, as in the case of
// the Add()
func (ec *Collector) SetFilter(erf Filter) {
	defer ec.with(ec.lock())
	ec.list.filter = erf
}

///////////////////////////////////////////////////////////////////////
//
// Collector error rendering and introspection methods
//
///////////////////////////////////////////////////////////////////////

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.List, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, Len is not
// updated beyond the current level. In this way Len really reports
// "height," but this is the same for the top level.
func (ec *Collector) Len() int { defer ec.with(ec.lock()); return ec.list.Len() }

// Resolve returns an error of type *erc.Collector, or nil if there
// have been no errors added. As a special case, single errors are
// unwrapped and returned directly.
func (ec *Collector) Resolve() error { return ec.Err() }

// Err returns the contents of the collector, as an error. Provides
// the same functionality as Resolve(). The underlying error is of
// type *erc.Collector, or nil if there have been no errors added. As
// a special case, single errors are unwrapped and returned directly.
func (ec *Collector) Err() error {
	defer ec.with(ec.lock())
	switch ec.list.Len() {
	case 0:
		return nil
	case 1:
		return ec.list.Front().Err()
	default:
		return ec
	}
}

// Ok returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) Ok() bool { return ec.Len() == 0 }

// Error implements the error interface, and renders an error message
// that includes all of the constituent errors.
func (ec *Collector) Error() string { defer ec.with(ec.lock()); return ec.list.Error() }

// Unwrap returns all of the constituent errors held by the
// collector. The implementation of errors.Is and errors.As mean that
// this method is not called for either of those functions, you can
// use this director or with ers.Unwind() to get all errors in a
// slice.
//
// Internally collectors use a linked list implementation, so Unwrap()
// requires building the slice.
func (ec *Collector) Unwrap() []error { defer ec.with(ec.lock()); return ec.list.Unwind() }

// Is supports the errors.Is() function and returns true if any of the
// errors in the collector OR their ancestors are the target error.
func (ec *Collector) Is(target error) bool { defer ec.with(ec.lock()); return ec.list.Is(target) }

// As supports the errors.As() operation to access
func (ec *Collector) As(target any) bool { defer ec.with(ec.lock()); return ec.list.As(target) }

// Iterator returns an iterator over the errors in the
// collector. The oldest or earliest errors collected appear first in
// the iteration order.
func (ec *Collector) Iterator() iter.Seq[error] {
	return func(yield func(err error) bool) {
		ec.mu.Lock()
		for err := range ec.list.FIFO() {
			ec.mu.Unlock()
			if !yield(err) {
				return
			}
			ec.mu.Lock()
		}
		ec.mu.Unlock()
	}

}

///////////////////////////////////////////////////////////////////////
//
// Collection methods
//
///////////////////////////////////////////////////////////////////////

// Push collects an error if that error is non-nil.
func (ec *Collector) Push(err error) {
	if ers.IsError(err) {
		defer ec.with(ec.lock())
		ec.list.Push(err)
	}
}

// Add appends one or more errors to the collectors. Nil errors are
// always omitted from the collector.
func (ec *Collector) Add(errs ...error) {
	switch len(errs) {
	case 0:
		return
	case 1:
		ec.Push(errs[0])
	default:
		defer ec.with(ec.lock())
		ec.list.Add(errs)
	}

}

// When is a helper function, typically useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func (ec *Collector) When(cond bool, val error) { ec.Push(ers.When(cond, val)) }

// Whenf conditionally creates and adds an error to the collector, as
// When, and with a similar use case, but permits Sprintf/Errorf
// formating.
func (ec *Collector) Whenf(cond bool, val string, args ...any) {
	ec.Push(ers.Whenf(cond, val, args...))
}

// Wrap adds an annotated error to the Collector, IF the wrapped
// error is non-nil: nil errors are always a noop. If there is no
// annotation, the error is added directly. Errors are annotated using
// fmt.Errorf and the %w token.
func (ec *Collector) Wrap(err error, annotation string) {
	ec.Push(ers.Wrap(err, annotation))
}

// Wrapf adds an annotated error to the collector IF the error is
// non-nil: nil errors make Wrapf a noop. Errors are annotated using
// fmt.Errorf and the %w token.
func (ec *Collector) Wrapf(err error, tmpl string, args ...any) {
	ec.Push(ers.Wrapf(err, tmpl, args...))
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func (ec *Collector) Check(fut func() error) { ec.Push(fut()) }

// Recover can be used in a defer to collect a panic and add it to the collector.
func (ec *Collector) Recover() { ec.Push(ParsePanic(recover())) }

// WithRecover calls the provided function, collecting any
func (ec *Collector) WithRecover(fn func()) { ec.Recover(); defer ec.Recover(); fn() }

// WithRecoverHook catches a panic and adds it to the error collector
// and THEN runs the specified hook if. If there was no panic, this
// function is a noop, and the hook never executes. Nil hooks are also
// a noop. Run WithRecoverHook in defer statements.
func (ec *Collector) WithRecoverHook(hook func()) {
	defer ec.Recover()
	if err := ParsePanic(recover()); err != nil {
		defer ec.with(ec.lock())
		ec.list.Push(err)
		if hook != nil {
			hook()
		}
	}
}
