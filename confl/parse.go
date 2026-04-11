package confl

import (
	"errors"
	"flag"
	"os"
	"reflect"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// timeTimeType is the reflect.Type for time.Time. Used to detect time.Time
// struct fields in bindFlags and checkRequired so they are treated as leaves
// rather than recursed into.
var timeTimeType = reflect.TypeFor[time.Time]()

// conflagure is the internal implementation. Tests call it directly with a
// custom FlagSet and explicit args to avoid touching flag.CommandLine.
func conflagure(fs *flag.FlagSet, conf any, args []string) error {
	if conf == nil {
		return ers.Wrap(ErrInvalidSpecification, "conf must be a pointer to a struct, got nil")
	}
	t := reflect.TypeOf(conf)
	v := reflect.ValueOf(conf)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return ers.Wrapf(ErrInvalidSpecification, "conf must be a pointer to a struct, got %T", conf)
	}

	if err := bindFlags(fs, v.Elem(), "", 0); err != nil {
		return err
	}

	if err := fs.Parse(args); err != nil {
		callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0)
		return err
	}

	return checkRequired(v.Elem(), "")
}

// bindFlags recursively walks val, registering every field tagged with `flag:`
// onto fs. prefix is prepended to every flag name registered in this call and
// any recursive calls. Anonymous (embedded) struct fields are traversed with
// the same prefix (flat namespace). Named struct fields with a `flag:` tag
// extend the prefix: `flag:"ns"` produces "-ns.<leaf>" flags.
func bindFlags(fs *flag.FlagSet, val reflect.Value, prefix string, depth int) error {
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		if fval.Kind() == reflect.Struct && fval.Type() != timeTimeType {
			recurse, err := bindStructField(fs, field, fval, prefix, depth)
			if err != nil {
				return err
			}
			if recurse {
				continue
			}
		}

		name := field.Tag.Get("flag")
		if name == "" {
			continue
		}
		if !field.IsExported() {
			return ers.Wrapf(ErrInvalidSpecification, "field %q with flag tag %q must be exported", field.Name, name)
		}
		if err := registerFlag(fs, fval.Addr().Interface(), joinStr(prefix, name),
			field.Tag.Get("short"),
			field.Tag.Get("default"),
			field.Tag.Get("format"),
			field.Tag.Get("help"),
		); err != nil {
			return erc.Join(ErrInvalidSpecification, err)
		}
	}
	return nil
}

// bindStructField handles a struct-kinded field during bindFlags traversal.
// It returns (true, nil) when the caller should continue to the next field,
// (false, nil) when the field should be processed as a leaf, and (false, err)
// on error.
func bindStructField(fs *flag.FlagSet, field reflect.StructField, fval reflect.Value, prefix string, depth int) (bool, error) {
	if name := field.Tag.Get("cmd"); name != "" {
		if depth > 0 {
			return false, ers.Wrapf(ErrInvalidSpecification,
				"field %q: nested cmd: tags (subcommands inside subcommands) are not supported",
				field.Name)
		}
		return true, nil // top-level subcommand field; handled by collectSubcommands
	}
	// Unexported struct fields are always recursed without a flag.Value check.
	if !field.IsExported() {
		return true, bindFlags(fs, fval, prefix, depth)
	}
	// If the exported pointer implements flag.Value, treat as a leaf.
	if _, ok := fval.Addr().Interface().(flag.Value); ok {
		return false, nil
	}
	childPrefix := prefix
	if ns := field.Tag.Get("flag"); ns != "" && !field.Anonymous {
		childPrefix = joinStr(prefix, ns, ".")
	}
	return true, bindFlags(fs, fval, childPrefix, depth)
}

// checkRequired recursively walks val and returns an error for any field
// marked required:"true" that is still zero after parsing. prefix mirrors the
// prefix used during bindFlags so that error messages show the full flag name.
func checkRequired(val reflect.Value, prefix string) error {
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		if fval.Kind() == reflect.Struct && fval.Type() != timeTimeType {
			if recurse, err := checkStructField(field, fval, prefix); err != nil {
				return err
			} else if recurse {
				continue
			}
		}

		if fval.Kind() == reflect.Slice {
			if field.Tag.Get("required") != "true" {
				continue
			}
			name := field.Tag.Get("flag")
			if name != "" && fval.Len() == 0 {
				return ers.Wrapf(ErrInvalidInput, "required flag -%s%s not set", prefix, name)
			}
			continue
		}

		if field.Tag.Get("required") != "true" {
			continue
		}
		name := field.Tag.Get("flag")
		if name != "" && fval.IsZero() {
			return ers.Wrapf(ErrInvalidInput, "required flag -%s%s not set", prefix, name)
		}
	}
	return nil
}

// checkStructField handles a struct-kinded field during checkRequired traversal.
// Returns (true, nil) when the caller should continue to the next field,
// (false, nil) when the field should be processed as a leaf, and (false, err)
// on error.
func checkStructField(field reflect.StructField, fval reflect.Value, prefix string) (bool, error) {
	if field.Tag.Get("cmd") != "" {
		return true, nil // subcommand fields are validated separately during dispatch
	}
	// Unexported struct fields are always recursed without a flag.Value check.
	if !field.IsExported() {
		return true, checkRequired(fval, prefix)
	}
	// If the exported pointer implements flag.Value, treat as a leaf.
	if _, ok := fval.Addr().Interface().(flag.Value); ok {
		return false, nil
	}
	childPrefix := prefix
	if !field.Anonymous {
		if ns := field.Tag.Get("flag"); ns != "" {
			childPrefix = joinStr(prefix, ns, ".")
		}
	}
	return true, checkRequired(fval, childPrefix)
}
