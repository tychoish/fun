package ers

import (
	"errors"
	"fmt"
	"time"
)

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

// WithTime wraps an error with a timestamp implementation. The output of
// time.Now will be captured when WithTime returns, and can be accessed
// via the GetTime method or using the '%+v' formatting argument.
//
// The Timestamp errors, correctly supports errors.Is and errors.As,
// passing through to the wrapped error.
func WithTime(err error) error {
	if err == nil {
		return nil
	}

	return &timestamped{err: err, ts: time.Now()}
}

// NewWithTime creates a new error object with the provided string.
func NewWithTime(e string) error { return WithTime(New(e)) }

type timestamped struct {
	err error
	ts  time.Time
}

func (e *timestamped) Error() string      { return e.err.Error() }
func (e *timestamped) Time() time.Time    { return e.ts }
func (e *timestamped) Is(err error) bool  { return errors.Is(e.err, err) }
func (e *timestamped) As(target any) bool { return errors.As(e.err, target) }
func (e *timestamped) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "[%s] %v", e.ts.Format(time.RFC3339), e.err)
			return
		}
		fallthrough
	case 's':
		_, _ = fmt.Fprint(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
