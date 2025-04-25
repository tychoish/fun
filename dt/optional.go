package dt

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

// Optional is a wrapper type for optional value, where using a
// pointer is unsuitable or awkward. The type provides a reasonable
// interface for manipulating the value, does not need to be
// initialized upon construction (i.e. safe to put in structs without
// needing to specify an initial value.) MarshalText and MarshalBinary
// (with corresponding) unmarshal) interfaces make Optional types easy
// to embed.
//
// Marshal/Unmarshal methods are provided to support using optional in
// primary structs and to avoid the need to duplicate structs.
type Optional[T any] struct {
	v       T
	defined bool
}

// NewOptional is simple constructor that constructs a new populated
// Optional value.
func NewOptional[T any](in T) Optional[T] { return Optional[T]{v: in, defined: true} }

// Default sets value of the optional to the provided value if it is
// not already been defined.
func (o *Optional[T]) Default(in T) { ft.WhenApply(!o.Ok(), o.Set, in) }

// DefaultFuture resolves the future if the optional has not yet been
// set.
func (o *Optional[T]) DefaultFuture(in fn.Future[T]) { ft.WhenApplyFuture(!o.Ok(), o.Set, in) }

// Set marks the optional value as defined, and sets the optional
// value. You can set an optional to the zero value for type T. To
// "unset" a value use the Reset().
func (o *Optional[T]) Set(in T) { o.defined = true; o.v = in }

// Resolve returns the current value of the optional. Zero values of T
// are ambiguous.
func (o *Optional[T]) Resolve() T { return o.v }

// Reset unsets the OK value  of the optional, and unsets the
// reference to the existing value.
func (o *Optional[T]) Reset() { o.defined = false; var zero T; o.v = zero }

// Swap returns the previous value of the optional and replaces it
// with the provided value
func (o *Optional[T]) Swap(next T) (prev T) { prev = o.v; o.Set(next); return }

// Get returns the current value of the optional and the ok value. Use
// this to disambiguate zero values.
func (o *Optional[T]) Get() (T, bool) { return o.v, o.defined }

// Future provides access to the value of the optional as a Future
// function. This does not disambiguate zero values. Use in
// conjunction with Optional.Handler and adt.AccessorsWithLock and
// adt.AccessorsWithReadLock to handle concurency control.
func (o *Optional[T]) Future() fn.Future[T] { return o.Resolve }

// Handler provides access to setting the optional value as
// fn.Handler function. Use in conjunction with Optional.Future and
// adt.AccessorsWithLock and adt.AccessorsWithReadLock to handle
// concurency control.
func (o *Optional[T]) Handler() fn.Handler[T] { return o.Set }

// Ok returns true when the optional
func (o Optional[T]) Ok() bool { return o.defined }

// Scan implements the sql.Scanner interface. This is invalid if the
// type of the optional value is not a primitive value type.
func (o *Optional[T]) Scan(src any) (err error) {
	if src == nil {
		var zero T
		o.defined = false
		o.v = zero
		return nil
	}
	o.init()

	defer func() { o.defined = (err == nil && ft.Not(ft.IsNil(o.v))) }()

	if ft.IsType[int](src) && ft.IsType[int64](o.v) {
		o.v = any(int64(src.(int))).(T)
		return nil
	}
	if val, ok := ft.Cast[T](src); ok {
		o.v = val
		return nil
	}
	if ft.IsType[sql.Scanner](o.v) {
		o.init()
		return any(o.v).(sql.Scanner).Scan(src)
	}

	switch val := src.(type) {
	case string:
		return o.UnmarshalText([]byte(val))
	case []byte:
		return o.UnmarshalBinary(val)
	default:
		return erc.Join(ers.ErrInvalidRuntimeType, ers.ErrInvalidInput,
			fmt.Errorf("%T can not be the value for Optional[%T]", src, o.v))
	}
}

func (o *Optional[T]) init() {
	if ft.IsNil(o.v) && ft.IsPtr(o.v) {
		o.v = reflect.New(reflect.ValueOf(any(o.v)).Type().Elem()).Interface().(T)
	}
}

