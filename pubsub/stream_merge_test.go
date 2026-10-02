package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

func infiniteStream() *Stream[int] {
	return MakeStream(func(context.Context) (int, error) { return 1, nil })
}

func TestMergeStreamsCloseDoesNotHang(t *testing.T) {
	m := MergeStreams(VariadicStream(infiniteStream(), infiniteStream()))
	if _, err := m.Read(t.Context()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = m.Close() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked on producers that nothing is consuming")
	}
}

func TestAbandonedStreamsStopWorkers(t *testing.T) {
	for name, mk := range map[string]func() *Stream[int]{
		"Buffer":         func() *Stream[int] { return infiniteStream().Buffer(2) },
		"BufferParallel": func() *Stream[int] { return infiniteStream().BufferParallel(2) },
		"Split":          func() *Stream[int] { return infiniteStream().Split(2)[0] },
		"ConvertParallel": func() *Stream[int] {
			return ConvertFn(func(i int) int { return i }).Parallel(infiniteStream())
		},
	} {
		t.Run(name, func(t *testing.T) {
			leakCheck := testt.NoGoroutineLeak(t, 5*time.Second)
			s := mk()
			if _, err := s.Read(t.Context()); err != nil {
				t.Fatal(err)
			}
			_ = s.Close()
			leakCheck()
		})
	}
}
