package srv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestService(t *testing.T) {
	t.Parallel()
	t.Run("Accessors", func(t *testing.T) {
		t.Run("Name", func(t *testing.T) {
			s := &Service{}
			if s.String() != "Service<>" {
				t.Error(s.String())
			}
			s.Name = "buddy"
			if s.String() != "Service<buddy>" {
				t.Error(s.String())
			}
		})
		t.Run("Defaults", func(t *testing.T) {
			s := &Service{}
			if s.Running() {
				t.Error("should not be running")
			}
		})
	})
	t.Run("Wait", func(t *testing.T) {
		t.Parallel()
		t.Run("Unstarted", func(t *testing.T) {
			s := &Service{}
			err := s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if !errors.Is(err, ErrServiceNotStarted) {
				t.Fatal(err)
			}
		})
		t.Run("WaitWaits", func(t *testing.T) {
			startAt := time.Now()
			s := makeBlockingService(t)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := s.Start(ctx); err != nil {
				t.Error(err)
			}

			sig := make(chan struct{})
			go func() {
				defer close(sig)
				if err := s.Wait(); err != nil {
					t.Error(err)
				}
			}()
			time.Sleep(100 * time.Millisecond)
			s.Close()
			<-sig
			dur := time.Since(startAt)
			if dur < 100*time.Millisecond || dur > time.Second {
				t.Error(dur)
			}
		})
	})
	t.Run("Close", func(t *testing.T) {
		s := &Service{}
		if s.Running() {
			t.Fatal("not running")
		}
		{
			// ensure it doesn't panic:
			s.Close()
		}
		if s.Running() {
			t.Fatal("still not running")
		}
	})
	t.Run("Start", func(t *testing.T) {
		t.Parallel()
		t.Run("AlreadyStarted", func(t *testing.T) {
			s := makeBlockingService(t)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Start(ctx); !errors.Is(err, ErrServiceAlreadyStarted) {
				t.Error(err)
			}
			s.Close()
			if err := s.Wait(); err != nil {
				t.Error(err)
			}
		})
		t.Run("AlreadyFinished", func(t *testing.T) {
			s := makeBlockingService(t)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			s.Close()
			if err := s.Wait(); err != nil {
				t.Error(err)
			}
			if err := s.Start(ctx); !errors.Is(err, ErrServiceReturned) {
				t.Error(err)
			}
		})
	})
	t.Run("PanicSafety", func(t *testing.T) {
		t.Run("Cleanup", func(t *testing.T) {
			s := &Service{
				Cleanup: func() error { panic("whoops") },
				Run:     func(context.Context) error { return nil },
			}

			ctx := t.Context()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("NilRun", func(t *testing.T) {
			s := &Service{}
			ctx := t.Context()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("Run", func(t *testing.T) {
			s := &Service{
				Run: func(context.Context) error { panic("whoops") },
			}

			ctx := t.Context()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
	})
	t.Run("Group", func(t *testing.T) {
		counter := &atomic.Int64{}
		list := make([]*Service, 0, 100)
		for i := range 100 {
			list = append(list, &Service{
				Name: fmt.Sprint(i),
				Run: func(_ context.Context) error {
					counter.Add(1)
					return nil
				},
			})
		}
		s := Group(irt.Slice(list))
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := s.Wait(); err != nil {
			t.Error(err)
		}
		if counter.Load() != 100 {
			t.Error(counter.Load())
		}
	})
	t.Run("Shutdown", func(t *testing.T) {
		t.Run("PanicSafely", func(t *testing.T) {
			s := &Service{
				Run: func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return nil
					case <-time.After(time.Second):
						t.Fatal("shouldn't timeout")
					}
					return nil
				},
				Cleanup:  func() error { return nil },
				Shutdown: func() error { panic("whoops") },
			}

			ctx := t.Context()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}

			time.Sleep(10 * time.Millisecond)
			s.Close()
			err := s.Wait()
			if err == nil {
				t.Fatal("expected error")
			}

			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("ErrorPropogates", func(t *testing.T) {
			s := &Service{
				Run:      func(context.Context) error { return nil },
				Cleanup:  func() error { return nil },
				Shutdown: func() error { return errors.New("expected") },
			}

			ctx := t.Context()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}

			err := s.Wait()
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != "expected" {
				t.Error("got unexpected error", err)
			}
		})
		t.Run("HappensBefore", func(t *testing.T) {
			s := &Service{
				Run:      func(context.Context) error { return nil },
				Cleanup:  func() error { return nil },
				Shutdown: func() error { return nil },
			}
			testCheckOrderingEffects(t, s)

			ctx := t.Context()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("HTTP", func(t *testing.T) {
		t.Run("Ordering", func(t *testing.T) {
			hs, addr := newLoopbackServer(nil)
			s := HTTP("test", time.Minute, hs)
			testCheckOrderingEffects(t, s)

			ctx := t.Context()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			<-addr
			s.Close()
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("ErrorStartup", func(t *testing.T) {
			ctx := t.Context()

			hs1, addr1 := newLoopbackServer(nil)
			s1 := HTTP("test", time.Second, hs1)
			if err := s1.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// the first server is listening once its address is published.
			taken := <-addr1
			if !s1.Running() {
				t.Error("should be running")
			}
			hs2 := &http.Server{Addr: taken}
			s2 := HTTP("test", time.Second, hs2)
			if err := s2.Start(ctx); err != nil {
				t.Error(err)
			}

			// the address is in use, so the second service exits on its own with an error.
			if err := s2.Wait(); err == nil {
				t.Error("second service should have errored")
			}
			s2.Close()

			if !s1.Running() {
				t.Error("should STILL be running")
			}
			s1.Close()
			if err := s1.Wait(); err != nil {
				t.Error(err)
			}
		})
		t.Run("ErrorShutdown", func(t *testing.T) {
			hs, addr := newLoopbackServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				time.Sleep(20 * time.Millisecond)
			}))
			s := HTTP("test", time.Millisecond, hs)
			testCheckOrderingEffects(t, s)

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			target := <-addr

			sig := make(chan struct{})
			go func() {
				defer close(sig)
				//nolint:bodyclose
				resp, err := http.DefaultClient.Do(
					erc.Must(http.NewRequestWithContext(ctx, http.MethodGet, "http://"+target+"/", nil)),
				)
				if err != nil {
					t.Error(err)
				}
				if resp == nil {
					t.Error("response body should not be nil")
					return
				}

				if resp.StatusCode != http.StatusOK {
					t.Error(resp.StatusCode)
				}
			}()

			<-sig
			s.Close()
			if err := s.Wait(); err != nil {
				t.Fatal("expected no error", err)
			}
		})
	})
	t.Run("ErrorHandler", func(t *testing.T) {
		oberr := &adt.Atomic[error]{}
		s := &Service{
			Run:      func(context.Context) error { return errors.New("run") },
			Shutdown: func() error { return errors.New("shutdown") },
			Cleanup:  func() error { panic("cleanup") },
		}
		s.ErrorHandler.Set(func(err error) { oberr.Set(err) })

		assert.NotError(t, s.Start(t.Context()))
		err := s.Wait()
		errs := ers.Unwind(err)
		assert.Error(t, err)
		check.Equal(t, len(errs), 3)
		assert.Error(t, oberr.Get())
		assert.Equal(t, err.Error(), oberr.Get().Error())
		assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
	})
	t.Run("Worker", func(t *testing.T) {
		t.Run("Timeout", func(t *testing.T) {
			s := makeBlockingService(t)
			ctx := testt.ContextWithTimeout(t, 10*time.Millisecond)
			var err error
			assert.MaxRuntime(t, 20*time.Millisecond, func() {
				err = s.Worker().Run(ctx)
			})
			assert.Error(t, err)
			assert.True(t, ers.IsExpiredContext(err))
			assert.True(t, s.isStarted.Load())
		})
		t.Run("ErrorPropogate", func(t *testing.T) {
			expected := errors.New("run")
			s := &Service{
				Run:      func(context.Context) error { return expected },
				Shutdown: func() error { return errors.New("shutdown") },
			}

			ctx := testt.Context(t)
			err := s.Worker().Run(ctx)
			assert.ErrorIs(t, err, expected)
		})
	})
}

// newLoopbackServer returns an http.Server bound to an ephemeral
// loopback port. The returned channel (buffered) receives the actual
// listen address once the server is accepting connections.
func newLoopbackServer(h http.Handler) (*http.Server, <-chan string) {
	addr := make(chan string, 1)
	return &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: h,
		BaseContext: func(l net.Listener) context.Context {
			addr <- l.Addr().String()
			return context.Background()
		},
	}, addr
}
