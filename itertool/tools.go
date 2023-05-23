package itertool

import (
	"errors"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

// Options describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type Options struct {
	// ContinueOnPanic forces the entire IteratorMap operation to
	// halt when a single map function panics. All panics are
	// converted to errors and propagated to the output iterator's
	// Close() method.
	ContinueOnPanic bool
	// ContinueOnError allows a map or generate function to return
	// an error and allow the work of the broader operation to
	// continue. Errors are aggregated propagated to the output
	// iterator's Close() method.
	ContinueOnError bool
	// NumWorkers describes the number of parallel workers
	// processing the incoming iterator items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output iterator.
	NumWorkers int
	// OutputBufferSize controls how buffered the output pipe on the
	// iterator should be. Typically this should be zero, but
	// there are workloads for which a moderate buffer may be
	// useful.
	OutputBufferSize int
	// IncludeContextExpirationErrors changes the default handling
	// of context cancellation errors. By default all errors
	// rooted in context cancellation are not propagated to the
	// Close() method, however, when true, these errors are
	// captured. All other error handling semantics
	// (e.g. ContinueOnError) are applicable.
	IncludeContextExpirationErrors bool
	// SkipErrorCheck, when specified, should return true if the
	// error should be skipped and false otherwise.
	SkipErrorCheck func(error) bool
}

func (o Options) shouldCollectError(err error) bool {
	return err != nil && errors.Is(err, fun.ErrRecoveredPanic) || (o.SkipErrorCheck == nil || !o.SkipErrorCheck(err)) || (!o.IncludeContextExpirationErrors || !erc.ContextExpired(err))
}

func (o Options) wrapErrorCheck(newCheck func(error) bool) func(error) bool {
	oldCheck := o.SkipErrorCheck
	return func(err error) bool {
		return (oldCheck == nil || oldCheck(err)) &&
			(newCheck == nil || newCheck(err))
	}
}
