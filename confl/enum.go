package confl

import (
	"encoding"
	"flag"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/strut"
)

// enumEntry is one accepted name and the typed value it selects.
type enumEntry struct {
	name string
	val  reflect.Value // of the enum element type
}

// enumCounter tracks the iota-style running value while an integer enum tag is
// parsed. next holds the value for the next unnamed entry; unsigned kinds
// reinterpret it as a uint64.
type enumCounter struct {
	next    int64
	wrapped bool // the previous entry held the maximum 64-bit value
}

// assign stores the entry's value in v: the explicit valStr when hasVal is
// set, otherwise the running value. It then advances the counter.
func (c *enumCounter) assign(v reflect.Value, valStr string, hasVal bool) error {
	signed := v.CanInt()
	switch {
	case hasVal && signed:
		n, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return err
		}
		c.next = n
	case hasVal:
		n, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			return err
		}
		c.next = int64(n)
	case c.wrapped:
		// OverflowInt and OverflowUint cannot see a wrap at 64 bits.
		return fmt.Errorf("value after the maximum %s overflows", v.Type())
	}

	switch {
	case signed && v.OverflowInt(c.next):
		return fmt.Errorf("value %d overflows %s", c.next, v.Type())
	case !signed && v.OverflowUint(uint64(c.next)):
		return fmt.Errorf("value %d overflows %s", uint64(c.next), v.Type())
	case signed:
		v.SetInt(c.next)
		c.wrapped = c.next == math.MaxInt64
	default:
		v.SetUint(uint64(c.next))
		c.wrapped = uint64(c.next) == math.MaxUint64
	}
	c.next++
	return nil
}

// parseEnumTag builds the ordered name/value table for the element type t from
// the raw enum: entries. String kinds use each entry as both name and value.
// Integer kinds number entries like iota, with name=N overriding the running
// value. It rejects empty and duplicate names, duplicate values, and values
// that overflow t.
func parseEnumTag(raw []string, t reflect.Type, exact bool) ([]enumEntry, error) {
	isString := t.Kind() == reflect.String
	if probe := reflect.New(t).Elem(); !isString && !probe.CanInt() && !probe.CanUint() {
		return nil, ers.Wrapf(ErrInvalidSpecification,
			"enum: is not supported on %s fields (string and integer kinds only)", t.Kind())
	}

	var counter enumCounter
	entries := make([]enumEntry, 0, len(raw))
	for _, item := range raw {
		name, valStr, hasVal := item, "", false
		if !isString { // "=" is not special in string names
			name, valStr, hasVal = strings.Cut(item, "=")
			name, valStr = strings.TrimSpace(name), strings.TrimSpace(valStr)
		}
		if name == "" {
			return nil, ers.Wrapf(ErrInvalidSpecification, "enum has an empty name in entry %q", item)
		}

		v := reflect.New(t).Elem()
		if isString {
			v.SetString(name)
		} else if err := counter.assign(v, valStr, hasVal); err != nil {
			return nil, ers.Wrapf(ErrInvalidSpecification, "enum entry %q: %w", item, err)
		}

		for _, e := range entries {
			switch {
			case enumNameEqual(e.name, name, exact):
				return nil, ers.Wrapf(ErrInvalidSpecification,
					"enum names %q and %q are duplicates; add case-sensitive:\"true\" if they differ only by case", e.name, name)
			case e.val.Equal(v):
				return nil, ers.Wrapf(ErrInvalidSpecification, "enum entries %q and %q have the same value", e.name, name)
			}
		}
		entries = append(entries, enumEntry{name: name, val: v})
	}
	return entries, nil
}

func enumNameEqual(a, b string, exact bool) bool {
	if exact {
		return a == b
	}
	return strings.EqualFold(a, b)
}

// enumValue is the flag.Value for a field carrying an enum: tag. target is
// the settable field: a scalar, a slice, or a pointer of the element type.
type enumValue struct {
	target       reflect.Value
	elem         reflect.Type
	entries      []enumEntry
	choices      string // the entry names, for usage and error text
	exact        bool
	sep          string
	sepSet       bool
	clearOnFirst bool // the slice holds defaults that the first Set discards
	set          bool // a default, env var, or flag has supplied a value
}

// enumElemType returns the element type of an enum field type, looking
// through one level of slice or pointer.
func enumElemType(t reflect.Type) reflect.Type {
	if k := t.Kind(); k == reflect.Slice || k == reflect.Pointer {
		return t.Elem()
	}
	return t
}

// callEnumValidate runs the optional Validate() on a copy of v, so both value
// and pointer receivers are honored.
func callEnumValidate(v reflect.Value) error {
	p := reflect.New(v.Type())
	p.Elem().Set(v)
	if val, ok := reflect.TypeAssert[interface{ Validate() error }](p); ok {
		return val.Validate()
	}
	return nil
}

// nameOf returns the tag name of v, or "" when v is not in the table.
func (e *enumValue) nameOf(v reflect.Value) string {
	for _, en := range e.entries {
		if en.val.Equal(v) {
			return en.name
		}
	}
	return ""
}

func (e *enumValue) notAChoice() error {
	return ers.Wrapf(ErrInvalidInput, "must be one of %s", e.choices)
}

