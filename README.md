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

## Usecases and Highlights

- Error Handling tools. The `erc.Collector` and `ers.Stack` types
  allow you to wrap, annotate, and aggregate errors. This makes
  continue-on-error patterns simple to implement. Because
  `erc.Collector` can be used concurrently, handling errors in Go
  routines, becomes much less of a headache. In addition the
  `ers.Error` type, as a string alias, permits `const` definition of
  errors.

- Service Management. `srv.Service` handles the lifecycle of
  "background" processes inside of an application. You can now start
  services like HTTP servers, background monitoring and workloads, and
  sub-processes, and ensure that they exit cleanly (at the right
  time!) and that their errors propagate clearly.

- Streams. The `fun.Stream[T]` type provides a stream/iterator,
  interface and tool kit for developing Go applications that lean
  heavily into streaming data and message-passing patterns. The
  associated `fun.Generator[T]` and `fun.Handler[T]` functions make
  provide a comfortable abstraction for interacting with these data.

- Pubsub provides queue and message broker primitives to support
  writing concurrent message passing tools within a single
  application, including a one-to-many or many-to-many communication.

- High-level data types: `dt` (data type) hosts a collection of
  wrappers and helpers, the `dt.Ring[T]` type provides a simple
  ring-buffer, in addition singally (`dt.Stack[T]`)and doubly linked
  lists (`dt.List[T]`). The `adt` package provides type-specific
  helpers and wrappers around Go's atomic/synchronization primitives,
  including the `adt.Pool[T]`, `adt.Map[T]` and `adt.Atomic[T]`
  types. Finally the `adt/shard.Map[T]` provides a logical thread-safe
  hash map, that is sharded across a collection of maps to reduce mutex
  contention.

## Packages

- [erc](https://pkg.go.dev/github.com/tychoish/fun/erc) (error
  collection utilites.)
- [ers](https://pkg.go.dev/github.com/tychoish/fun/erc) (error and
  panic handling utilites.)
- [dt](https://pkg.go.dev/github.com/tychoish/fun/dt) (generic
  container datatypes, including ordered and unordered sets, singly
  and doubly linked list, as well as wrappers around maps and slices.)
- [ft](https://pkg.go.dev/github.com/tychoish/fun/ft) function tools:
  simple tools for handling function objects.
- [adt](https://pkg.go.dev/github.com/tychoish/fun/adt) (strongly typed
  atomic data structures, wrappers, tools, and operations.)
- [shard](https://pkg.go.dev/github.com/tychoish/fun/adt/shard) is a
  "sharded map" to reduce contention for multi-threaded access of
  synchronized maps.
- [itertool](https://pkg.go.dev/github.com/tychoish/fun/itertool)
  (stream/iteration tools.)
- [pubsub](https://pkg.go.dev/github.com/tychoish/fun/pubsub) (message
  broker and concurrency-safe queue and deque.)
- [srv](https://pkg.go.dev/github.com/tychoish/fun/srv) (service
  orchestration and management framework.)
- [assert](https://pkg.go.dev/github.com/tychoish/fun/assert)
  (minimal generic-based assertion library, in the tradition of
  testify.) The assertions in `assert` abort the flow of the test while
  [check](https://pkg.go.dev/github.com/tychoish/fun/assert/check),
  provide non-critical assertions.
- [testt](https://pkg.go.dev/github.com/tychoish/fun/testt) (testy)
  are a collection of "nice to have" test helpers and utilities.
- [ensure](https://pkg.go.dev/github.com/tychoish/fun/ensure) is an
  experimental test harness and orchestration tool with more "natural"
  assertions.

For more information, see the documentation, but of general interest:

- The root `fun` package contains a few generic function types and
  with a collection of methods for interacting and managing and
  manipulating these operations. The `fun.Stream` provides a
  framework for interacting with sequences, including some powerful
  high-level parallel processing tools.

- In `itertools` and with `fun.Stream`, a stream framework and tools
  for interacting with streams (iterators) and generators.

- In `srv`, a service orchestration toolkit and lifecycle tools.

- In `ft` a number of low-level function-manipulation tools.

- In `pubsub`, a channel-based message broker (for one-to-many channel
  patterns), with several backend patterns for dealing with
  load-shedding and message distribution patterns.

- Queue and Deque implementations (in `pubsub`) that provide
  thread-safe linked-list based implementations and `Wait` methods to
  block until new items added.

- In `dt` a collection of data types and tools for manipulating
  different container types, as well as implementations of linked
  lists and sets. `dt` also provides an `fun`-idiomatic wrappers around
  generic slices and maps, which complement the tools in the `fun`
  package.

- In `adt`, a collection of Atomic/Pool/Map operations that use
  generics to provide strongly typed interfaces for common operations.

- In `erc`, an error collector implementation for threadsafe error
  aggregation and introspection, particularly in worker-pool,
  applications. `ers` provides related functionality.

- `fun` includes a number of light weight testing tools:
  - `assert` and `assert/check` provide "testify-style" assertions
	with more simple output, leveraging generics.
  - `testt` context, logging, and timer helpers for use in tests.
  - `ensure` and `ensure/is` a somewhat experimental "chain"-centered
	API for assertions.

## Contribution

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.
- minimal dependencies.

Please feel free to open issues or file pull requests.

Have fun!
