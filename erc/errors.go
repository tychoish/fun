// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
)

// // Producer exposes the state of the error stack as a producer
// // function, and can be used to build iterators or other operatios in
// // the fun package.
// func (e *Stack) Producer() fun.Producer[error] { return fun.CheckProducer(e.CheckProducer()) }

// // Iterator returns an iterator object over the contents of the
// // stack. While the content of the iterator is immutable and will not
// // observe new errors added to the stack, the iterator itself is not
// // safe for concurrent access, and must only be used from one
// // goroutine, or with a synchronized approach. You may create multiple
// // Iterators on the same stack without issue.
// func (e *Stack) Iterator() *fun.Iterator[error] { return e.Producer().Iterator() }

// Collector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors which can be resolved as a single error. The
// constituent errors (and flattened, in the case of wrapped errors),
// are an *erc.Stack object, which can be introspected as needed.
type Collector struct {
	mu    sync.Mutex
	stack ers.Stack
}

// New constructs an empty Collector. Collectors can be used without
// any special construction, but this function is shorter.
func New() *Collector { return &Collector{} }

func lock(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); return mtx }
func with(mtx *sync.Mutex)             { mtx.Unlock() }

// Add collects an error if that error is non-nil.
func (ec *Collector) Add(err error) {
	if err == nil {
		return
	}
	defer with(lock(&ec.mu))
	ec.stack.Push(err)
}

// Obesrver returns the collector's Add method as a
// fun.Handler[error] object for integration and use with the
// function types.
func (ec *Collector) Handler() fun.Handler[error] { return ec.Add }

// Future returns a function that is generally equivalent to
// Collector.Resolve(); however, the errors are returned as an unwound
// slice of errors, rather than the ers.Stack object.
func (ec *Collector) Future() fun.Future[error] { return ec.Resolve }

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.Stack, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, Len is not
// updated beyond the current level. In this way Len really reports
// "height," but this is the same for the top level.
func (ec *Collector) Len() int { defer with(lock(&ec.mu)); return ec.stack.Len() }

// Iterator produces an iterator for all errors present in the
// collector. The iterator proceeds from the current error to the
// oldest error, and will not observe new errors added to the
// collector.
func (ec *Collector) Iterator() *fun.Iterator[error] {
	defer with(lock(&ec.mu))
	return fun.CheckProducer(ec.stack.CheckProducer()).Iterator()
}

// Resolve returns an error of type *erc.Stack, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// ers.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	defer with(lock(&ec.mu))

	if ec.stack.Len() == 0 {
		return nil
	}

	return &ec.stack
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool { return ec.Len() != 0 }

// OK returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) OK() bool { return ec.Len() == 0 }
