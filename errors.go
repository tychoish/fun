package fun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrorStack represents the error type returned by an ErrorCollector
// when it has more than one error. The implementation provides
// support for errors.Unwrap and errors.Is, and provides an Errors()
// method which returns a slice of the constituent errors for
// additional use.
type ErrorStack struct {
	err  error
	next *ErrorStack
}

func (e *ErrorStack) append(err error) *ErrorStack {
	if err == nil {
		return e
	}

	if e == nil {
		e = &ErrorStack{}
	}

	switch werr := err.(type) {
	case *ErrorStack:
		if werr.next == nil {
			werr.next = e
			return werr
		}

		for werr.next != nil {
			if werr.err != nil {
				e = &ErrorStack{
					err:  werr.err,
					next: e,
				}
			}
			werr = werr.next
		}

		return e
	case interface{ Unwrap() error }:
		for {
			e = &ErrorStack{
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
		return &ErrorStack{
			next: e,
			err:  err,
		}
	}
}

func (e *ErrorStack) Error() string {
	if e.err == nil && e.next == nil {
		return "empty error"
	}

	if e.next != nil && e.next.err != nil {
		return strings.Join([]string{e.next.Error(), e.err.Error()}, "; ")
	}

	return e.err.Error()
}

func (e *ErrorStack) Is(err error) bool { return errors.Is(e.err, err) }

func (e *ErrorStack) Unwrap() error {
	if e.next != nil {
		return e.next
	}
	return nil
}

func (e *ErrorStack) Errors() []error {
	if e.err == nil {
		return nil
	}

	if e.next == nil {
		return []error{e.err}
	}

	return append([]error{e.err}, e.next.Errors()...)
}

func (e *ErrorStack) len() int {
	if e == nil || e.err == nil {
		return 0
	}
	if e.next == nil {
		return 1
	}

	var out int = 1
	for item := e.next; item != nil; item = item.next {
		if item.err != nil {
			out++
		}
	}

	return out
}

// ErrorCollector is a simplified version of the error collector in
// github.com/tychoish/emt. The collector is thread safe and
// aggregates errors producing a single error.
type ErrorCollector struct {
	mu    sync.Mutex
	stack *ErrorStack
}

// Add collects an error if that error is non-nil.
func (ec *ErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.stack == nil {
		ec.stack = &ErrorStack{}
	}

	ec.stack = ec.stack.append(err)
}

// Recover calls the builtin recover() function and converts it to an
// error in the collector, for direct use in defers.
func (ec *ErrorCollector) Recover() {
	if r := recover(); r != nil {
		ec.Add(fmt.Errorf("panic: %v", r))
	}
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func (ec *ErrorCollector) Check(fn func() error) { ec.Add(fn()) }

// CheckCtx executes a simple function that takes a context and if it
// returns an error, adds it to the collector, primarily for use in
// defer statements.
func (ec *ErrorCollector) CheckCtx(ctx context.Context, fn func(context.Context) error) {
	ec.Add(fn(ctx))
}

// Resolve returns an error of type AggregatedError, or nil if there
// have been no errors added.
func (ec *ErrorCollector) Resolve() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.stack == nil {
		return nil
	}
	if ec.stack.next == nil {
		return ec.stack.err
	}

	return ec.stack
}

func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.stack != nil && ec.stack.err != nil
}
