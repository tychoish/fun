// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"fmt"
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
type Collector struct {
	mu   sync.Mutex
	list list
}

// Join takes a slice of errors and converts it into an *erc.List
// typed error. This operation has several advantages relative to
// using errors.Join(): if you call erc.Join repeatedly on the same
// error set of errors the resulting error is convertable
func Join(errs ...error) error { st := &Collector{}; st.Add(errs...); return st.Resolve() }

// Push collects an error if that error is non-nil.
func (ec *Collector) Push(err error) {
	if err != nil {
		defer internal.With(internal.Lock(&ec.mu))
		ec.list.Push(err)
	}
}

func (ec *Collector) Add(errs ...error) {
	switch len(errs) {
	case 0:
		return
	case 1:
		ec.Push(errs[0])
	default:
		defer internal.With(internal.Lock(&ec.mu))
		ec.list.Add(errs...)
	}

}

// Generator returns an iterator over the errors in the collector.
func (ec *Collector) Generator() iter.Seq[error] {
	return func(yield func(err error) bool) {
		ec.mu.Lock()
		for err := range ec.list.LIFO() {
			ec.mu.Unlock()
			if !yield(err) {
				return
			}
			ec.mu.Lock()
		}
		ec.mu.Unlock()
	}

}

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.List, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, Len is not
// updated beyond the current level. In this way Len really reports
// "height," but this is the same for the top level.
func (ec *Collector) Len() int { defer internal.With(internal.Lock(&ec.mu)); return ec.list.Len() }

// Resolve returns an error of type *erc.List, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// ers.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	defer internal.With(internal.Lock(&ec.mu))

	if ec.list.Len() == 0 {
		return nil
	}

	return ec.list.Resolve()
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool { return ec.Len() != 0 }

// Ok returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) Ok() bool { return ec.Len() == 0 }

///////////////////////////////////////////////////////////////////////
//
// Collector Helpers
//
///////////////////////////////////////////////////////////////////////

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
		defer internal.With(internal.Lock(&ec.mu))
		ec.list.Push(err)
		if hook != nil {
			hook()
		}
	}
}

///////////////////////////////////////////////////////////////////////
//
// Function Helpers
//
///////////////////////////////////////////////////////////////////////

// Wrap produces a wrapped error if the err is non-nil, wrapping the
// error with the provided annotation. When the error is nil, Wrap
// returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrap" but
// taking advantage of newer standard library error wrapping.
func Wrap(err error, annotation ...any) error {
	if ers.IsOk(err) {
		return nil
	}

	return Join(err, ers.New(fmt.Sprint(annotation...)))
}

// Wrapf produces a wrapped error, if the error is non-nil, with a
// formated wrap annotation. When the error is nil, Wrapf does not
// build an error and returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrapf" but
// taking advantage of newer standard library error wrapping.
func Wrapf(err error, tmpl string, args ...any) error {
	if ers.IsOk(err) {
		return nil
	}
	return Join(err, fmt.Errorf(tmpl, args...))
}
