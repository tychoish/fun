// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
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
	mu    sync.Mutex
	stack ers.Stack
}

// Add collects an error if that error is non-nil.
func (ec *Collector) Add(err error) {
	if err != nil {
		defer internal.With(internal.Lock(&ec.mu))
		ec.stack.Push(err)
	}
}

// AddWithHook adds a non-nill error and runs the provided hook
// function.
func (ec *Collector) AddWithHook(err error, hook func()) {
	if err != nil {
		defer internal.With(internal.Lock(&ec.mu))
		ec.stack.Push(err)
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
// slice of errors, rather than the ers.Stack object.
func (ec *Collector) Future() fn.Future[error] { return ec.Resolve }

// Future returns a function that is generally equivalent to
// Collector.Resolve(); however, the errors are returned as an unwound
// slice of errors, rather than the ers.Stack object.
func (ec *Collector) Generator() func() (error, bool) {
	next := ec.stack.Generator()
	return func() (error, bool) {
		defer internal.With(internal.Lock(&ec.mu))
		return next()
	}
}

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.Stack, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, Len is not
// updated beyond the current level. In this way Len really reports
// "height," but this is the same for the top level.
func (ec *Collector) Len() int { defer internal.With(internal.Lock(&ec.mu)); return ec.stack.Len() }

// Resolve returns an error of type *erc.Stack, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// ers.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	defer internal.With(internal.Lock(&ec.mu))

	if ec.stack.Len() == 0 {
		return nil
	}

	return &ec.stack
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool { return ec.Len() != 0 }

// Ok returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) Ok() bool { return ec.Len() == 0 }
