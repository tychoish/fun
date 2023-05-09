// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
)

// Stack represents the error type returned by an ErrorCollector
// when it has more than one error. The implementation provides
// support for errors.Unwrap and errors.Is, and provides an Errors()
// method which returns a slice of the constituent errors for
// additional use.
type Stack struct {
	err  error
	next *Stack
}

func (e *Stack) append(err error) *Stack {
	if err == nil {
		return e
	}

	switch werr := err.(type) {
	case *Stack:
		if werr.next == nil {
			werr.next = e
			return werr
		}

		for werr.next != nil {
			if werr.err != nil {
				e = &Stack{
					err:  werr.err,
					next: e,
				}
			}
			werr = werr.next
		}

		return e
	default:
		return &Stack{
			next: e,
			err:  err,
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
func (e *Stack) Unwrap() error {
	if e.next != nil {
		return e.next
	}

	return nil
}

type esIterImpl struct {
	current *Stack
}

func (_ *esIterImpl) Close() error { return nil }
func (e *esIterImpl) Value() error { return e.current.err }
func (e *esIterImpl) Next(ctx context.Context) bool {
	if e.current.next == nil || ctx.Err() != nil {
		return false
	}

	e.current = e.current.next
	return e.current.err != nil
}

// Iterator returns an iterator object over the contents of the
// stack. While the content of the iterator is immutable and will not
// observe new errors added to the stack, the iterator itself is not
// safe for concurrent access, and must only be used from one
// goroutine, or with a synchronized approach. You may create multiple
// Iterators on the same stack without issue.
func (e *Stack) Iterator() fun.Iterator[error] {
	return adt.NewIterator[error](&sync.Mutex{}, &esIterImpl{current: &Stack{next: e}})
}

// Collector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors which can be resolved as a single error. The
// constituent errors (and flattened, in the case of wrapped errors),
// are an *erc.Stack object, which can be introspected as needed.
type Collector struct {
	mu    sync.Mutex
	stack *Stack
}

// Add collects an error if that error is non-nil.
func (ec *Collector) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.stack == nil {
		ec.stack = &Stack{}
	}

	ec.stack = ec.stack.append(err)
}

// Iterator produces an iterator for all errors present in the
// collector. The iterator proceeds from the current error to the
// oldest error, and will not observe new errors added to the
// collector.
func (ec *Collector) Iterator() fun.Iterator[error] {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	return adt.NewIterator[error](&ec.mu, &esIterImpl{current: &Stack{next: ec.stack}})
}

// Resolve returns an error of type *erc.Stack, or nil if there have
// been no errors added. The error stack respects errors.Is and
// errors.As, and can be iterated or used in combination with
// erc.Unwind() to introspect the available errors.
func (ec *Collector) Resolve() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.stack == nil {
		return nil
	}

	return ec.stack
}

// HasErrors returns true if there are any underlying errors, and
// false otherwise.
func (ec *Collector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.stack != nil && ec.stack.err != nil
}
