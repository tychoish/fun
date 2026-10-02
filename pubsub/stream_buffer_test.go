package pubsub

import (
	"context"
	"errors"
	"testing"
)

func failingStream() *Stream[int] {
	n := 0
	return MakeStream(func(context.Context) (int, error) {
		n++
		if n > 3 {
			return 0, errors.New("boom")
		}
		return n, nil
	})
}

func TestBufferedStreamsKeepSourceErrors(t *testing.T) {
	for name, mk := range map[string]func() *Stream[int]{
		"Buffer":         func() *Stream[int] { return failingStream().Buffer(2) },
		"BufferParallel": func() *Stream[int] { return failingStream().BufferParallel(2) },
		"Split":          func() *Stream[int] { return failingStream().Split(2)[0] },
		"ConvertParallel": func() *Stream[int] {
			return ConvertFn(func(i int) int { return i }).Parallel(failingStream())
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := mk()
			for s.Next(t.Context()) {
			}
			if err := s.Close(); err == nil {
				t.Fatal("source error was lost")
			}
		})
	}
}
