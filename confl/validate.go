package confl

import (
	"flag"
	"reflect"

	"github.com/tychoish/fun/ers"
)

// Validate checks cfg's struct tags for specification errors without parsing
// any arguments. It returns ErrInvalidSpecification on the first problem found.
// Call it at program startup or in test init to catch tag errors early:
//
//	var _ = erc.Must(confl.Validate(&MyConfig{}))
func Validate(cfg any) error {
	val, err := unwrapConf(cfg)
	if err != nil {
		return err
	}
	// Use a temporary FlagSet for duplicate-name detection only; no flags are
	// actually parsed.
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	return validateStruct(fs, val, "", 0)
}

// validateStruct walks val, checking all tag constraints without registering
// real flags.  It mirrors the logic in bindFlags but focuses solely on
// structural correctness.
func validateStruct(fs *flag.FlagSet, val reflect.Value, prefix string, depth int) error {
	t := val.Type()

	// Track narg:"rest" fields and whether any cmd: fields exist at this level.
	var restFieldName string
	hasCmdField := false

	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		// ── struct fields ────────────────────────────────────────────────────
		if fval.Kind() == reflect.Struct && fval.Type() != timeTimeType {
			isCmd, handled, err := validateStructField(fs, field, fval, prefix, depth)
			if err != nil {
				return err
			}
			if isCmd {
				hasCmdField = true
			}
			if handled {
				continue
			}
		}

		// ── narg tag ─────────────────────────────────────────────────────────
		name := field.Tag.Get("flag")
		narg := field.Tag.Get("narg")
		if err := checkNargTags(field, fval, name, narg); err != nil {
			return err
		}

		if narg == "rest" {
			if restFieldName != "" {
				return ers.Wrapf(ErrInvalidSpecification,
					"multiple narg:\"rest\" fields found (%q and %q); only one is allowed per struct",
					restFieldName, field.Name)
			}
			restFieldName = field.Name
			continue
		}

		if name == "" {
			continue
		}
		if err := checkExportedFlag(field, name); err != nil {
			return err
		}

		// ── sep: tag ─────────────────────────────────────────────────────────
		sepVal, sepSet := field.Tag.Lookup("sep")
		if err := checkSepTag(field, fval, sepSet); err != nil {
			return err
		}

		// Slice field with non-empty default but no sep: tag.
		if fval.Kind() == reflect.Slice && field.Tag.Get("default") != "" && !sepSet {
			return ers.Wrapf(ErrInvalidSpecification,
				"field %q is a slice with a non-empty default: value but has no sep: tag; add sep:\",\" to split on commas, sep:\"<x>\" for a custom separator, or sep:\"\" to treat the whole value as one element",
				field.Name)
		}

		// ── short: length check ───────────────────────────────────────────────
		short := field.Tag.Get("short")
		if short != "" && len(short) != 1 {
			return ers.Wrapf(ErrInvalidSpecification,
				"short flag for %q must be exactly one character, got %q", joinStr(prefix, name), short)
		}

		// ── duplicate flag detection ──────────────────────────────────────────
		fullName := joinStr(prefix, name)
		if fs.Lookup(fullName) != nil {
			return ers.Wrapf(ErrInvalidSpecification,
				"flag %q already registered (duplicate field or flat-namespace collision)", fullName)
		}
		if short != "" && fs.Lookup(short) != nil {
			return ers.Wrapf(ErrInvalidSpecification,
				"short flag %q (for %q) already registered", short, fullName)
		}

		// ── default value parseability ────────────────────────────────────────
		defStr := field.Tag.Get("default")
		ptr := fval.Addr().Interface()
		if err := validateDefault(ptr, defStr, sepVal, sepSet, field.Tag.Get("format"), fullName); err != nil {
			return err
		}

		// Register a placeholder bool so duplicate checks work for subsequent
		// fields processed in this same validateStruct call.
		fs.Bool(fullName, false, "")
		if short != "" {
			fs.Bool(short, false, "")
		}
	}

	// narg:"rest" is incompatible with cmd: fields at the same struct level.
	if restFieldName != "" && hasCmdField {
		return ers.Wrapf(ErrInvalidSpecification,
			"field %q has narg:\"rest\" but struct also has cmd: fields; they are incompatible",
			restFieldName)
	}

	return nil
}

// validateStructField handles a struct-kinded field during validateStruct
// traversal. Returns (hasCmdField, handled, err):
//   - hasCmdField: true when a cmd: tag was found on this field
//   - handled: true when the caller should continue to the next field
//   - err: non-nil on a validation failure
func validateStructField(fs *flag.FlagSet, field reflect.StructField, fval reflect.Value, prefix string, depth int) (hasCmdField, handled bool, err error) {
	cmdName := field.Tag.Get("cmd")
	if cmdName != "" {
		if !field.IsExported() {
			return false, false, ers.Wrapf(ErrInvalidSpecification,
				"field %q with cmd: tag must be exported", field.Name)
		}
		if _, hasFlag := field.Tag.Lookup("flag"); hasFlag {
			return false, false, ers.Wrapf(ErrInvalidSpecification,
				"field %q has both flag: and cmd: tags; they are mutually exclusive",
				field.Name)
		}
		// Use a fresh FlagSet per subcommand so duplicate-name detection is
		// scoped to the subcommand's own flag namespace.
		subFS := flag.NewFlagSet("validate", flag.ContinueOnError)
		if err := validateStruct(subFS, fval, "", depth+1); err != nil {
			return false, false, err
		}
		return true, true, nil
	}
	if !field.IsExported() {
		return false, true, validateStruct(fs, fval, prefix, depth)
	}
	if _, ok := fval.Addr().Interface().(flag.Value); ok {
		return false, false, nil // treat as leaf
	}
	childPrefix := prefix
	if ns := field.Tag.Get("flag"); ns != "" && !field.Anonymous {
		childPrefix = joinStr(prefix, ns, ".")
	}
	return false, true, validateStruct(fs, fval, childPrefix, depth)
}

// validateDefault checks that defStr can be parsed for the type of ptr.
// It mirrors the logic in registerFlag without touching a real FlagSet.
func validateDefault(ptr any, defStr, sep string, sepSet bool, format, flagName string) error {
	if defStr == "" {
		return nil
	}
	// Delegate to a throw-away FlagSet to validate the default.
	dummyFS := flag.NewFlagSet("validate-default", flag.ContinueOnError)
	spec := flagSpec{
		Name:    flagName,
		Default: defStr,
		Sep:     sep,
		SepSet:  sepSet,
		Format:  format,
	}
	return registerFlag(dummyFS, ptr, spec)
}
