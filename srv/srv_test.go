package srv

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
)

func makeBlockingService(t *testing.T) *Service {
	t.Helper()
	fired := &atomic.Bool{}

	s := &Service{
		Name: t.Name(),
		Shutdown: func() error {
			t.Helper()
			fired.Store(true)
			return nil
		},
		Run: func(ctx context.Context) error {
			t.Helper()
			timer := testt.Timer(t, 24*time.Hour)
			select {
			case <-timer.C:
				t.Error("this shouldn't happen")
			case <-ctx.Done():
				return nil
			}
			return errors.New("shouldn't get here")
		},
	}

	t.Cleanup(func() {
		t.Helper()
		_ = s.Wait()
		if !fired.Load() {
			t.Error("should run cleanup")
		}
	})

	return s
}

func testCheckOrderingEffects(t *testing.T, s *Service) {
	t.Helper()
	var (
		runAt      time.Time
		cleanupAt  time.Time
		shutdownAt time.Time
	)
	t.Cleanup(func() {
		if runAt.After(cleanupAt) || runAt.After(shutdownAt) {
			t.Error("run at is too late")
		}
		if cleanupAt.Before(shutdownAt) || shutdownAt.After(cleanupAt) {
			t.Error("cleanup is wrong")
		}
		if shutdownAt.Before(runAt) || shutdownAt.After(cleanupAt) {
			t.Error("shutdown is wrong")
		}
		if t.Failed() {
			t.Log("runttime", runAt)
			t.Log("shutdown", shutdownAt)
			t.Log("cleanup@", cleanupAt)
		}
	})

	baseRun := s.Run
	s.Run = func(ctx context.Context) error {
		defer func() { runAt = time.Now() }()
		return baseRun(ctx)
	}
	if s.Cleanup != nil {
		baseCleanup := s.Cleanup
		s.Cleanup = func() error {
			defer func() { cleanupAt = time.Now() }()
			return baseCleanup()
		}
	} else {
		s.Cleanup = func() error {
			defer func() { cleanupAt = time.Now() }()
			return nil
		}
	}

	if s.Shutdown != nil {
		baseShutdown := s.Shutdown
		s.Shutdown = func() error {
			defer func() { shutdownAt = time.Now() }()
			return baseShutdown()
		}
	} else {
		s.Shutdown = func() error {
			defer func() { shutdownAt = time.Now() }()
			return nil
		}
	}
}

func makeSeq(size int) iter.Seq[int] {
	slice := make([]int, size)
	for i := 0; i < size; i++ {
		slice[i] = rand.Intn(size)
	}
	return irt.Slice(slice)
}

func makeQueue(t *testing.T, size int, count *atomic.Int64) *pubsub.Queue[fnx.Worker] {
	t.Helper()
	queue := pubsub.NewUnlimitedQueue[fnx.Worker]()

	for i := 0; i < size; i++ {
		assert.NotError(t, queue.Push(func(_ context.Context) error {
			time.Sleep(2 * time.Millisecond)
			count.Add(1)
			return nil
		}))
	}
	assert.NotError(t, queue.Close())
	return queue
}

func makeErroringQueue(t *testing.T, size int, count *atomic.Int64) *pubsub.Queue[fnx.Worker] {
	t.Helper()
	queue := pubsub.NewUnlimitedQueue[fnx.Worker]()

	for i := 0; i < size; i++ {
		idx := i
		assert.NotError(t, queue.Push(func(_ context.Context) error {
			time.Sleep(2 * time.Millisecond)
			count.Add(1)
			return fmt.Errorf("%d.%q", idx, t.Name())
		}))
	}
	assert.NotError(t, queue.Close())
	return queue
}
