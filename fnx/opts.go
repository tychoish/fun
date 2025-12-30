package fnx

import (
	"github.com/tychoish/fun/erc"
)

// OptionProvider is a function type for building functional
// arguments, and is used for the parallel stream processing (map,
// transform, for-each, etc.) in the fun and itertool packages, and
// available with tooling for use in other contexts.
//
// The type T should always be mutable (e.g. a map, or a pointer).
type OptionProvider[T any] func(T) error

// JoinOptionProviders takes a zero or more option providers and
// produces a single combined option provider. With zero or nil
// arguments, the operation becomes a noop.
func JoinOptionProviders[T any](op ...OptionProvider[T]) OptionProvider[T] {
	var noop OptionProvider[T] = func(T) error { return nil }
	if len(op) == 0 {
		return noop
	}
	return noop.Join(op...)
}

// Apply applies the current Operation Provider to the configuration,
// and if the type T implements a Validate() method, calls that. All
// errors are aggregated.
func (op OptionProvider[T]) Apply(in T) error {
	err := op(in)
	switch validator := any(in).(type) {
	case interface{ Validate() error }:
		err = erc.Join(validator.Validate(), err)
	}
	return err
}

// Build processes a configuration object, returning a modified
// version (or a zero value, in the case of an error).
func (op OptionProvider[T]) Build(conf T) (out T, err error) {
	err = op.Apply(conf)
	if err != nil {
		return
	}
	return conf, nil
}

// Join aggregates a collection of Option Providers into a single
// option provider. The amalgamated operation is panic safe and omits
// all nil providers.
func (op OptionProvider[T]) Join(opps ...OptionProvider[T]) OptionProvider[T] {
	opps = append([]OptionProvider[T]{op}, opps...)
	return func(option T) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()
		for idx := range opps {
			if opps[idx] == nil {
				continue
			}
			err = erc.Join(opps[idx](option), err)
		}

		return err
	}
}
