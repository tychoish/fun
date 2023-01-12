package fun

import (
	"errors"
	"strings"
	"sync"
)

type ErrorCollector struct {
	mu    sync.Mutex
	cache string
	errs  []error
}

func (ec *ErrorCollector) resetCache() {
	if ec.cache != "" {
		ec.cache = ""
	}
}

func (ec *ErrorCollector) setCache() {
	out := make([]string, len(ec.errs))
	for idx := range ec.errs {
		out[idx] = ec.errs[idx].Error()
	}
	ec.cache = strings.Join(out, "; ")
}

func (ec *ErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errs = append(ec.errs, err)
	ec.resetCache()
}

func (ec *ErrorCollector) Resolve() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.errs) == 0 {
		return nil
	}
	if ec.cache == "" {
		ec.setCache()
	}

	return errors.New(ec.cache)
}
