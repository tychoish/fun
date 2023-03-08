package srv

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

func makeBlockingService(t *testing.T) *Service {
	t.Helper()
	fired := &atomic.Bool{}

	t.Cleanup(func() {
		if !fired.Load() {
			t.Error("should run shutdown")
		}
	})

	return &Service{
		Name: t.Name(),
		Cleanup: func() error {
			fired.Store(true)
			return nil
		},
		Run: func(ctx context.Context) error {
			timer := testt.Timer(t, time.Hour)
			select {
			case <-timer.C:
				t.Error("this shouldn't happen")
			case <-ctx.Done():
				return nil
			}
			return errors.New("shouldn't get here")
		},
	}
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
