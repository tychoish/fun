# fun -- Core Library for Go Programming

[![Go Reference](https://pkg.go.dev/badge/github.com/tychoish/fun.svg)](https://pkg.go.dev/github.com/tychoish/fun)

`fun` is a simple, well-tested, zero-dependency "Core Library" for
Go, with support for common patterns and paradigms. Stream processing,
error handling, pubsub/message queues, and service
architectures. If you've maintained a piece of Go software, you've
probably written one-off versions of some of these tools: let's avoid
needing to _keep_ writing these tools.

`fun` aims to be easy to adopt: the interfaces (and implementations!)
are simple and (hopefully!) easy to use. There are no external
dependencies and _all_ of the code is well-tested. You can (and
should!) always adopt the tools that make the most sense to use for
your project.

## Use Cases and Highlights

- Error Handling tools. The `erc.Collector` type allow you to wrap,
  annotate, and aggregate errors. This makes continue-on-error
  patterns simple to implement. Because `erc.Collector` can be used
  concurrently, handling errors in Go routines, becomes much less of a
  headache. In addition, the `ers.Error` type, as a string alias,
  permits `const` definition of errors.

- Service Management. `srv.Service` handles the lifecycle of
  "background" processes inside of an application. You can now start
  services like HTTP servers, background monitoring and workloads, and
  sub-processes, and ensure that they exit cleanly (at the right
  time!) and that their errors propagate clearly back to the "main"
  thread.

- Streams. The `fun.Stream[T]` type provides a stream/iterator,
  interface and tool kit for developing Go applications that lean
  heavily into streaming data and message-passing patterns. The
  associated `fun.Generator[T]` and `fun.Handler[T]` functions
  provide a comfortable abstraction for interacting with these data.

- Pubsub. The `pubsub` package provides queue and message broker
  primitives for concurrent applications to support writing
  applications that use message-passing patterns inside of single
  application. The `pubsub.Broker[T]` provides one-to-many or
  many-to-many (e.g. broadcast) communication. The and deque and queue
  structures provide configurable queue abstractions to provide
  control for many workloads.

- High-level data types: `dt` (data type) hosts a collection of
  wrappers and helpers, the `dt.Ring[T]` type provides a simple
  ring-buffer, in addition singl-ly (`dt.Stack[T]`)and doubly linked
  lists (`dt.List[T]`). The `adt` package provides type-specific
  helpers and wrappers around Go's atomic/synchronization primitives,
  including the `adt.Pool[T]`, `adt.Map[T]` and `adt.Atomic[T]`
  types. Finally the `adt/shard.Map[T]` provides a logical thread-safe
  hash map, that is sharded across a collection of maps to reduce mutex
  contention.

## Packages

- Data Types and Function Tools:
  - [dt](https://pkg.go.dev/github.com/tychoish/fun/dt) (generic
    container data-types, including ordered and unordered sets, singly
    and doubly linked list, as well as wrappers around maps and slices.)
  - [adt](https://pkg.go.dev/github.com/tychoish/fun/adt) (strongly typed
    atomic data structures, wrappers, tools, and operations.)
  - [shard](https://pkg.go.dev/github.com/tychoish/fun/adt/shard) (a
    "sharded map" to reduce contention for multi-threaded access of
    synchronized maps.)
  - [ft](https://pkg.go.dev/github.com/tychoish/fun/ft) function tools:
    simple tools for handling function objects.
  - [itertool](https://pkg.go.dev/github.com/tychoish/fun/itertool)
    (stream/iteration tools and helpers.)
- Service Architecture:
  - [srv](https://pkg.go.dev/github.com/tychoish/fun/srv) (service
    orchestration and management framework.)
  - [pubsub](https://pkg.go.dev/github.com/tychoish/fun/pubsub) (message
	broker and concurrency-safe queue and deque.)
- Error Handling:
  - [erc](https://pkg.go.dev/github.com/tychoish/fun/erc) (error
    collection, annotation, panic handling utilities for aggregating
    errors and managing panics, in concurrent contexts.)
  - [ers](https://pkg.go.dev/github.com/tychoish/fun/erc) (constant
    errors and low level error primitives used throughout the package.)
- Test Infrastructure:
  - [assert](https://pkg.go.dev/github.com/tychoish/fun/assert)
	(minimal generic-based assertion library, in the tradition of
	testify.) The assertions in `assert` abort the flow of the test
	while check](https://pkg.go.dev/github.com/tychoish/fun/assert/check),
	provide non-critical assertions.
  - [testt](https://pkg.go.dev/github.com/tychoish/fun/testt) (testy;
    a collection of "nice to have" test helpers and utilities.)
  - [ensure](https://pkg.go.dev/github.com/tychoish/fun/ensure) (an
    experimental test harness and orchestration tool with more
    "natural" assertions.)

## Examples

(coming soon.)

## Version History

There are no plans for a 1.0 release, though major backward-breaking
changes increment the second (major) release value, and limited
maintenance releases are possible/considered to maintain
compatibility.

- `v0.12.0`: go1.24 and greater. Major API impacting
  release. (current)
- `v0.11.0`: go1.23 and greater. (maintained)
- `v0.10.0`: go1.20 and greater. (maintained)
- `v0.9.0`: go1.19 and greater.

There may be small changes to exported APIs within a release series,
although, major API changes will increment the major release value.

## Contribution

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.
- minimal dependencies.

Please feel free to open issues or file pull requests.

Have fun!
