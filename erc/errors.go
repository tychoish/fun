// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
)

// Stack represents the error type returned by an ErrorCollector
// when it has more than one error. The implementation provides
// support for errors.Unwrap and errors.Is, and provides an Errors()
// method which returns a slice of the constituent errors for
// additional use.
type Stack struct {
	err   error
	next  *Stack
	count int
}

func (e *Stack) append(err error) *Stack {
	if err == nil {
		return e
	}
	var count int
	if e != nil {
		count = e.count
	}
	switch werr := err.(type) {
	case *Stack:
		if werr.next == nil {
			werr.next = e
			werr.count = count + 1
			return werr
		}

		for werr.next != nil {
			if werr.err != nil {
				e = &Stack{
					err:   werr.err,
					next:  e,
					count: count + 1,
				}
				count = e.count
			}
			werr = werr.next
		}

		return e
	case interface{ Unwrap() []error }:
		for _, err := range werr.Unwrap() {
			e = e.append(err)
		}
		return e
	default:
		return &Stack{
			next:  e,
			err:   err,
			count: count + 1,
		}
	}
}

// Error produces the aggregated error strings from this method. If
// the error at the current layer is nil.
func (e *Stack) Error() string {
	if e.err == nil && e.next == nil {
		return "empty error"
	}

	if e.next != nil && e.next.err != nil {
		return strings.Join([]string{e.next.Error(), e.err.Error()}, ": ")
	}

	return e.err.Error()
}

// Is calls errors.Is on the underlying error to provied compatibility
// with errors.Is, which takes advantage of this interface.
func (e *Stack) Is(err error) bool { return errors.Is(e.err, err) }

// As calls errors.As on the underlying error to provied compatibility
// with errors.As, which takes advantage of this interface.
func (e *Stack) As(target any) bool { return errors.As(e.err, target) }

// Unwrap returns the next iterator in the stack, and is compatible
// with errors.Unwrap.
func (e *Stack) Unwrap() []error {
	if e == nil || e.next == nil {
		return nil
	}
	return ft.Must(e.Iterator().Slice(context.Background()))
}

// Producer exposes the state of the error stack as a producer
// function, and can be used to build iterators or other operatios in
// the fun package.
func (e *Stack) Producer() fun.Producer[error] {
	iter := &Stack{next: e}
	return fun.Producer[error](func(ctx context.Context) (value error, err error) {
		switch {
		case iter.next == nil || iter.next.err == nil:
			return nil, io.EOF
		case ctx.Err() != nil:
			return nil, ctx.Err()
		default:
			iter = iter.next
			value = iter.err
			return value, nil
		}
	})
}

// Iterator returns an iterator object over the contents of the
// stack. While the content of the iterator is immutable and will not
// observe new errors added to the stack, the iterator itself is not
// safe for concurrent access, and must only be used from one
// goroutine, or with a synchronized approach. You may create multiple
// Iterators on the same stack without issue.
func (e *Stack) Iterator() *fun.Iterator[error] { return e.Producer().Iterator() }

// Collector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors which can be resolved as a single error. The
// constituent errors (and flattened, in the case of wrapped errors),
// are an *erc.Stack object, which can be introspected as needed.
type Collector struct {
	mu    sync.Mutex
	stack *Stack
}

func lock(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); return mtx }
func with(mtx *sync.Mutex)             { mtx.Unlock() }

// Add collects an error if that error is non-nil.
func (ec *Collector) Add(err error) {
	if err == nil {
		return
	}
	defer with(lock(&ec.mu))
	if ec.stack == nil {
		ec.stack = &Stack{}
	}

	ec.stack = ec.stack.append(err)
}

// Obesrver returns the collector's Add method as a
// fun.Observer[error] object for integration and use with the
// function types.
func (ec *Collector) Observer() fun.Observer[error] { return ec.Add }

// Len reports on the total number of non-nil errors collected. The
// count tracks a cached size of the *erc.Stack, giving Len() stable
// performance characteristics; however, because the Collector unwrap
// and merge Stack and other { Unwrap() []error } errors, the number
// may not
func (ec *Collector) Len() int {
	defer with(lock(&ec.mu))
	if ec.stack == nil {
		return 0
	}
	return ec.stack.count
}

// Iterator produces an iterator for all errors present in the
// collector. The iterator proceeds from the current error to the
// oldest error, and will not observe new errors added to the
// collector.
func (ec *Collector) Iterator() *fun.Iterator[error] {
	defer with(lock(&ec.mu))

	return ec.stack.Producer().WithLock(&ec.mu).Iterator()
}

// Resolve returns an error of type *erc.Stack, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// fun.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	defer with(lock(&ec.mu))

	if ec.stack == nil {
		return nil
	}

	return ec.stack
}

// Producer returns a function that is generally equivalent to
// Collector.Resolve(); however, the errors are returned as an
// Unwound/Unwrapped slice of errors, rather than the ers.Stack
// object. Additionally, the producer can return an error if the context
// is canceled.
func (ec *Collector) Producer() fun.Producer[[]error] {
	return func(ctx context.Context) ([]error, error) {
		defer with(lock(&ec.mu))
		return ec.stack.Iterator().Slice(ctx)
	}
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.stack != nil && ec.stack.err != nil
}
