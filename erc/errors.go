// Package erc provides a simple/fast error aggregation tool for
// collecting and aggregating errors. The tools are compatible with
// go's native error wrapping, and are generally safe for use from
// multiple goroutines, so can simplify error collection patterns in
// worker-pool patterns.
package erc

import (
	"bytes"
	"fmt"
	"iter"
	"strings"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
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

func (ec *Collector) mtx() *sync.Mutex  { return &ec.mu }
func (*Collector) with(m *sync.Mutex)   { m.Unlock() }
func (ec *Collector) lock() *sync.Mutex { m := ec.mtx(); m.Lock(); return m }

///////////////////////////////////////////////////////////////////////
//
// Collector constructors and setup methods
//
///////////////////////////////////////////////////////////////////////

// Join takes a slice of errors and converts it into an *erc.List
// typed error. This operation has several advantages relative to
// using errors.Join(): if you call erc.Join repeatedly on the same
// error set of errors the resulting error is convertable.
func Join(errs ...error) error { st := &Collector{}; st.Join(errs...); return st.Err() }

// JoinSeq takes an iterator sequence of errors and aggregates them into
// a single error. This operation is similar to Join but accepts an
// iter.Seq[error] instead of a variadic slice. Nil errors are filtered
// out, and if all errors are nil, JoinSeq returns nil. If only one
// non-nil error exists, it is returned directly. Otherwise, a
// *Collector is returned containing all non-nil errors.
//
// The resulting error is compatible with errors.Is and errors.As for
// introspecting the constituent errors.
func JoinSeq(errs iter.Seq[error]) error { st := &Collector{}; st.From(errs); return st.Err() }

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
			st.Join(errs...)
			return st
		}
		return nil
	case interface{ Unwrap() []error }:
		if errs := et.Unwrap(); len(errs) > 0 {
			st := &Collector{}
			st.Join(errs...)
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
func (ec *Collector) Error() string {
	defer ec.with(ec.lock())
	switch ec.list.Len() {
	case 0:
		return "<nil>"
	case 1:
		return ec.list.Front().Error()
	default:
		buf := new(bytes.Buffer)
		for elem := range ec.list.FIFO() {
			if buf.Len() != 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(elem.Error())
		}

		return buf.String()
	}
}

// Unwind returns all of the constituent errors held by the
// collector. The implementation of errors.Is and errors.As mean that
// this method is not called for either of those functions, you can
// use this director or with ers.Unwind() to get all errors in a
// slice.
//
// Internally collectors use a linked list implementation, so Unwind()
// requires building the slice.
func (ec *Collector) Unwrap() []error { defer ec.with(ec.lock()); return ec.list.Unwrap() }

// Is supports the errors.Is() function and returns true if any of the
// errors in the collector OR their ancestors are the target error.
func (ec *Collector) Is(target error) bool { defer ec.with(ec.lock()); return ec.list.Is(target) }

// As supports the errors.As() operation to access.
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
// Error (in Collector) Constructors
//
///////////////////////////////////////////////////////////////////////

// Errorf constructs an error, using fmt.Errorf, and adds it to the collector.
func (ec *Collector) Errorf(tmpl string, args ...any) { ec.Push(fmt.Errorf(tmpl, args...)) }

// New adds an error to the collector using the provided string as its value. The error is an ers.Error object.
func (ec *Collector) New(val string) { ec.Push(ers.New(val)) }

///////////////////////////////////////////////////////////////////////
//
// Conditional Errors
//
///////////////////////////////////////////////////////////////////////

// If adds the error to the collector when the condition is true, and ignores it otherwise.
func (ec *Collector) If(cond bool, val error) { ec.Push(ers.If(cond, val)) }

// When is a helper function, typically useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func (ec *Collector) When(cond bool, val string) { ec.Push(ers.When(cond, val)) }

// Whenf conditionally creates and adds an error to the collector, as
// When, and with a similar use case, but permits Sprintf/Errorf
// formating.
func (ec *Collector) Whenf(cond bool, val string, args ...any) {
	ec.Push(ers.Whenf(cond, val, args...))
}

///////////////////////////////////////////////////////////////////////
//
// Error Annotation Methods
//
///////////////////////////////////////////////////////////////////////

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

// Annotate, like Wrap, attaches context to an error, and adds it to
// the collector when the error is non-nil, otherwise Annotate is a
// noop.
//
// Unlike Wrap, Annotate places the annotation after the error, and
// encloses it in square brackets.
func (ec *Collector) Annotate(err error, annotation string) {
	ec.Push(ers.Annotate(err, annotation))
}

// Annotatef, like Wrap, attaches context to an error, and adds it to
// the collector when the error is non-nil, otherwise Annotatef is a
// noop.
//
// Unlike Wrap, Annotate places the annotation after the error, and
// encloses it in square brackets.
func (ec *Collector) Annotatef(err error, tmpl string, args ...any) {
	ec.Push(ers.Annotatef(err, tmpl, args...))
}

///////////////////////////////////////////////////////////////////////
//
// Collection methods
//
///////////////////////////////////////////////////////////////////////

// Push collects an error if that error is non-nil.
func (ec *Collector) Push(err error) {
	if ers.IsError(err) {
		ec.pushWithLock(err)
	}
}

// PushOk adds the error to the collector and then returns true if the
// error is nil, and false otherwise.
func (ec *Collector) PushOk(err error) bool {
	if ers.IsError(err) {
		ec.pushWithLock(err)
		return false
	}
	return true
}

// Join appends one or more errors to the collectors. Nil errors are
// always omitted from the collector.
func (ec *Collector) Join(errs ...error) {
	switch len(errs) {
	case 0:
		return
	case 1:
		ec.Push(errs[0])
	default:
		ec.addWithLock(irt.Slice(errs))
	}
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func (ec *Collector) Check(fut func() error) { ec.Push(fut()) }

// SetFilter sets (or overrides) the current filter on the
// collector. Errors errors collected by the filter are passed to the
// filter function. Filters will not receive nil errors.
func (ec *Collector) SetFilter(erf Filter) { defer ec.with(ec.lock()); ec.list.filter = erf }

// From adds all non-nil error values from the iterator sequence to the
// error collector. Nil errors in the sequence are automatically filtered
// out and ignored. This method is thread-safe and can be called
// concurrently with other Collector methods.
//
// The method uses irt.KeepErrors to filter the sequence, ensuring only
// non-nil errors are added to the collector.
func (ec *Collector) From(seq iter.Seq[error])        { irt.Apply(irt.KeepErrors(seq), ec.pushWithLock) }
func (ec *Collector) pushWithLock(err error)          { defer ec.with(ec.lock()); ec.list.Push(err) }
func (ec *Collector) addWithLock(seq iter.Seq[error]) { defer ec.with(ec.lock()); ec.list.From(seq) }

///////////////////////////////////////////////////////////////////////
//
// Panic Management
//
///////////////////////////////////////////////////////////////////////

// Recover can be used in a defer to collect a panic and add it to the collector.
func (ec *Collector) Recover() { ec.Push(ParsePanic(recover())) }

// WithRecover calls the provided function, collecting any panic and
// converting it to an error// that is added to the collector.
func (ec *Collector) WithRecover(fn func()) { defer func() { ec.Push(ParsePanic(recover())) }(); fn() }

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

func (ec *Collector) extractErrors(in []any) {
	args := make([]string, 0, len(in))

	for idx := range in {
		switch val := in[idx].(type) {
		case nil:
			continue
		case error:
			ec.Push(val)
		case string:
			args = append(args, strings.TrimSpace(val))
		case func() error:
			ec.Push(val())
		case fmt.Stringer:
			args = append(args, strings.TrimSpace(val.String()))
		case func() string:
			args = append(args, strings.TrimSpace(val()))
		default:
			args = append(args, fmt.Sprint(val))
		}
	}

	if len(args) > 0 {
		ec.Push(ers.New(irt.JoinStringsWith(irt.RemoveZeros(irt.Slice(args)), " ")))
	}
}
