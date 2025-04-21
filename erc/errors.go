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
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/internal"
)

// Collector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors which can be resolved as a single error. The
// constituent errors (and flattened, in the case of wrapped errors),
// are an *erc.Stack object, which can be introspected as needed.
type Collector struct {
	mu   sync.Mutex
	list list
}

// Add collects an error if that error is non-nil.
func (ec *Collector) Add(err error) {
	if err != nil {
		defer internal.With(internal.Lock(&ec.mu))
		ec.list.Push(err)
	}
}

// AddWithHook adds a non-nill error and runs the provided hook
// function.
func (ec *Collector) AddWithHook(err error, hook func()) {
	if err != nil {
		defer internal.With(internal.Lock(&ec.mu))
		ec.list.Push(err)
		if hook != nil {
			hook()
		}
	}
}

// Handler returns the collector's Add method as a
// fn.Handler[error] object for integration and use with the
// function types.
func (ec *Collector) Handler() fn.Handler[error] { return ec.Add }

// Future returns a function that is generally equivalent to
// Collector.Resolve(); however, the errors are returned as an unwound
// slice of errors, rather than the erc.Stack object.
func (ec *Collector) Future() fn.Future[error] { return ec.Resolve }

// Future returns a function that is generally equivalent to
// Collector.Resolve(); however, the errors are returned as an unwound
// slice of errors, rather than the erc.Stack object.
func (ec *Collector) Generator() iter.Seq[error] { return ec.list.FIFO() }

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.Stack, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, Len is not
// updated beyond the current level. In this way Len really reports
// "height," but this is the same for the top level.
func (ec *Collector) Len() int { defer internal.With(internal.Lock(&ec.mu)); return ec.list.Len() }

// Resolve returns an error of type *erc.Stack, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// ers.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	defer internal.With(internal.Lock(&ec.mu))

	if ec.list.Len() == 0 {
		return nil
	}

	return &ec.list
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool { return ec.Len() != 0 }

// Ok returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) Ok() bool { return ec.Len() == 0 }

///////////////////////////////////////////////////////////////////////
//
// Helpers
//
///////////////////////////////////////////////////////////////////////

// When is a helper function, typically useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func (ec *Collector) When(cond bool, val any) { ec.Add(ers.When(cond, val)) }

// Whenf conditionally creates and adds an error to the collector, as
// When, and with a similar use case, but permits Sprintf/Errorf
// formating.
func (ec *Collector) Whenf(cond bool, val string, args ...any) { ec.Add(ers.Whenf(cond, val, args...)) }

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func (ec *Collector) Check(fut fn.Future[error]) { ec.Add(fut.Resolve()) }

// WithRecover calls the provided function, collecting any
func (ec *Collector) WithRecover(fn func()) { ec.Recover(); defer ec.Recover(); fn() }

// Recover can be used in a defer to collect a panic and add it to the collector.
func (ec *Collector) Recover() { ec.Add(ers.ParsePanic(recover())) }

///////////////////////////////////////////////////////////////////////
//
// Panic helpers
//
///////////////////////////////////////////////////////////////////////

// RecoverHook runs adds the output of recover() to the error
// collector, and runs the specified hook if. If there was no panic,
// this function is a noop. Run RecoverHook in defer statements.
func RecoverHook(ec *Collector, hook func()) { ec.AddWithHook(ers.ParsePanic(recover()), hook) }
