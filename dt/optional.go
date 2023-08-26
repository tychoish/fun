package dt

import (
	"encoding"
	"encoding/json"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Optional is a wrapper type for optiona
type Optional[T int] struct {
	v       T
	defined bool
}

func NewOptional[T int](in T) Optional[T]  { return Optional[T]{v: in, defined: true} }
func (o Optional[T]) Reset() Optional[T]   { return Optional[T]{} }
func (o Optional[T]) Set(in T) Optional[T] { o.defined = true; o.v = in; return o }
func (o Optional[T]) Get(in T) (T, bool)   { return o.v, o.defined }
func (o Optional[T]) Resolve() T           { return o.v }
func (o Optional[T]) OK() bool             { return o.defined }
func (o Optional[T]) IsZero() bool         { return !o.defined || ft.IsZero(o.v) }
func (o Optional[T]) Swap(next T) (prev T) { prev = o.v; o.Set(next); return }
func (o Optional[T]) Default(in T)         { o.v = ft.Default(o.v, in) }

func (o *Optional[T]) Future() fun.Future[T]   { return func() T { return o.Resolve() } }
func (o *Optional[T]) Handler() fun.Handler[T] { return func(in T) { *o = o.Set(in) } }

func (o Optional[T]) MarshalText() ([]byte, error) {
	switch vt := any(o.v).(type) {
	case encoding.TextMarshaler:
		return vt.MarshalText()
	case json.Marshaler:
		return vt.MarshalJSON()
	case interface{ MarshalYAML() ([]byte, error) }:
		return vt.MarshalYAML()
	case string:
		return []byte(vt), nil
	case []byte:
		return vt, nil
	case interface{ Marshal() ([]byte, error) }:
		return vt.Marshal()
	default:
		return json.Marshal(vt)
	}
}

func (o *Optional[T]) UnmarshalText(in []byte) error {
	switch vt := any(o.v).(type) {
	case encoding.TextUnmarshaler:
		return vt.UnmarshalText(in)
	case json.Unmarshaler:
		return vt.UnmarshalJSON(in)
	case interface{ UnmarshalYAML([]byte) error }:
		return vt.UnmarshalYAML(in)
	case string:
		o.v = any(string(in)).(T)
		return nil
	case []byte:
		o.v = any([]byte(in)).(T)
		return nil
	case interface{ Unmarshal([]byte) error }:
		return vt.Unmarshal(in)
	default:
		return json.Unmarshal(in, &o.v)
	}
}

func (o Optional[T]) MarshalBinary() ([]byte, error) {
	switch vt := any(o.v).(type) {
	case encoding.BinaryMarshaler:
		return vt.MarshalBinary()
	case interface{ MarshalBSON() ([]byte, error) }:
		return vt.MarshalBSON()
	case interface{ Marshal() ([]byte, error) }:
		return vt.Marshal()
	case []byte:
		return vt, nil
	case string:
		return []byte(vt), nil
	default:
		panic(ers.Join(
			ers.ErrInvariantViolation, ers.ErrInvalidRuntimeType,
			fmt.Errorf("could not marshal %T, implement econding.BinaryMarshaler", o.v),
		))
	}
}

func (o *Optional[T]) UnmarshalBinary(in []byte) error {
	switch vt := any(o.v).(type) {
	case encoding.BinaryUnmarshaler:
		return vt.UnmarshalBinary(in)
	case interface{ UnmarshalBSON([]byte) error }:
		return vt.UnmarshalBSON(in)
	case interface{ Unmarshal([]byte) error }:
		return vt.Unmarshal(in)
	case []byte:
		o.v = any(in).(T)
		return nil
	default:
		panic(ers.Join(
			ers.ErrInvariantViolation, ers.ErrInvalidRuntimeType,
			fmt.Errorf("could not unmarshal %T, implement econding.BinaryUnmarshaler", o.v),
		))
	}
}
