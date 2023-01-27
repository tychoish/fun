# fun -- Go Generic Functions and Tools

[![Go Reference](https://pkg.go.dev/badge/github.com/tychoish/fun.svg)](https://pkg.go.dev/github.com/tychoish/fun)

``fun`` is a simple, well tested, zero-dependency, collection of
packages with generic function, tools, patterns, and the kind of thing
you *could* write one-offs for but shouldn't.

Packages:

- [erc](https://pkg.go.dev/github.com/tychoish/fun/erc) (error collecting)
- [itertool](https://pkg.go.dev/github.com/tychoish/fun/itertool) (iterator tools)
- [pubsub](https://pkg.go.dev/github.com/tychoish/fun/pubsub) (message broker
  and queues)
- [set](https://pkg.go.dev/github.com/tychoish/fun/set) (generic
  ordered and unordered sets)
- [seq](https://pkg.go.dev/github.com/tychoish/fun/seq) (generic
  linked lists.)

For more information, see the documentation, but of general interest:

- In `itertools` and with `fun.Iterator`, an iterator framework and
  tools for interacting with iterators and generators.
- In `pubsub`, a channel-based message broker (for one-to-many channel
  patterns), with several backend patterns for dealing with
  load-shedding and message distribution patterns.
- In `erc`, an error collector implementation for threadsafe error
  aggregation and introspection, particularly in worker-pool,
  applications.
- In `set`, a `Set` type, with ordered and unordered implementations.
- Queue and Deque implementations (in `pubsub`) that provide
  thread-safe linked-list based implementations and `Wait` methods to
  block until new items added.
- In `seq`, general purpose linked list implementations, with a
  healthy feature set and flexible interface.

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.

Have fun!
