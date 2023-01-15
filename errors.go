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
	mtx    sync.RWMutex
	err    error
	next   *ErrorStack
	cached *string
	size   int
	errs   []error
}

func (e *ErrorStack) append(err error) *ErrorStack {
	if err == nil {
		return e
	}

	if e == nil {
		e = &ErrorStack{}
	}
	defer func() {
		e.mtx.Lock()
		defer e.mtx.Unlock()

		e.cached = nil
		e.errs = nil
	}()

	switch werr := err.(type) {
	case *ErrorStack:
		if werr.next == nil {
			werr.next = e
			return werr
		}

		return e.append(werr.next).append(werr.err)
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
	e.mtx.Lock()
	defer e.mtx.Unlock()

	if e.cached != nil {
		return *e.cached
	}

	if e.err == nil && e.next == nil {
		return "empty error"
	}

	var out string
	if e.next != nil && e.next.err != nil {
		out = strings.Join([]string{e.next.Error(), e.err.Error()}, "; ")
	} else {
		out = e.err.Error()
	}

	e.cached = &out

	return out
}

func (e *ErrorStack) Is(err error) bool {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	return errors.Is(e.err, err) || (e.next != nil && e.next.Is(err))
}

func (e *ErrorStack) Unwrap() error {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	if e.next != nil {
		return e.next
	}
	return nil
}

func (e *ErrorStack) Errors() []error {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	if e.err == nil {
		return nil
	}
	if e.next == nil {
		return []error{e.err}
	}

	if len(e.errs) > 1 {
		return e.errs
	}

	e.errs = append([]error{e.err}, e.next.Errors()...)
	return e.errs
}

func (e *ErrorStack) len() int {
	if e == nil {
		return 0
	}
	if e.err == nil {
		return 0
	}
	if e.next == nil {
		return 1
	}
	if e.size > 0 {
		return e.size
	}

	var out int
	if e.err != nil {
		out++
	}
	for item := e.next; item != nil; item = item.next {
		if item.err != nil {
			out++
		}
	}
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.size = out
	return out
}

// ErrorCollector is a simplified version of the error collector in
// github.com/tychoish/emt. This is thread safe and aggregates errors
// producing a single error.
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
		ec.stack = &ErrorStack{
			err: err,
		}
		return
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

func (ec *ErrorCollector) Len() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.stack == nil {
		return 0
	}

	return ec.stack.len()
}

func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.stack != nil && ec.stack.err != nil
}