// parse converts one user-supplied token into a typed, validated value. A
// TextUnmarshaler type does its own parsing, but the result must still be one
// of the tag's values.
func (e *enumValue) parse(s string) (reflect.Value, error) {
	var v reflect.Value
	p := reflect.New(e.elem)
	if um, ok := reflect.TypeAssert[encoding.TextUnmarshaler](p); ok {
		if err := um.UnmarshalText([]byte(s)); err != nil {
			return v, ers.Wrapf(ErrInvalidInput, "%w", err)
		}
		if v = p.Elem(); e.nameOf(v) == "" {
			return v, e.notAChoice()
		}
	} else {
		i := slices.IndexFunc(e.entries, func(en enumEntry) bool { return enumNameEqual(en.name, s, e.exact) })
		if i < 0 {
			return v, e.notAChoice()
		}
		v = e.entries[i].val
	}
	if err := callEnumValidate(v); err != nil {
		return v, ers.Wrapf(ErrInvalidInput, "%w", err)
	}
	return v, nil
}

// Set implements flag.Value. Every element is parsed before the field is
// touched, so a failing Set leaves the field and any pending default intact.
func (e *enumValue) Set(s string) error {
	kind := e.target.Kind()
	parts := []string{s}
	if kind == reflect.Slice && e.sepSet && e.sep != "" {
		parts = slices.Collect(splitTrimmed(s, e.sep))
	}
	vals := make([]reflect.Value, 0, len(parts))
	for _, part := range parts {
		v, err := e.parse(part)
		if err != nil {
			return err
		}
		vals = append(vals, v)
	}
	switch kind {
	case reflect.Slice:
		if e.clearOnFirst {
			e.target.Set(e.target.Slice(0, 0))
			e.clearOnFirst = false
		}
		e.target.Set(reflect.Append(e.target, vals...))
	case reflect.Pointer:
		p := reflect.New(e.elem)
		p.Elem().Set(vals[0])
		e.target.Set(p)
	default:
		e.target.Set(vals[0])
	}
	e.set = true
	return nil
}

// String implements flag.Value. It reports "" until a value has been set, so
// -help shows (default X) only for fields with a default: tag, even when the
// zero value of an iota enum is one of the names. The flag package also calls
// it on a zero enumValue to detect zero defaults, so it must tolerate one.
func (e *enumValue) String() string {
	if e == nil || !e.set {
		return ""
	}
	switch e.target.Kind() {
	case reflect.Slice:
		names := make([]string, e.target.Len())
		for i := range names {
			names[i] = e.nameOf(e.target.Index(i))
		}
		return strut.JOIN.WithComma().Strings(names...)
	case reflect.Pointer:
		if e.target.IsNil() {
			return ""
		}
		return e.nameOf(e.target.Elem())
	default:
		return e.nameOf(e.target)
	}
}

// usage appends the accepted names to the help text.
func (e *enumValue) usage(help string) string {
	if help == "" {
		return strut.JOIN.WithConcat().Strings("one of: ", e.choices)
	}
	return strut.JOIN.WithConcat().Strings(help, " (one of: ", e.choices, ")")
}

// registerEnumFlag registers a flag for a field carrying an enum: tag. ptr is
// a pointer to the field, whose type is a named string or integer type, or a
// slice or pointer of one. The default, if any, is applied through Set.
func registerEnumFlag(fs *flag.FlagSet, ptr any, spec flagSpec) error {
	if _, ok := ptr.(flag.Value); ok {
		return ers.Wrapf(ErrInvalidSpecification, "field %q: enum: cannot be combined with a flag.Value type", spec.Name)
	}
	target := reflect.ValueOf(ptr).Elem()
	elem := enumElemType(target.Type())

	entries, err := parseEnumTag(spec.Enum, elem, spec.EnumExactCase)
	if err != nil {
		return ers.Wrapf(err, "field %q", spec.Name)
	}
	names := make([]string, len(entries))
	for i, en := range entries {
		if err := callEnumValidate(en.val); err != nil {
			return ers.Wrapf(ErrInvalidSpecification, "field %q: enum option %q is not valid: %w", spec.Name, en.name, err)
		}
		names[i] = en.name
	}

	val := &enumValue{
		target:  target,
		elem:    elem,
		entries: entries,
		choices: strut.JOIN.With(", ").Strings(names...),
		exact:   spec.EnumExactCase,
		sep:     spec.Sep,
		sepSet:  spec.SepSet,
	}
	if spec.Default != "" {
		if target.Kind() == reflect.Slice && !spec.SepSet {
			return ers.Wrapf(ErrInvalidSpecification, "field %q is a slice with a non-empty default: value but has no sep: tag", spec.Name)
		}
		if err := val.Set(spec.Default); err != nil {
			return ers.Wrapf(ErrInvalidSpecification, "field %q default %q: %w", spec.Name, spec.Default, err)
		}
		val.clearOnFirst = target.Kind() == reflect.Slice
	}
	registerAlias(spec, func(n, u string) {
		if n == spec.Name { // the short alias keeps its "short for" text
			u = val.usage(u)
		}
		fs.Var(val, n, u)
	})
	return nil
}
