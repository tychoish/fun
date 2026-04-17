package confl

import (
	"flag"
	"reflect"
	"strings"

	"github.com/tychoish/fun/ers"
)

func joinStr(args ...string) string { return strings.Join(args, "") }
func callWhen[T any](cond bool, op func(T), num T) {
	if cond {
		op(num)
	}
}

// checkNargTags validates narg tag constraints shared by validateStruct and
// bindFlags. It covers: unknown narg values, unexported narg fields, non-slice
// narg fields, narg:"rest" combined with flag:, and narg:"until" without flag:.
// Callers that need to detect multiple narg:"rest" fields must track that
// separately (validateStruct does; bindFlags delegates to collectRestField).
func checkNargTags(field reflect.StructField, fval reflect.Value, name, narg string) error {
	if narg == "" {
		return nil
	}
	switch narg {
	case "rest", "until":
	default:
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has unknown narg value %q (must be \"rest\" or \"until\")",
			field.Name, narg)
	}
	if !field.IsExported() {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q with narg tag must be exported", field.Name)
	}
	if fval.Kind() != reflect.Slice {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has narg:%q but is not a slice type (got %s)",
			field.Name, narg, fval.Kind())
	}
	if narg == "rest" && name != "" {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has both narg:\"rest\" and flag: tag; they are mutually exclusive",
			field.Name)
	}
	if narg == "until" && name == "" {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has narg:\"until\" but no flag: tag; until fields must have a flag: tag",
			field.Name)
	}
	return nil
}

// checkSepTag validates that the sep: tag is only used on slice fields.
func checkSepTag(field reflect.StructField, fval reflect.Value, sepSet bool) error {
	if sepSet && fval.Kind() != reflect.Slice {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has sep: tag but is not a slice type (got %s); sep: is only valid on slice fields",
			field.Name, fval.Kind())
	}
	return nil
}

// checkExportedFlag validates that a field carrying a flag: tag is exported.
func checkExportedFlag(field reflect.StructField, name string) error {
	if !field.IsExported() {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q with flag tag %q must be exported", field.Name, name)
	}
	return nil
}

// flagSpec holds the tag-derived metadata for a single flag registration.
// It is constructed from struct field tags in bindFlags and passed through
// registerFlag, registerFuncFlag, and registerSliceFlag to avoid long argument
// lists. The Format field is only meaningful for time.Time fields.
// Sep and SepSet carry the sep: struct tag. SepSet is true when the tag was
// explicitly present (even if its value is empty). Sep is only meaningful for
// slice fields; the registerFlag dispatcher rejects it on scalar types.
type flagSpec struct {
	Name    string
	Short   string
	Default string
	Help    string
	Format  string
	Sep     string // separator for slice element splitting; "" means no splitting
	SepSet  bool   // true when the sep: tag was explicitly present
}

// validate checks structural constraints that can be verified without a
// FlagSet, then checks for duplicate registration against fs.
func (s flagSpec) validate(fs *flag.FlagSet) error {
	if s.Short != "" && len(s.Short) != 1 {
		return ers.Wrapf(ErrInvalidSpecification,
			"short flag for %q must be exactly one character, got %q", s.Name, s.Short)
	}
	if fs.Lookup(s.Name) != nil {
		return ers.Wrapf(ErrInvalidSpecification,
			"flag %q already registered (duplicate field or flat-namespace collision)", s.Name)
	}
	if s.Short != "" && fs.Lookup(s.Short) != nil {
		return ers.Wrapf(ErrInvalidSpecification,
			"short flag %q (for %q) already registered", s.Short, s.Name)
	}
	return nil
}

// registerAlias calls register(spec.Name, spec.Help) and, when Short is
// non-empty, register(spec.Short, "short for -<name>").
func registerAlias(spec flagSpec, register func(name, usage string)) {
	register(spec.Name, spec.Help)
	if spec.Short != "" {
		register(spec.Short, joinStr("short for -", spec.Name))
	}
}
