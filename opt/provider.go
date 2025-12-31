package opt

import (
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
)

// Provider is a function type for building functional
// arguments, and is used for the parallel stream processing (map,
// transform, for-each, etc.) in the fun and itertool packages, and
// available with tooling for use in other contexts.
//
// The type T should always be mutable (e.g. a map, or a pointer).
type Provider[T any] func(T) error

// New constructs a new Provider, essentially for type casting purposes.
func New[T any](in func(T) error) Provider[T] { return in }

// Join takes a zero or more providers and
// produces a single combined provider. With zero or nil
// arguments, the operation becomes a noop.
func Join[T any](op ...Provider[T]) Provider[T] {
	var noop Provider[T] = func(T) error { return nil }
	if len(op) == 0 {
		return noop
	}
	return noop.Join(op...)
}

// Apply applies the current Operation Provider to the configuration,
// and if the type T implements a Validate() method, calls that. All
// errors are aggregated.
func (op Provider[T]) Apply(in T) (err error) {
	defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()

	err = op(in)

	switch validator := any(in).(type) {
	case interface{ Validate() error }:
		return erc.Join(validator.Validate(), err)
	default:
		return err
	}
}

// Build processes a configuration object, returning a modified
// version (or a zero value, in the case of an error).
func (op Provider[T]) Build(conf T) (out T, err error) {
	err = op.Apply(conf)
	if err != nil {
		return
	}
	return conf, nil
}

// Join aggregates a collection of Option Providers into a single
// option provider. The amalgamated operation is panic safe and omits
// all nil providers.
func (op Provider[T]) Join(opps ...Provider[T]) Provider[T] {
	return func(conf T) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()

		for ops := range irt.Slice(append([]Provider[T]{op}, opps...)) {
			if ops == nil {
				continue
			}
			err = erc.Join(ops(conf), err)
		}

		return err
	}
}
