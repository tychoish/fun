package internal

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Error string

const ErrInvariantViolation Error = Error("invariant violation")

func (e Error) Error() string { return string(e) }

func (e Error) Is(err error) bool {
	switch {
	case err == nil && e == "":
		return e == ""
	case (err == nil) != (e == ""):
		return false
	default:
		switch x := err.(type) {
		case Error:
			return x == e
		default:
			return false
		}
	}
}

func TTLExec[T any](dur time.Duration) func(op func() T) T {
	if dur < 0 {
		panic(errors.Join(fmt.Errorf("ttl must not be negative: %d", dur), ErrInvariantViolation))
	}

	if dur == 0 {
		return func(op func() T) T { return op() }
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
	}
}

func LimitExec[T any](in int) func(func() T) T {
	if in <= 0 {
		panic(errors.Join(fmt.Errorf("limit must be greater than zero: %d", in), ErrInvariantViolation))
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
	}
}
