package srv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
)

func TestImplementationHelpers(t *testing.T) {
	t.Run("Wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
		start := time.Now()
		if err := svc.Start(ctx); err != nil {
			t.Error(err)
		}

		if err := svc.Wait(); err != nil {
			t.Error(err)
		}

		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("RunCollect", func(t *testing.T) {
		t.Run("Happy", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			RunWaitCollect(ec, svc)(ctx)
			if err := ec.Resolve(); err != nil {
				t.Error(err)
			}
		})
		t.Run("AlreadyStarted", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			if err := svc.Start(ctx); err != nil {
				t.Error(err)
			}

			RunWaitCollect(ec, svc)(ctx)

			if err := ec.Resolve(); !errors.Is(err, ErrServiceAlreadyStarted) {
				t.Error(err)
			}
		})
	})
	t.Run("RunObserve", func(t *testing.T) {
		t.Run("Happy", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			RunWaitObserve(ec.Add, svc)(ctx)
			if err := ec.Resolve(); err != nil {
				t.Error(err)
			}
		})
		t.Run("AlreadyStarted", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			if err := svc.Start(ctx); err != nil {
				t.Error(err)
			}

			RunWaitObserve(ec.Add, svc)(ctx)

			if err := ec.Resolve(); !errors.Is(err, ErrServiceAlreadyStarted) {
				t.Error(err)
			}
		})
	})

	t.Run("RunWait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
		start := time.Now()
		RunWait(svc)(ctx)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Millisecond {
			t.Error(time.Since(start))
		}
	})

}
