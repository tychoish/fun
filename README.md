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
- [adt](https://pkg.go.dev/github.com/tychoish/fun/adt) (strongly typed
  atomic data structures, wrappers, tools, and operations.)
- [itertool](https://pkg.go.dev/github.com/tychoish/fun/itertool)
  (iterator tools.)
- [pubsub](https://pkg.go.dev/github.com/tychoish/fun/pubsub) (message
  broker and concurrency-safe queue and deque.)
- [srv](https://pkg.go.dev/github.com/tychoish/fun/srv) (service
  orchestration and management framework.)

For more information, see the documentation, but of general interest:

- In `itertools` and with `fun.Iterator`, an iterator framework and
  tools for interacting with iterators and generators.
- In `pubsub`, a channel-based message broker (for one-to-many channel
  patterns), with several backend patterns for dealing with
  load-shedding and message distribution patterns.
- In `erc`, an error collector implementation for threadsafe error
  aggregation and introspection, particularly in worker-pool,
  applications. `ers` provides related functionality.
- In `dt` a collection of data types and tools for manipulating
  different container types, as well as implementations of linked
  lists and sets.
- Queue and Deque implementations (in `pubsub`) that provide
  thread-safe linked-list based implementations and `Wait` methods to
  block until new items added.
- In `srv`, a service orchestration toolkit and lifecycle tools.
- In `adt`, a collection of Atomic/Pool/Map operations that use
  generics to provide strongly typed interfaces for common operations.

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.
- minimal dependencies.

Have fun!
