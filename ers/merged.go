package ers

import (
	"errors"
	"fmt"
	"sync"
)

type Collector struct {
	mtx sync.Mutex
	err error
	num int
}

func lock(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); return mtx }
func with(mtx *sync.Mutex)             { mtx.Unlock() }

func (c *Collector) HasErrors() bool { defer with(lock(&c.mtx)); return c.err != nil }
func (c *Collector) Resolve() error  { defer with(lock(&c.mtx)); return c.err }
func (c *Collector) Len() int        { defer with(lock(&c.mtx)); return c.num }

func (c *Collector) Add(err error) {
	if err == nil {
		return
	}

	defer with(lock(&c.mtx))
	c.num++
	c.err = Merge(err, c.err)
}

func Merge(one, two error) error {
	switch {
	case one == nil && two == nil:
		return nil
	case one == nil && two != nil:
		return two
	case one != nil && two == nil:
		return one
	default:
		return &Combined{Current: one, Previous: two}
	}

}

type Combined struct {
	Current  error
	Previous error
}

func (dwe *Combined) Unwrap() error { return dwe.Previous }
func (dwe *Combined) Error() string { return fmt.Sprintf("%v: %v", dwe.Current, dwe.Previous) }

func (dwe *Combined) Is(target error) bool {
	return errors.Is(dwe.Current, target) || errors.Is(dwe.Previous, target)
}

func (dwe *Combined) As(target any) bool {
	return errors.As(dwe.Current, target) || errors.As(dwe.Previous, target)
}
