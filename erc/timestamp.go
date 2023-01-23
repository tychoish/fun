package erc

import (
	"errors"
	"fmt"
	"io"
	"time"
)

type timestamped struct {
	err error
	ts  time.Time
}

// WithTime adds the error to the collector, only if the error is
// nil, and annotates that error object with the Timestamp type
// implemented in this package.
func WithTime(ec *Collector, err error) { ec.Add(Time(err)) }

// Time wraps an error with a timestamp implementation. The output of
// time.Now will be captured when Time returns, and can be accessed
// via the GetTime method or using the '%+v' formatting argument.
//
// The Timestamp errors, correctly supports errors.Is and errors.As,
// passing through to the wrapped error.
func Time(err error) error {
	if err == nil {
		return nil
	}

	return &timestamped{err: err, ts: time.Now()}
}

// GetTime unwraps the error looking for an error type that
// implements the timestamp interface:
//
//	interface{ Time() time.Time }
//
// and, if so returns the output of that method. Otherwise,
// GetTime returns a zeroed time object. Any Error implementation
// can implement this interface, though the Timestamp type provides a
// basic implementation.
func GetTime(err error) time.Time {
	var irf interface{ Time() time.Time }
	if errors.As(err, &irf) {
		return irf.Time()
	}
	return time.Time{}
}

func (e *timestamped) Error() string      { return e.err.Error() }
func (e *timestamped) Time() time.Time    { return e.ts }
func (e *timestamped) Is(err error) bool  { return errors.Is(e.err, err) }
func (e *timestamped) As(target any) bool { return errors.As(e.err, target) }
func (e *timestamped) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "[%s] %v", e.ts.Format(time.RFC3339), e.err)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}
