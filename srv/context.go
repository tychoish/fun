// Package srv provides a framework and toolkit for service
// orchestration.
//
// srv contains three basic components: a Service type for managing
// the lifecylce of specific services, an Orchestrator for
// process-level management of services, and a collection of tools
package srv

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/risky"
)

type (
	baseContextCxtKey     struct{}
	orchestratorCtxKey    struct{}
	shutdownTriggerCtxKey struct{}
	cleanupCtxKey         struct{}
	workerPoolNameCtxKey  string
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
//	defer func()  { risky.Force(srv.GetOrchestrator(ctx).Service().Wait()) }()
//
// In this example, the wait will begin during shutdown and risky.Force
// will raise an ErrInvariantViolation panic with the contents of
// Wait's error.
//
// If an Orchestrator is already set on the context, this operation
// panics with an invariant violation.
func WithOrchestrator(ctx context.Context) context.Context {
	orca := &Orchestrator{}
	ctx = SetOrchestrator(ctx, orca)
	return ctx
}

// SetOrchestrator attaches an orchestrator to a context, if one is
// already set this is a panic with an invariant violation.
func SetOrchestrator(ctx context.Context, or *Orchestrator) context.Context {
	svc := or.Service()
	if !svc.Running() {
		fun.Invariant.Must(svc.Start(ctx), "orchestrator must start")
	}

	return context.WithValue(ctx, orchestratorCtxKey{}, or)
}

// GetOrchestrator resolves the Orchestrator attached to the context or
// panics if there is no orchestrator attached to the context.
func GetOrchestrator(ctx context.Context) *Orchestrator {
	or, ok := ctx.Value(orchestratorCtxKey{}).(*Orchestrator)

	fun.Invariant.IsTrue(ok, "orchestrator was not correctly attached")

	return or
}

// HasOrchestrator returns true if the orchestrator is attached to the
// configuration, and false otherwise.
func HasOrchestrator(ctx context.Context) bool {
	_, ok := ctx.Value(orchestratorCtxKey{}).(*Orchestrator)
	return ok
}

// HasCleanup returns true if a cleanup process is registered in the
// context.
func HasCleanup(ctx context.Context) bool {
	_, ok := ctx.Value(cleanupCtxKey{}).(*pubsub.Queue[fun.Worker])
	return ok
}

// WithCleanup adds a Cleanup service as created by the Cleanup()
// constructor, to an orchestrator attached to the context (or creates
// the orchestrator if needed,)
func WithCleanup(ctx context.Context) context.Context {
	if !HasOrchestrator(ctx) {
		ctx = WithOrchestrator(ctx)
	}
	pipe := pubsub.NewUnlimitedQueue[fun.Worker]()

	fun.Invariant.Must(GetOrchestrator(ctx).Add(Cleanup(pipe, 0)))

	return context.WithValue(ctx, cleanupCtxKey{}, pipe)
}

func getCleanup(ctx context.Context) *pubsub.Queue[fun.Worker] {
	val, ok := ctx.Value(cleanupCtxKey{}).(*pubsub.Queue[fun.Worker])
	fun.Invariant.IsTrue(ok, "cleanup service not configured")
	return val
}

// AddCleanup appends a cleanup function to the cleanup service
// pending in the context. Raises an invariant failure if the cleanup
// service was not previously configured, or if you attempt to add a
// new cleanup function while shutdown is running.
func AddCleanup(ctx context.Context, cleanup fun.Worker) {
	fun.Invariant.Must(getCleanup(ctx).Add(cleanup))
}

// AddCleanupError adds an error to the cleanup handler which is
// returned when that service shuts down. Useful to propagating errors
// encountered during runtime that don't warrent a panic to process
// shutdown.
func AddCleanupError(ctx context.Context, err error) {
	AddCleanup(ctx, func(context.Context) error { return err })
}

// SetShutdownSignal attaches a context.CancelFunc for the current context
// to that context, which can be accesed with the GetShutdown function
// to make it possible to trigger a safe and clean shutdown in
// functions that have access to the context but that.
//
// If a shutdown function is already set on this context, this
// operation is a noop.
func SetShutdownSignal(ctx context.Context) context.Context {
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

	fun.Invariant.IsTrue(ok, "Shutdown cancel function was not correctly attached")

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

	fun.Invariant.IsTrue(ok, "base context was not correctly attached")

	return bctx
}

// HasBaseContext returns true if a base context is already set, and
// false otherwise.
func HasBaseContext(ctx context.Context) bool {
	_, ok := ctx.Value(baseContextCxtKey{}).(context.Context)
	return ok
}

// WithWorkerPool setups a long running WorkerPool service, starts it,
// and attaches it to the returned context. The lifecycle of the
// WorkerPool service is managed by the orchestrator attached to this
// context: if not orchestrator is attached to the context, a new one
// is created and added to the context.
//
// In order to permit multiple parallel worker pools at a time
// attached to one context, specify an ID.
//
// The number of go routines servicing the work queue is determined by
// the options.NumWorkers: the minimum value is 1. Values less than
// one become one.
//
// The default queue created by WithWorkerPool has a flexible capped
// size that's roughly twice the number of active workers and a hard
// limit of 4 times the number of active workers. You can use
// SetWorkerPool to create an unbounded queue or a queue with
// different capacity limits.
//
// Be aware that with this pool, errors returned from worker functions
// remain in memory until they are returned when the service exits. In
// many cases keeping these errors is reasonable; however, for very
// long lived processes with a high error volume, this may not be
// workable. In these cases: avoid returning errors, collect
// and process them within your worker functions, or use an observer
// worker pool.
//
// Use AddToWorkerPool with the specified key to dispatch work to this
// worker pool.
func WithWorkerPool(
	ctx context.Context,
	key string,
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) context.Context {
	return SetWorkerPool(ctx, key, getQueueForOpts(optp...), optp...)
}

// WithObserverWorkerPool setups a long running WorkerPool service,
// starts it, and attaches it to the returned context. The lifecycle
// of the WorkerPool service is managed by the orchestrator attached
// to this context: if not orchestrator is attached to the context, a
// new one is created and added to the context.
//
// In order to permit multiple parallel worker pools at a time
// attached to one context, specify an ID.
//
// The number of go routines servicing the work queue is determined by
// the options.NumWorkers: the minimum value is 1. Values less than
// one become one.
//
// The default queue created by WithWorkerPool has a flexible capped
// size that's roughly twice the number of active workers and a hard
// limit of 4 times the number of active workers. You can use
// SetObserverWorkerPool to create an unbounded queue or a queue with
// different capacity limits.
//
// All errors encountered during the execution of worker functions,
// including panics, are passed to the observer function and are not
// retained.
func WithObserverWorkerPool(
	ctx context.Context,
	key string,
	observer fun.Observer[error],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) context.Context {
	return SetObserverWorkerPool(ctx, key, getQueueForOpts(optp...), observer, optp...)
}

func getQueueForOpts(optp ...fun.OptionProvider[*fun.WorkerGroupConf]) *pubsub.Queue[fun.Worker] {
	opts := &fun.WorkerGroupConf{}
	fun.Invariant.Must(fun.JoinOptionProviders(optp...).Apply(opts))

	return risky.Force(pubsub.NewQueue[fun.Worker](
		pubsub.QueueOptions{
			SoftQuota:   2 * opts.NumWorkers,
			HardLimit:   4 * opts.NumWorkers,
			BurstCredit: float64(opts.NumWorkers),
		}))
}

// SetWorkerPool constructs a WorkerPool based on the *pubsub.Queue
// provided. The lifecycle of the WorkerPool service is managed by
// the orchestrator attached to this context: if not orchestrator is
// attached to the context, a new one is created and added to the
// context.
//
// The number of go routines servicing the work queue is determined by
// the options.NumWorkers: the minimum value is 1. Values less than
// one become one.
//
// In order to permit multiple parallel worker pools at a time
// attached to one context, specify an ID.
//
// Use AddToWorkerPool with the specified key to dispatch work to this
// worker pool.
func SetWorkerPool(
	ctx context.Context,
	key string,
	queue *pubsub.Queue[fun.Worker],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) context.Context {
	return setupWorkerPool(ctx, key, queue, func(orca *Orchestrator) {
		fun.Invariant.Must(orca.Add(WorkerPool(queue, optp...)))
	})
}

// SetObserverWorkerPool constructs a WorkerPool based on the
// *pubsub.Queue provided. The lifecycle of the WorkerPool service is
// managed by the orchestrator attached to this context: if not
// orchestrator is attached to the context, a new one is created and
// added to the context.
//
// Errors produced by the workers, including captured panics where
// appropriate, are passed to the observer function and are not
// retained.
//
// The number of go routines servicing the work queue is determined by
// the options.NumWorkers: the minimum value is 1. Values less than
// one become one.
//
// In order to permit multiple parallel worker pools at a time
// attached to one context, specify an ID.
//
// Use AddToWorkerPool with the specified key to dispatch work to this
// worker pool.
func SetObserverWorkerPool(
	ctx context.Context,
	key string,
	queue *pubsub.Queue[fun.Worker],
	observer fun.Observer[error],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) context.Context {
	return setupWorkerPool(ctx, key, queue, func(orca *Orchestrator) {
		fun.Invariant.Must(orca.Add(ObserverWorkerPool(queue, observer, optp...)))
	})
}

func setupWorkerPool(ctx context.Context, key string, queue *pubsub.Queue[fun.Worker], attach func(*Orchestrator)) context.Context {
	if !HasOrchestrator(ctx) {
		ctx = WithOrchestrator(ctx)
	}
	attach(GetOrchestrator(ctx))
	return context.WithValue(ctx, workerPoolNameCtxKey(key), queue)
}

// AddToWorkerPool dispatches work to the WorkerPool's queue. If there
// is no worker pool attached with the given key, an error is
// returned. If the queue has been closed or the queue is full errors
// from the pubsub package are propagated to the caller.
//
// AddToWorkerPool will propagate worker functions to both
// conventional and obsesrver pools. Conventional pools will retain
// any error produced a worker function until the service exits, while
// observer pools pass errors to the observer function and then
// release them.
func AddToWorkerPool(ctx context.Context, key string, fn fun.Worker) error {
	queue, ok := ctx.Value(workerPoolNameCtxKey(key)).(*pubsub.Queue[fun.Worker])
	if !ok {
		return fmt.Errorf("worker pool named %q is not registered [%T]", key, ctx.Value(workerPoolNameCtxKey(key)))
	}
	if err := queue.Add(fn); err != nil {
		return fmt.Errorf("queue=%s: %w", key, err)
	}
	return nil
}
