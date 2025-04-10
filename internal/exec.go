package internal

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func TTLExec[T any](dur time.Duration) (func(func() T) T, error) {
	if dur < 0 {
		return nil, fmt.Errorf("ttl must not be negative: %d", dur)
	}

	if dur == 0 {
		return func(op func() T) T { return op() }, nil
	}
	var (
		lastAt time.Time
		output T
	)
	mtx := &sync.RWMutex{}

	return func(op func() T) T {
		if out, ok := func() (T, bool) {
			mtx.RLock()
			defer mtx.RUnlock()
			return output, !lastAt.IsZero() && time.Since(lastAt) < dur
		}(); ok {
			return out
		}

		mtx.Lock()
		defer mtx.Unlock()

		since := time.Since(lastAt)
		if lastAt.IsZero() || since >= dur {
			output = op()
			lastAt = time.Now()
		}

		return output
	}, nil
}

func LimitExec[T any](in int) (func(func() T) T, error) {
	if in <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero: %d", in)
	}

	counter := &atomic.Int64{}
	mtx := &sync.Mutex{}

	var output T
	return func(op func() T) T {
		if counter.CompareAndSwap(int64(in), int64(in)) {
			return output
		}

		mtx.Lock()
		defer mtx.Unlock()
		num := counter.Load()

		if num < int64(in) {
			output = op()
			counter.Store(min(int64(in), num+1))
		}

		return output
	}, nil
}
