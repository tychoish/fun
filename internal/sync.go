package internal

import (
	"sync"
)

func Mnemonize[T any](in func() T) func() T {
	once := &sync.Once{}
	var value T

	return func() T {
		once.Do(func() { value = in() })
		return value
	}
}
