# fun -- Core Library for Go Programming

[![Go Reference](https://pkg.go.dev/badge/github.com/tychoish/fun.svg)](https://pkg.go.dev/github.com/tychoish/fun)

`fun` is a simple, well-tested, zero-dependency "Core Library" for
Go. Think of it as a collection of the best parts of the `utils` package or `shared` or `common` packages in any given Go application, but with solid design, highly ergonomic interfaces, and thorough testing.

`fun` aims to be easy to adopt: the interfaces (and implementations!)
are simple and (hopefully!) easy to use. There are no external
dependencies and _all_ of the code is well-tested. You can (and
should!) always adopt the tools that make the most sense to use for
your project.

## Highlights

- Every **iterator** tool you always wish you had in the `irt` (ha!)
  package. Easily create, manipulate, and transform iterators, and
  avoid writing (and rewriting,) basic logic again and again.

- **Error tools**. `erc.Collector` is an error aggregator safe for
  concurrent access, with methods to collect errors conditionally (for
  validation), and annotating errors. The `ers` collection has a
  string-derived error type `ers.Error` so that you can have `const`
  errors.

- A set of **worker pools** helpers and tools in the `wpa` package for
  both ephemeral workloads as well as long-running
  pipelines. Particularly powerful in combination with the data types,
  iterator tools, and pubsub tooling.

- Powerful synchronization and **atomic** primitives, updated for the
  era of generics in `adt`:

  - `adt.Map[K,V]` means now you can use `sync.Map` without
    (terrifying) type casts everywhere.

  - `adt.Pool[T]` provides a type-safe `sync.Pool` with cleanup and
    constructor hooks.

  - `adt.Atomic[T]` is an atomic value container, that you can use to
    avoid littering your code with mutexes. (n.b. you _can_ store
    mutable atomic values in `adt.Atomic[T]` which are (of course) not
    safe for concurrent use, which is a bit of a gotcha, but for this
    `adt.Locked[T]` steps in to handle the mutex for you.

  - `adt.Once[T]` means you can get `sync.OnceFunc`, and friends, but
    as a type that you can add as a field to your on types for mutexes
    that you don't have to manage yourself.

  - within `adt`, `shard.Map[K,V]` provides a write-optimized, fully
    versioned map implementation that divides the keyspace among a
    number of constituent maps (shards) to reduce mutex contention for
    certain workloads.

- Delightful **data types** you always wish you had, in the `dt`
  package:

  - a ring buffer (`dt.Ring[T]`)
  - single and doubly linked lists (`dt.Stack[T]` and `dt.List[T]`)
  - sets (`dt.Set[K]` and `dt.OrderedSet[T]`)
  - an ordered map `dt.OrderedMap[K,V]` (also thread safe, and
    accessible via `adt`.)
  - histograms (in `dt/hdrhist`)
  - an "optional" wrapper (`dt.Optional[T]`) so you can avoid
    overloading pointer values in your type definitions.

- A collection of **function object tools**, wrappers for common
  function type in `fn` (without contexts) and `fnx` (with contexts),
  and operations in `ft`.

- Higher order **pubsub** tools, notably threadsafe queue and deque
  implementations--`pubsub.Queue[T]` and `pubsub.Deque[T]` which avoid
  channels entirely, and have support for unlimited, fixed-buffered,
  and burstable limits. Also a one-to-many or a many-to-many message
  broker, so you can have a "broadcast channel."

- **Service orchestration**. `srv.Service` handles the lifecycle of
  "background" processes inside of an application. You can now start
  services like HTTP servers, background monitoring and workloads, and
  sub-processes, and ensure that they exit cleanly (at the right
  time!) and that their errors propagate clearly back to the "main"
  thread.

- Lightweight **test infrastructure**: `assert` and `check`
  (testify-style assertions, but with generics,) testing tools and
  helpers in `testt` (testy!), and a fluent-style interface with
  `ensure`.

## Version History

There are no plans for a 1.0 release, though major backward-breaking
changes increment the second (major) release value, and limited
maintenance releases are possible/considered to maintain
compatibility.

- `v0.14.0`: go1.24 and greater. Re-focus API; new `irt` and `wpa`
  packages. Move all code out of the root package. (current)
- `v0.13.0`: go1.24 and greater. Reorganization of function API, and a
  bridge to the next major release. (experimental)
- `v0.12.0`: go1.24 and greater. Major API impacting
  release. (maintained)
- `v0.11.0`: go1.23 and greater.
- `v0.10.0`: go1.20 and greater.
- `v0.9.0`: go1.19 and greater.

There may be small changes to exported APIs within a release series,
although, major API changes will increment the major release value.

## Contribution

Contributions welcome, the general goals of the project:

- superior API ergonomics.
- great high-level abstractions.
- obvious and clear implementations.
- no external dependencies.

Please feel free to open issues or file pull requests.

Have fun!
