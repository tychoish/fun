package srv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/seq"
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
			timer := time.NewTimer(time.Hour)
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

func TestContext(t *testing.T) {
	t.Run("From", func(t *testing.T) {
		t.Run("Panics", func(t *testing.T) {
			ec := &erc.Collector{}
			func() {
				defer erc.Recover(ec)
				FromContext(context.Background())
			}()
			err := ec.Resolve()
			if err == nil {
				t.Error("should have panic'd")
			}
			if err.Error() != "panic: nil orchestrator" {
				t.Error("wrong panic value")
			}
		})
		t.Run("Fetch", func(t *testing.T) {
			orc := &Orchestrator{}

			ctx := context.Background()

			nctx := WithContext(ctx, orc)

			or := FromContext(nctx)
			if or == nil {
				t.Error("should not be nil")
			}
			if or != orc {
				t.Error("should be the same orchestrator")
			}
		})
	})
	t.Run("With", func(t *testing.T) {
		orc := &Orchestrator{}

		ctx := context.Background()

		nctx := WithContext(ctx, orc)
		if nctx == ctx {
			t.Error("should have replaced context")
		}

		nnctx := WithContext(nctx, orc)
		if nnctx != nctx {
			t.Error("should not replace context")
		}
		t.Run("Duplicate", func(t *testing.T) {
			ec := &erc.Collector{}
			func() {
				defer erc.Recover(ec)
				_ = WithContext(nctx, &Orchestrator{})
			}()
			err := ec.Resolve()
			if err == nil {
				t.Error("should have panic'd")
			}
			if err.Error() != "panic: multiple orchestrators" {
				t.Error("wrong panic value")
			}

		})
	})
}

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
				Run: func(context.Context) error { panic("whoops"); return nil },
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				resp, err := http.DefaultClient.Get("http://127.0.0.2:2340/")
				if err != nil {
					t.Log(resp)
					t.Error(err)
				}
			}()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Millisecond)
			s.Close()
			if err := s.Wait(); err == nil {
				t.Fatal("expected no error")
			}
		})
	})

}

func TestOrchestrator(t *testing.T) {
	t.Parallel()
	t.Run("Add", func(t *testing.T) {
		t.Run("BeforeStart", func(t *testing.T) {
			counter := &atomic.Int64{}
			orc := &Orchestrator{}
			for i := 0; i < 100; i++ {
				orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						counter.Add(1)
						return nil
					},
				})
			}
			if counter.Load() != 0 {
				t.Error(counter.Load())
			}
			if orc.srv != nil {
				t.Error("service is created lazily, later")
			}
			if orc.pipe == nil {
				t.Error("pipe should be created")
			}
		})
		t.Run("NilServices", func(t *testing.T) {
			orc := &Orchestrator{}
			for i := 0; i < 100; i++ {
				orc.Add(nil)
			}
			if orc.pipe == nil {
				t.Error("pipe should be created")
			}
			if orc.pipe.Len() != 0 {
				t.Error(orc.pipe.Len())
			}
		})
	})
	t.Run("Service", func(t *testing.T) {
		t.Run("MultipleCalls", func(t *testing.T) {
			orc := &Orchestrator{}
			if orc.srv != nil {
				t.Error("service is created lazily, later")
			}

			s := orc.Service()
			if s == nil {
				t.Fatal("service should always be produced")
			}
			if orc.srv == nil {
				t.Error("service is created lazily, by now")
			}
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					time.Sleep(time.Duration(id*10) * time.Millisecond)
					s2 := orc.Service()
					if s != s2 {
						t.Error("service is cached and the same", id)
					}
				}(i)
			}
			wg.Wait()
		})
		t.Run("RunEndToEnd", func(t *testing.T) {
			counter := &atomic.Int64{}
			orc := &Orchestrator{}
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						counter.Add(1)
						wg.Done()
						return nil
					},
				})
			}
			s := orc.Service()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait()
			// close wrapper service
			s.Close()
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
			if counter.Load() != 100 {
				t.Error("all services did not run")
			}
			t.Run("Reset", func(t *testing.T) {
				if orc.pipe.Len() != 100 {
					t.Error(orc.pipe.Len())
				}
				orc.Add(nil)
				if orc.pipe.Len() != 0 {
					t.Error(orc.pipe.Len())
				}
				ns := orc.Service()
				if ns == nil {
					t.Error("should be a valid service")
				}
				if ns == s {
					t.Error("should be a different service")
				}
			})
		})
		t.Run("PropogateErrors", func(t *testing.T) {
			orc := &Orchestrator{}
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						defer wg.Done()
						return errors.New("42")
					},
				})
			}
			s := orc.Service()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait()
			// close wrapper service
			s.Close()

			err := s.Wait()
			if err == nil {
				t.Fatal("should have errors")
			}
			if errs := erc.Unwind(err); len(errs) != 100 {
				t.Fatal(len(errs))
			}
		})
		t.Run("StartRunningServices", func(t *testing.T) {
			orc := &Orchestrator{}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			for i := 0; i < 100; i++ {
				s := makeBlockingService(t)
				s.Start(ctx)
				orc.Add(s)
			}
			startAt := time.Now()
			s := orc.Service()
			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Wait(); err != nil {
				t.Error(err)
			}
			// the fixture ensures that all sub-services run
			if dur := time.Since(startAt); dur > 20*time.Millisecond {
				t.Error(dur)
			}
		})
		t.Run("FinishedServicesError", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			s := &Service{Name: "base", Run: func(context.Context) error { return nil }}
			if err := s.Start(ctx); err != nil {
				t.Fatal()
			}
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
			orc := &Orchestrator{}
			osrv := orc.Service()
			if err := osrv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				orc.Add(s)
			}
			if !osrv.Running() {
				t.Error("should still be running")
			}

			time.Sleep(5 * time.Millisecond)

			osrv.Close()
			err := osrv.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if osrv.Running() {
				t.Error("should not be running")
			}
			errs := erc.Unwind(err)
			if len(errs) != 100 {
				t.Log(errs)
				t.Error(len(errs))
			}
		})
		t.Run("PanicSafely", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			orc := &Orchestrator{}
			osrv := orc.Service()
			if err := osrv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				orc.Add(&Service{Name: fmt.Sprint(i)})
			}

			time.Sleep(5 * time.Millisecond)

			osrv.Close()
			err := osrv.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			errs := erc.Unwind(err)
			if len(errs) != 100 {
				t.Log(errs)
				t.Error(len(errs))
			}
		})
		t.Run("LogRunningServices", func(t *testing.T) {
			orc := &Orchestrator{}
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				orc.Add(makeBlockingService(t))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			startAt := time.Now()
			s := orc.Service()
			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Wait(); err != nil {
				t.Error(err)
			}
			// the fixture ensures that all sub-services run
			if dur := time.Since(startAt); dur > 20*time.Millisecond {
				t.Error(dur)
			}
		})
	})
}