// Value implements the SQL driver.Valuer interface. Scan implements
// the sql Scanner interface. This is invalid if the type of the
// optional value is not a primitive value type.
func (o Optional[T]) Value() (driver.Value, error) {
	if !o.defined || ft.IsNil(o.v) {
		return nil, nil
	}

	switch val := any(o.v).(type) {
	case int64, float64, bool, time.Time, string, []byte:
		return val, nil
	case int:
		return int64(val), nil
	case driver.Valuer:
		return val.Value()
	case encoding.TextMarshaler:
		return val.MarshalText()
	case encoding.BinaryMarshaler:
		return val.MarshalBinary()
	default:
		return nil, erc.Join(ers.ErrInvalidRuntimeType, ers.ErrInvalidInput,
			fmt.Errorf("%T cannot be cast to driver.Value", val))
	}
}

// MarshalText defines how to marshal the optional value in text-based
// contexts. In most cases it falls back on the value of the optional:
// using encoding.TextMarshaler, json.Marshaler, yaml.Marahaler, or a
// generic Marshal() ([]byte,error) interface are called. Strings and
// []byte values are through directly, and failing all of these
// options, this falls back to json.Marshal().
func (o Optional[T]) MarshalText() ([]byte, error) {
	switch vt := any(o.v).(type) {
	case encoding.TextMarshaler:
		return vt.MarshalText()
	case json.Marshaler:
		return vt.MarshalJSON()
	case interface{ MarshalYAML() ([]byte, error) }:
		return vt.MarshalYAML()
	case interface{ Marshal() ([]byte, error) }:
		return vt.Marshal()
	case string:
		return []byte(vt), nil
	case []byte:
		return vt, nil
	default:
		return json.Marshal(vt)
	}
}

// UnmarshalText provides an inverse version of MarshalText for the
// encoding.UnmarshalText interface. encoding.TextUnmarshaler,
// json.Unmarshaler, yaml.Unmarshaler, and a generic Unmarshler
// interface. Strings and bytes slices pass through directly, and
// json.Unmarshal() is used in all other situations.
func (o *Optional[T]) UnmarshalText(in []byte) (err error) {
	defer func() { o.defined = (err == nil && ft.Not(ft.IsNil(o.v))) }()
	o.init()

	switch vt := any(o.v).(type) {
	case encoding.TextUnmarshaler:
		return vt.UnmarshalText(in)
	case json.Unmarshaler:
		return vt.UnmarshalJSON(in)
	case interface{ UnmarshalYAML([]byte) error }:
		return vt.UnmarshalYAML(in)
	case interface{ Unmarshal([]byte) error }:
		return vt.Unmarshal(in)
	case string:
		o.v = any(string(in)).(T)
		return nil
	case []byte:
		o.v = any(in).(T)
		return nil
	default:
		return json.Unmarshal(in, &o.v)
	}
}

// MarshalBinary supports marshaling optional values into binary
// formats and uses the value of the optional to dictate
// behavior. The underlying value must implement encoding.BinaryMarshaler,
// bson.Marshaler or a generic Marshal() interface. byte slices are
// passed through.
func (o Optional[T]) MarshalBinary() ([]byte, error) {
	switch vt := any(o.v).(type) {
	case encoding.BinaryMarshaler:
		return vt.MarshalBinary()
	case interface{ Marshal() ([]byte, error) }:
		return vt.Marshal()
	case interface{ MarshalBSON() ([]byte, error) }:
		return vt.MarshalBSON()
	case []byte:
		return vt, nil
	default:
		return nil, fmt.Errorf("could not marshal %T, implement econding.BinaryMarshaler", o.v)
	}
}

// UnmarshalBinary provides a compliment to MarshalBinary, and serves
// as a passthrough for encoding.BinaryUnmarshaler, bson.Unmarshaler
// and the generic Unmarshal interface.
func (o *Optional[T]) UnmarshalBinary(in []byte) (err error) {
	defer func() { o.defined = (err == nil && ft.Not(ft.IsNil(o.v))) }()

	o.init()
	switch vt := any(o.v).(type) {
	case encoding.BinaryUnmarshaler:
		return vt.UnmarshalBinary(in)
	case interface{ Unmarshal([]byte) error }:
		return vt.Unmarshal(in)
	case interface{ UnmarshalBSON([]byte) error }:
		return vt.UnmarshalBSON(in)
	case []byte:
		o.v = any(in).(T)
		return nil
	default:
		return fmt.Errorf("could not unmarshal %T, implement econding.BinaryUnmarshaler", o.v)
	}
}
