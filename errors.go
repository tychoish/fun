package fun

import (
	"errors"
	"strings"
	"sync"
)

// ErrorCollector is a simplified version of the error collector in
// github.com/tychoish/emt. This is thread safe and aggregates errors
// producing a single error.
type ErrorCollector struct {
	mu    sync.Mutex
	cache error
	errs  []error
}

func (ec *ErrorCollector) resetCache() {
	if ec.cache != nil {
		ec.cache = nil
	}
}

func (ec *ErrorCollector) setCache() {
	out := make([]string, len(ec.errs))
	for idx := range ec.errs {
		out[idx] = ec.errs[idx].Error()
	}
	ec.cache = errors.New(strings.Join(out, "; "))
}

// Add collects an error if that error is non-nil.
func (ec *ErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errs = append(ec.errs, err)
	ec.resetCache()
}

// Resolve returns an error, or nil if there have been no errors
// added. The error value is cached.
func (ec *ErrorCollector) Resolve() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.errs) == 0 {
		return nil
	}
	if ec.cache == nil {
		ec.setCache()
	}

	return ec.cache
}

func (ec *ErrorCollector) Len() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	return len(ec.errs)
}

func (ec *ErrorCollector) HasErrors() bool { return ec.Len() > 0 }
