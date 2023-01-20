// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tychoish/fun"
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
	case interface{ Unwrap() error }:
		for {
			e = &Stack{
				err:  err,
				next: e,
			}
			err = errors.Unwrap(err)
			if err == nil {
				break
			}
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
		return strings.Join([]string{e.next.Error(), e.err.Error()}, "; ")
	}

	return e.err.Error()
}

// Is calls errors.Is on the underlying error to provied compatibility
// with errors.Is, which takes advantage of this interface.
func (e *Stack) Is(err error) bool { return errors.Is(e.err, err) }

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

func (_ *esIterImpl) Close(_ context.Context) error { return nil }
func (e *esIterImpl) Value() error                  { return e.current.err }
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
	return &esIterImpl{current: &Stack{next: e}}
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

// Recover calls the builtin recover() function and converts it to an
// error that is populated in the collector.
func Recover(ec *Collector) {
	if r := recover(); r != nil {
		ec.Add(fmt.Errorf("panic: %v", r))
	}
}

// RecoverAction calls the builtin recover() function and converts it to an
// error in the collector, for direct use in defers. The callback
// function is only called if the panic is non-nil.
func (ec *Collector) Recover(callback func()) {
	if r := recover(); r != nil {
		ec.Add(fmt.Errorf("panic: %v", r))
		if callback != nil {
			callback()
		}
	}
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func (ec *Collector) Check(fn func() error) { ec.Add(fn()) }

// CheckCtx executes a simple function that takes a context and if it
// returns an error, adds it to the collector, primarily for use in
// defer statements.
func CheckCtx(ctx context.Context, ec *Collector, fn func(context.Context) error) {
	ec.Check(func() error { return fn(ctx) })
}

// Iterator produces an iterator for all errors present in the
// collector. The iterator proceeds from the current error to the
// oldest error, and will not observe new errors added to the
// collector. While the iterator isn't safe for current access from
// multiple go routines, the sequence of errors stored are not
// modified after creation.
func (ec *Collector) Iterator() fun.Iterator[error] {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	return ec.stack.Iterator()
}

// Resolve returns an error of type AggregatedError, or nil if there
// have been no errors added.
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
