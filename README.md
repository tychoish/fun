# fun -- Go Generic Functions and Tools

[![Go Reference](https://pkg.go.dev/badge/github.com/tychoish/fun.svg)](https://pkg.go.dev/github.com/tychoish/fun)

``fun`` is a simple, well tested, zero-dependency, collection of
packages with generic function, tools, patterns, and the kind of thing
you *could* write one-offs for but shouldn't.

Packages:

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
  (iterator tools.)
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
  manipulating these operations. The `fun.Iterator` provides a
  framework for interacting with sequences, including some powerful
  high-level parallel processing tools.
- In `itertools` and with `fun.Iterator`, an iterator framework and
  tools for interacting with iterators and generators.
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

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.
- minimal dependencies.

Have fun!
