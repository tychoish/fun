package srv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/seq"
)

func TestService(t *testing.T) {
	t.Parallel()
	t.Run("Accessors", func(t *testing.T) {
		t.Run("Name", func(t *testing.T) {
			s := &Service{}
			if s.String() != "Service<>" {
				t.Error(s.String())
			}
			s.Name = "merlin"
			if s.String() != "Service<merlin>" {
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

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if !strings.HasPrefix(err.Error(), "panic: whoops") {
				t.Error(err)
			}
		})
		t.Run("NilRun", func(t *testing.T) {
			s := &Service{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if !strings.HasPrefix(err.Error(), "panic:") {
				t.Error(err)
			}
		})
		t.Run("Run", func(t *testing.T) {
			s := &Service{
				Run: func(context.Context) error { panic("whoops") },
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := s.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if !strings.HasPrefix(err.Error(), "panic: whoops") {
				t.Error(err)
			}
		})
	})
	t.Run("Group", func(t *testing.T) {
		counter := &atomic.Int64{}
		list := seq.List[*Service]{}
		for i := 0; i < 100; i++ {
			list.PushBack(&Service{
				Name: fmt.Sprint(i),
				Run: func(ctx context.Context) error {
					counter.Add(1)
					return nil
				},
			})
		}
		s := Group(seq.ListValues(list.IteratorPop()))
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}

			time.Sleep(10 * time.Millisecond)
			s.Close()
			err := s.Wait()
			if err == nil {
				t.Fatal("expected error")
			}

			if err.Error() != "panic: whoops" {
				t.Fatal(err)
			}

		})
		t.Run("ErrorPropogates", func(t *testing.T) {
			s := &Service{
				Run:      func(context.Context) error { return nil },
				Cleanup:  func() error { return nil },
				Shutdown: func() error { return errors.New("expected") },
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			hs := &http.Server{
				Addr: "127.0.0.2:2340",
			}
			s := HTTP("test", time.Minute, hs)
			testCheckOrderingEffects(t, s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
			s.Close()
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("ErrorStartup", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			hs1 := &http.Server{
				Addr: "127.0.0.2:2340",
			}
			s1 := HTTP("test", time.Millisecond, hs1)
			if err := s1.Start(ctx); err != nil {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
			hs2 := &http.Server{
				Addr: "127.0.0.2:2340",
			}
			s2 := HTTP("test", time.Millisecond, hs2)
			if err := s2.Start(ctx); err != nil {
				t.Error(err)
			}

			s2.Close()
			if err := s2.Wait(); err == nil {
				t.Error("second service should have errored")
			}

			s1.Close()
			if err := s1.Wait(); err != nil {
				t.Error(err)
			}
		})
		t.Run("ErrorShutdown", func(t *testing.T) {
			hs := &http.Server{
				Addr: "127.0.0.2:2340",
				Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
					time.Sleep(20 * time.Millisecond)
				}),
			}
			s := HTTP("test", time.Millisecond, hs)
			testCheckOrderingEffects(t, s)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				resp, err := http.DefaultClient.Do(
					fun.Must(http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.2:2340/", nil)),
				)
				if err != nil {
					t.Error(err)
				}
				if err := resp.Body.Close(); err != nil {
					t.Log(err)
				}
			}()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
			s.Close()
			<-sig
			if err := s.Wait(); err == nil {
				t.Fatal("expected no error")
			}
		})
	})

}
