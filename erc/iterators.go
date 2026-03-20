package erc

import "iter"

// Handle converts an error-producing iterator into a regular iterator by
// handling errors separately. Values with non-nil errors are skipped and
// their errors are passed to the handler function. Only values without
// errors are yielded to the consumer. This allows error-free iteration
// while still processing errors through a handler.
//
// Use Handle when you want to skip failed values but continue processing
// and handle errors separately (e.g., logging, collecting, etc.).
func Handle[T any](seq iter.Seq2[T, error], handler func(error)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for val, err := range seq {
			switch {
			case err != nil:
				handler(err)
				continue
			case !yield(val):
				return
			}
		}
	}
}

// HandleUntil converts an error-producing iterator into a regular iterator
// with fail-fast behavior. It yields values until the first error is
// encountered. When an error occurs, the handler is called with that error
// and iteration stops immediately. Values before the error are yielded
// normally. This provides fail-fast semantics with error notification.
//
// Use HandleUntil when you want to stop processing at the first error
// and handle it (e.g., logging) before terminating iteration.
func HandleUntil[T any](seq iter.Seq2[T, error], handler func(error)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for val, err := range seq {
			switch {
			case err != nil:
				handler(err)
				return
			case !yield(val):
				return
			}
		}
	}
}

// HandleAll converts an error-producing iterator into a regular iterator
// that processes ALL values regardless of errors. The handler is called
// for every iteration (even when error is nil) and all values are yielded,
// including those with associated errors. This allows complete iteration
// while tracking all errors.
//
// Use HandleAll when you need to process every value from the sequence
// while tracking errors (e.g., for partial success scenarios).
func HandleAll[T any](seq iter.Seq2[T, error], handler func(error)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for val, err := range seq {
			handler(err)
			if !yield(val) {
				return
			}
		}
	}
}

// FromIterator consumes an iterator sequence and returns only the values
// that were successfully produced (without errors). All errors encountered
// during iteration are aggregated using an error Collector. If any errors
// occur, both a non-nil slice of successful values AND a non-nil aggregated
// error are returned, allowing partial results to be recovered.
//
// Use FromIterator when you want to skip failed values but continue
// processing and collect all errors for later inspection.
func FromIterator[T any](seq iter.Seq2[T, error]) ([]T, error) {
	out := slice[T]{}
	err := ForIterator(seq, out.push)
	return out, err
}

// FromIteratorAll consumes an iterator sequence and returns ALL values
// produced, regardless of whether they had associated errors. Unlike
// FromIterator, this function includes values even when an error occurred
// for that iteration. All errors are aggregated using an error Collector.
// If any errors occur, both a non-nil slice (containing all values) AND
// a non-nil aggregated error are returned.
//
// Use FromIteratorAll when you need to collect every value from the
// sequence, even those that produced errors, while still tracking all
// errors that occurred.
func FromIteratorAll[T any](seq iter.Seq2[T, error]) ([]T, error) {
	out := slice[T]{}
	err := ForIteratorAll(seq, out.push)
	return out, err
}

// FromIteratorUntil consumes an iterator sequence and stops immediately
// upon encountering the first error. Unlike FromIterator and FromIteratorAll,
// this function does NOT aggregate errors - it returns the first error
// encountered directly and stops iteration. All successfully processed
// values before the error are returned in the slice. If no errors occur,
// returns the complete slice with a nil error.
//
// Use FromIteratorUntil when you want fail-fast behavior and don't need
// to continue processing after the first error.
func FromIteratorUntil[T any](seq iter.Seq2[T, error]) ([]T, error) {
	out := slice[T]{}
	err := ForIteratorUntil(seq, out.push)
	return out, err
}

// ForIterator iterates through value-error pairs, passing the value
// to the function and aggregating the error. The function is ONLY
// called for the values where the error is nil.
func ForIterator[T any, OP ~func(T)](seq iter.Seq2[T, error], op OP) error {
	var ec Collector
	for value, err := range seq {
		if ec.PushOk(err) {
			op(value)
		}
	}
	return ec.Resolve()
}

// ForIteratorAll iterates through value-error pairs, passing the value
// to the function and aggregating the error. The function is ALWAYS
// called, even if the error is nil.
func ForIteratorAll[T any, OP ~func(T)](seq iter.Seq2[T, error], op OP) error {
	var ec Collector
	for value, err := range seq {
		ec.Push(err)
		op(value)
	}
	return ec.Resolve()
}

// ForIteratorUntil iterates through value-error pairs, passing the
// value to the function UNTIL the error value is non nil. The first
// error encountered is always returned.
func ForIteratorUntil[T any, OP ~func(T)](seq iter.Seq2[T, error], op OP) error {
	for value, err := range seq {
		if err != nil {
			return err
		}
		op(value)
	}
	return nil
}

type slice[T any] []T

func (sl *slice[T]) push(value T) { *sl = append(*sl, value) }
