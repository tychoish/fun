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
	"github.com/tychoish/fun/pubsub"
)

type (
	baseContextCxtKey       struct{}
	orchestratorCtxKey      struct{}
	shutdownTriggerCtxKey   struct{}
	shutdownWaitQueueCtxKey struct{}
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
//	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
//	defer cancel()
//	ctx = srv.WithOrchestrator(ctx)
//	defer fun.InvariantCheck(srv.GetOrchestrator(ctx).Service().Wait)
//
// In this example, InvariantCheck will run Wait at defer time, and
// raise an ErrInvariantViolation panic with the contents of Wait's
// error.
//
// If an Orchestrator is already set on the context, this operation
// panics with an invariant violation.
func WithOrchestrator(ctx context.Context) context.Context {
	fun.Invariant(!HasOrchestrator(ctx), "cannot attach more than one orchestrator")

	orca := &Orchestrator{}
	ctx = SetOrchestrator(ctx, orca)

	fun.InvariantMust(orca.Service().Start(ctx), "orchestrator service must start")

	return ctx
}

// SetOrchestrator attaches an orchestrator to a context, if one is
// already set this is a panic with an invariant violation.
func SetOrchestrator(ctx context.Context, or *Orchestrator) context.Context {
	fun.Invariant(!HasOrchestrator(ctx), "cannot attach more than one orchestrator")

	return context.WithValue(ctx, orchestratorCtxKey{}, or)
}

// GetOrchestrator resolves the Orchestrator attached to the context or
// panics if there is no orchestrator attached to the context.
func GetOrchestrator(ctx context.Context) *Orchestrator {
	or, ok := ctx.Value(orchestratorCtxKey{}).(*Orchestrator)

	fun.Invariant(ok, "orchestrator was not correctly attached")

	return or
}

// HasOrchestrator returns true if the orchestrator is attached to the
// configuration, and false otherwise.
func HasOrchestrator(ctx context.Context) bool {
	_, ok := ctx.Value(orchestratorCtxKey{}).(*Orchestrator)
	return ok
}

// SetShutdownSignal attaches a context.CancelFunc for the current context
// to that context, which can be accesed with the GetShutdown function
// to make it possible to trigger a safe and clean shutdown in
// functions that have access to the context but that.
//
// If a shutdown function is already set on this context, this
// operation is a noop.
func SetShutdownSignal(ctx context.Context) context.Context {
	fun.Invariant(!HasShutdownSignal(ctx), "cannot attach more than one shutdown")

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	return context.WithValue(ctx, shutdownTriggerCtxKey{}, cancel)
}

// GetShutdownSignal returns the previously attached cancellation function
// (via SetShutdown) for this context chain. To cancel all contexts
// that descend from the context created by SetShutdown callers must
// call the cancelfunction returned by GetShutdownSignal.
//
// If a shutdown function was not set, GetShutdownSignal panics with a
// fun.ErrInvariantViolation error.
func GetShutdownSignal(ctx context.Context) context.CancelFunc {
	cancel, ok := ctx.Value(shutdownTriggerCtxKey{}).(context.CancelFunc)

	fun.Invariant(ok, "Shutdown cancel function was not correctly attached")

	return cancel
}

// HasShutdownSignal returns true if a shutdown has already set and false
// otherwise.
func HasShutdownSignal(ctx context.Context) bool {
	_, ok := ctx.Value(shutdownTriggerCtxKey{}).(context.CancelFunc)
	return ok
}

// SetBaseContext attaches the current context as an accessible value
// from the returned context. Once you attach core services to a base
// context (e.g. loggers, orchestrators, etc.) call SetBaseContext to
// make that context accessible later. This base context is useful for
// starting background services or dispatching other asynchronous work
// in the context of request driven work which can be canceled early.
//
// If a base context is already set on this context, this operation
// panics with an invariant violation.
func SetBaseContext(ctx context.Context) context.Context {
	fun.Invariant(!HasBaseContext(ctx), "cannot set more than one base context.")

	bctx := ctx

	return context.WithValue(ctx, baseContextCxtKey{}, bctx)
}

// GetBaseContext gets a base context attached with SetBaseContext
// from the current context. If the base context is not attached,
// GetBaseContext panics with a fun.ErrInvariantViolation error.
//
// Use this context to start background services that should respect
// global shutdown and have access to the process' context, in the
// scope of a request that has a context that will be canceled early.
func GetBaseContext(ctx context.Context) context.Context {
	bctx, ok := ctx.Value(baseContextCxtKey{}).(context.Context)

	fun.Invariant(ok, "base context was not correctly attached")

	return bctx
}

// HasBaseContext returns true if a base context is already set, and
// false otherwise.
func HasBaseContext(ctx context.Context) bool {
	_, ok := ctx.Value(baseContextCxtKey{}).(context.Context)
	return ok
}

// WithShutdownManager constructs and attaches a wait service, which
// waits until all submitted wait functions have returned.
//
// If a shutdown manager is already set on this context, this
// operation is a noop.
//
// If a shutdown trigger (e.g. SetShutdown/GetShutdown) is not set on
// the provided context or an orchestrator, WithShutdownManager will
// attach these services to the cotnext before creating and adding a
// shutdown manager to this context.
func WithShutdownManager(ctx context.Context) context.Context {
	fun.Invariant(!HasShutdownManager(ctx), "cannot add more than one shutdown queue")

	if !HasOrchestrator(ctx) {
		ctx = WithOrchestrator(ctx)
	}

	queue := pubsub.DistributorChannel(make(chan fun.WaitFunc))

	orca := GetOrchestrator(ctx)
	s := Wait(pubsub.DistributorIterator(queue))

	fun.InvariantMust(s.Start(ctx), "wait service must start")
	fun.InvariantMust(orca.Add(s), "problem adding wait service to orchestrator")

	return context.WithValue(ctx, shutdownWaitQueueCtxKey{}, queue)
}

func getShutdownManager(ctx context.Context) pubsub.Distributor[fun.WaitFunc] {
	q, ok := ctx.Value(shutdownWaitQueueCtxKey{}).(pubsub.Distributor[fun.WaitFunc])
	fun.Invariant(ok, "shutdown queue was not correctly attached")
	return q
}

// AddToShutdownManager adds a wait function to the shutdown queue. If a
// shutdown manager is not set on the context, this raises a panic
// with an InvariantViolation.
func AddToShutdownManager(ctx context.Context, fn fun.WaitFunc) {
	fun.InvariantMust(getShutdownManager(ctx).Send(ctx, fn), "problem adding wait function to shutdown queue")
}

// HasShutdownManager returns true if a shutdown queue has started.
func HasShutdownManager(ctx context.Context) bool {
	_, ok := ctx.Value(shutdownWaitQueueCtxKey{}).(pubsub.Distributor[fun.WaitFunc])
	return ok
}
