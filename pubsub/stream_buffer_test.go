package pubsub

import (
	"context"
	"errors"
	"slices"
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

func TestBufferOrdering(t *testing.T) {
	const size = 2000
	input := make([]int, size)
	for i := range input {
		input[i] = i
	}
	for _, n := range []int{1, 2, 8} {
		t.Run("Buffer", func(t *testing.T) {
			out, err := SliceStream(input).Buffer(n).Slice(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(out, input) {
				t.Fatalf("Buffer(%d) did not preserve input order", n)
			}
		})
		t.Run("BufferParallel", func(t *testing.T) {
			out, err := SliceStream(input).BufferParallel(n).Slice(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(out)
			if !slices.Equal(out, input) {
				t.Fatalf("BufferParallel(%d) lost or duplicated items", n)
			}
		})
	}
}
