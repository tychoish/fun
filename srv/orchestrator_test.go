package srv

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

func TestOrchestrator(t *testing.T) {
	t.Parallel()
	t.Run("Stringer", func(t *testing.T) {
		orc := &Orchestrator{}
		if orc.String() != "Orchestrator<>" {
			t.Error(orc.String())
		}
		orc.Name = "funtime"
		if orc.String() != "Orchestrator<funtime>" {
			t.Error(orc.String())
		}
	})
	t.Run("Add", func(t *testing.T) {
		t.Run("BeforeStart", func(t *testing.T) {
			counter := &atomic.Int64{}
			orc := &Orchestrator{}
			for i := 0; i < 100; i++ {
				if err := orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						counter.Add(1)
						return nil
					},
				}); err != nil {
					t.Error(err)
				}
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
				if err := orc.Add(nil); err != nil {
					t.Error(err)
				}
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			wg := &fun.WaitGroup{}
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
			wg.Wait(ctx)
		})
		t.Run("RunEndToEnd", func(t *testing.T) {
			counter := &atomic.Int64{}
			orc := &Orchestrator{}
			wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				if err := orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						counter.Add(1)
						wg.Done()
						return nil
					},
				}); err != nil {
					t.Error(err)
				}
			}
			s := orc.Service()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait(ctx)
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
				if err := orc.Add(nil); err != nil {
					t.Error(err)
				}
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
			wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				if err := orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						defer wg.Done()
						return errors.New("42")
					},
				}); err != nil {
					t.Error(err)
				}
			}
			s := orc.Service()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait(ctx)
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
				if err := s.Start(ctx); err != nil {
					t.Fatal(err)
				}
				if err := orc.Add(s); err != nil {
					t.Fatal(err)
				}
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
			const num = 25
			for i := 0; i < num; i++ {
				if err := orc.Add(s); err != nil {
					t.Fatal(err)
				}
			}
			if !osrv.Running() {
				t.Error("should still be running")
			}

			time.Sleep(500 * time.Millisecond)

			osrv.Close()
			err := osrv.Wait()
			if err == nil {
				t.Fatal("should error")
			}
			if osrv.Running() {
				t.Error("should not be running")
			}
			errs := erc.Unwind(err)
			if orc.pipe.Len() != num {
				t.Error(orc.pipe.Len(), num)
			}
			if len(errs) == 0 {
				t.Error("should have errors")
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
				if err := orc.Add(&Service{Name: fmt.Sprint(i)}); err != nil {
					t.Fatal(err)
				}
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
			wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				if err := orc.Add(makeBlockingService(t)); err != nil {
					t.Error(err)
				}
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
	t.Run("ServicePassthrough", func(t *testing.T) {
		t.Run("RunEndToEnd", func(t *testing.T) {
			t.Parallel()
			counter := &atomic.Int64{}
			orc := &Orchestrator{}
			wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				if err := orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						counter.Add(1)
						wg.Done()
						return nil
					},
				}); err != nil {
					t.Error(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait(ctx)
			// close wrapper service
			orc.Service().Close()
			if err := orc.Wait(); err != nil {
				t.Fatal(err)
			}
			if counter.Load() != 100 {
				t.Error("all services did not run")
			}
		})
		t.Run("PropogateErrors", func(t *testing.T) {
			t.Parallel()
			orc := &Orchestrator{}
			wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				if err := orc.Add(&Service{
					Name: fmt.Sprint(i),
					Run: func(ctx context.Context) error {
						defer wg.Done()
						return errors.New("42")
					},
				}); err != nil {
					t.Error(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			// wait for all services to return
			wg.Wait(ctx)
			// close wrapper service
			orc.Service().Close()

			err := orc.Wait()
			if err == nil {
				t.Fatal("should have errors")
			}
			if errs := erc.Unwind(err); len(errs) != 100 {
				t.Fatal(len(errs))
			}
		})
		t.Run("StartRunningServices", func(t *testing.T) {
			t.Parallel()
			orc := &Orchestrator{}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			for i := 0; i < 100; i++ {
				s := makeBlockingService(t)
				if err := s.Start(ctx); err != nil {
					t.Fatal(err)
				}
				if err := orc.Add(s); err != nil {
					t.Fatal(err)
				}
			}
			startAt := time.Now()
			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := orc.Wait(); err != nil {
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
				t.Fatal(err)
			}
			if err := s.Wait(); err != nil {
				t.Fatal(err)
			}
			orc := &Orchestrator{}
			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			s.Close()
			_ = s.Wait()
			if orc.pipe == nil {
				t.Fatal("should not be empty")
			}
			runtime.Gosched()
			if s.Running() {
				t.Error("running and should be stoped")
			}
			const num = 100
			for i := 0; i < num; i++ {
				if err := orc.Add(s); err != nil {
					t.Error(err)
				}
			}
			if !orc.Service().Running() {
				t.Error("should still be running")
			}

			if orc.pipe.Len() != num {
				t.Error("should have services", orc.pipe.Len())
			}

			orc.Service().Close()

			time.Sleep(500 * time.Millisecond)

			if orc.Service().Running() {
				t.Error("should not be running")
			}
			err := orc.Wait()
			errs := erc.Unwind(err)
			if len(errs) == 0 {
				t.Log(err, errs, len(errs))
				t.Error("should have errors")
			}
		})
		t.Run("PanicSafely", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			orc := &Orchestrator{}
			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				if err := orc.Add(&Service{Name: fmt.Sprint(i)}); err != nil {
					t.Fatal(err)
				}
			}

			time.Sleep(5 * time.Millisecond)

			orc.Service().Close()
			err := orc.Wait()
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
			t.Parallel()
			orc := &Orchestrator{}
			// wg := &fun.WaitGroup{}
			for i := 0; i < 100; i++ {
				// wg.Add(1)
				if err := orc.Add(makeBlockingService(t)); err != nil {
					t.Error(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			startAt := time.Now()
			if err := orc.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := orc.Wait(); err != nil {
				t.Error(err)
			}
			// the fixture ensures that all sub-services run
			if dur := time.Since(startAt); dur > 105*time.Millisecond {
				t.Error(dur)
			}
		})
	})
}
