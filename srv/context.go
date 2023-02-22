// Package srv provides a framework and toolkit for service
// orchestration.
//
// srv contains three basic components: a Service type for managing
// the lifecylce of specific services, an Orchestrator for
// process-level management of services, and a collection of tools
package srv

import (
	"context"

	"github.com/tychoish/fun"
)

type (
	orchestratorContextKey struct{}
	shutdownTriggerCtxKey  struct{}
	baseContextContextKey  struct{}
)

// WithOrchestrator creates a new *Orchestrator, starts the associated
// service, and attaches it to the returned context. You should also,
// wait on the orchestrator's service to return before your process
// exits, as in:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	ctx = srv.WithOrchestrator(ctx)
//	defer srv.GetOrchestrator(ctx).Service().Wait()
//
// There are two flaws with this example: nothing calls cancel on the
// orchestrators context, and nothing observes the error from Wait().
// The base context passed to the orchestrator could easily be a
// singal.NotifyContext() so that the context is eventually canceled,
// or the caller should call cancel explicitly. The alternate
// implementation, that resolves these issues:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), )
//	defer cancel()
//	ctx = srv.WithOrchestrator(ctx)
//	defer fun.InvariantCheck(srv.GetOrchestrator(ctx).Service().Wait)
//
// InvariantCheck will run Wait at defer time, and raise an
// ErrInvariantViolation panic with the contents of Wait's error.
func WithOrchestrator(ctx context.Context) context.Context {
	orca := &Orchestrator{}
	ctx = SetOrchestrator(ctx, orca)

	s := orca.Service()

	fun.InvariantMust(s.Start(ctx), "orchestrator service must start")

	return ctx
}

// SetOrchestrator attaches an orchestrator to a context.
func SetOrchestrator(ctx context.Context, or *Orchestrator) context.Context {
	return context.WithValue(ctx, orchestratorContextKey{}, or)
}

// GetOrchestrator resolves the Orchestrator attached to the context or
// panics if there is no orchestrator attached to the context.
func GetOrchestrator(ctx context.Context) *Orchestrator {
	or, ok := ctx.Value(orchestratorContextKey{}).(*Orchestrator)

	fun.Invariant(ok, "orchestrator was not correctly attached")

	return or
}

// SetShutdown attaches a context.CancelFunc for the current context
// to that context, which can be accesed with the GetShutdown function
// to make it possible to trigger a safe and clean shutdown in
// functions that have access to the context but that.
//
// Because context's values work like a stack, it's possible to attach
// more than one shutdown to a single context chain, which would be
// very confusing and callers should avoid doing this.
func SetShutdown(ctx context.Context) context.Context {
	var cancel context.CancelFunc

	ctx, cancel = context.WithCancel(ctx)

	return context.WithValue(ctx, shutdownTriggerCtxKey{}, cancel)
}

// GetShutdown returns the previously attached cancellation function
// (via SetShutdown) for this context chain. To cancel all contexts
// that descend from the context created by SetShutdown callers must
// call the cancelfunction returned by GetShutdown.
//
// If a shutdown function was not set, GetShutdown panics with a
// fun.ErrInvariantViolation error.
func GetShutdown(ctx context.Context) context.CancelFunc {
	cancel, ok := ctx.Value(shutdownTriggerCtxKey{}).(context.CancelFunc)
	fun.Invariant(ok, "Shutdown cancel function was not correctly attached")

	return cancel
}

// SetBaseContext attaches the current context as an accessible value
// from the returned context. Once you attach core services to a base
// context (e.g. loggers, orchestrators, etc.) call SetBaseContext to
// make that context accessible later. This base context is useful for
// starting background services or dispatching other asynchronous work
// in the context of request driven work which can be canceled early.
//
// Because context's values work like a stack, it's possible to attach
// more than one base to a single context chain, which would be
// very confusing and callers should avoid doing this.
func SetBaseContext(ctx context.Context) context.Context {
	bctx := ctx
	return context.WithValue(ctx, baseContextContextKey{}, bctx)
}

// GetBaseContext gets a base context attached with SetBaseContext
// from the current context. If the base context is not attached,
// GetBaseContext panics with a fun.ErrInvariantViolation error.
//
// Use this context to start background services that should respect
// global shutdown and have access to the process' context, in the
// scope of a request that has a context that will be canceled early.
func GetBaseContext(ctx context.Context) context.Context {
	bctx, ok := ctx.Value(baseContextContextKey{}).(context.Context)

	fun.Invariant(ok, "base context was not correctly attached")

	return bctx
}
