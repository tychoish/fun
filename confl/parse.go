package confl

import (
	"errors"
	"flag"
	"maps"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// timeTimeType is the reflect.Type for time.Time. Used to detect time.Time
// struct fields in bindFlags and checkRequired so they are treated as leaves
// rather than recursed into.
var timeTimeType = reflect.TypeFor[time.Time]()

// unwrapConf validates that conf is a non-nil pointer to a struct and returns
// its Elem value. Returns ErrInvalidSpecification otherwise.
func unwrapConf(conf any) (reflect.Value, error) {
	if conf == nil {
		return reflect.Value{}, ers.Wrap(ErrInvalidSpecification, "conf must be a pointer to a struct, got nil")
	}
	t := reflect.TypeOf(conf)
	v := reflect.ValueOf(conf)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, ers.Wrapf(ErrInvalidSpecification, "conf must be a pointer to a struct, got %T", conf)
	}
	return v.Elem(), nil
}

// parseAndCheck binds flags from val onto fs, parses args, and validates
// required fields. It is the common parse/validate sequence shared by
// conflagure, dispatch, and conflagureCmd.
func parseAndCheck(fs *flag.FlagSet, val reflect.Value, args []string) error {
	if err := bindFlags(fs, val, "", 0); err != nil {
		return err
	}

	untilFlags := collectUntilFlags(val, "")
	if len(untilFlags) > 0 {
		args = expandUntilArgs(args, untilFlags)
	}

	if err := fs.Parse(args); err != nil {
		callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0)
		return err
	}

	if err := populateRestField(val, fs.Args()); err != nil {
		return err
	}

	return checkRequired(val, "")
}

// conflagure is the internal implementation. Tests call it directly with a
// custom FlagSet and explicit args to avoid touching flag.CommandLine.
func conflagure(fs *flag.FlagSet, conf any, args []string) error {
	val, err := unwrapConf(conf)
	if err != nil {
		return err
	}
	return parseAndCheck(fs, val, args)
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
		narg := field.Tag.Get("narg")

		if err := checkNargTags(field, fval, name, narg); err != nil {
			return err
		}

		if narg == "rest" {
			continue // rest fields are not registered as flags; handled by populateRestField
		}

		if name == "" {
			continue
		}
		if err := checkExportedFlag(field, name); err != nil {
			return err
		}
		sepVal, sepSet := field.Tag.Lookup("sep")
		if err := checkSepTag(field, fval, sepSet); err != nil {
			return err
		}
		if err := registerFlag(fs, fval.Addr().Interface(), flagSpec{
			Name:    joinStr(prefix, name),
			Short:   field.Tag.Get("short"),
			Default: field.Tag.Get("default"),
			Format:  field.Tag.Get("format"),
			Help:    field.Tag.Get("help"),
			Sep:     sepVal,
			SepSet:  sepSet,
		}); err != nil {
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
			narg := field.Tag.Get("narg")
			name := field.Tag.Get("flag")
			if narg == "rest" && fval.Len() == 0 {
				return ers.Wrapf(ErrInvalidInput, "required positional arguments not provided")
			}
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

// collectRestField scans val for a field tagged narg:"rest". It returns
// (field value, true, nil) when one is found, (zero, false, nil) when none
// exist, and an error for invalid configurations (multiple rest fields, or
// rest combined with cmd fields at the same struct level).
func collectRestField(val reflect.Value) (reflect.Value, bool, error) {
	t := val.Type()
	var restField reflect.Value
	var restFieldName string
	hasCmdField := false

	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		if field.Tag.Get("cmd") != "" {
			hasCmdField = true
			continue
		}
		if field.Tag.Get("narg") != "rest" {
			continue
		}
		if restFieldName != "" {
			return reflect.Value{}, false, ers.Wrapf(ErrInvalidSpecification,
				"multiple narg:\"rest\" fields found (%q and %q); only one is allowed per struct",
				restFieldName, field.Name)
		}
		restField = fval
		restFieldName = field.Name
	}

	if restFieldName == "" {
		return reflect.Value{}, false, nil
	}
	if hasCmdField {
		return reflect.Value{}, false, ers.Wrapf(ErrInvalidSpecification,
			"field %q has narg:\"rest\" but struct also has cmd: fields; they are incompatible",
			restFieldName)
	}
	return restField, true, nil
}

// populateRestField finds a narg:"rest" field in val and fills it with args.
// If no rest field exists, it is a no-op. parse helpers from register.go are
// used based on the element type of the slice.
func populateRestField(val reflect.Value, args []string) error {
	restField, ok, err := collectRestField(val)
	if err != nil {
		return err
	}
	if !ok || len(args) == 0 {
		return nil
	}
	return appendRestArgs(restField, args)
}

// appendRestArgs appends args to the slice pointed to by field, parsing each
// element according to the slice's element type. Supports the same types as
// registerSliceFlag in register.go.
func appendRestArgs(field reflect.Value, args []string) error {
	ptr := field.Addr().Interface()
	switch p := ptr.(type) {
	case *[]string:
		*p = append(*p, args...)
	case *[]int:
		return restAppend(p, args, parseInt[int](strconv.IntSize), "int")
	case *[]int64:
		return restAppend(p, args, parseInt[int64](64), "int64")
	case *[]int32:
		return restAppend(p, args, parseInt[int32](32), "int32")
	case *[]int16:
		return restAppend(p, args, parseInt[int16](16), "int16")
	case *[]int8:
		return restAppend(p, args, parseInt[int8](8), "int8")
	case *[]uint:
		return restAppend(p, args, parseUint[uint](strconv.IntSize), "uint")
	case *[]uint64:
		return restAppend(p, args, parseUint[uint64](64), "uint64")
	case *[]uint32:
		return restAppend(p, args, parseUint[uint32](32), "uint32")
	case *[]uint16:
		return restAppend(p, args, parseUint[uint16](16), "uint16")
	case *[]uint8:
		return restAppend(p, args, parseUint[uint8](8), "uint8")
	case *[]float64:
		return restAppend(p, args, parseFloat[float64](64), "float64")
	case *[]float32:
		return restAppend(p, args, parseFloat[float32](32), "float32")
	default:
		return ers.Wrapf(ErrInvalidSpecification,
			"narg:\"rest\" field has unsupported element type %T", ptr)
	}
	return nil
}

// collectUntilFlags scans val for fields tagged narg:"until" and returns a
// set of their flag names (with prefix applied). These are used by
// expandUntilArgs to pre-process args before flag.Parse.
func collectUntilFlags(val reflect.Value, prefix string) map[string]bool {
	result := map[string]bool{}
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		// Recurse into nested structs (same logic as bindFlags).
		if fval.Kind() == reflect.Struct && fval.Type() != timeTimeType {
			if field.Tag.Get("cmd") != "" {
				continue
			}
			if !field.IsExported() {
				maps.Copy(result, collectUntilFlags(fval, prefix))
				continue
			}
			if _, ok := fval.Addr().Interface().(flag.Value); ok {
				continue
			}
			childPrefix := prefix
			if ns := field.Tag.Get("flag"); ns != "" && !field.Anonymous {
				childPrefix = joinStr(prefix, ns, ".")
			}
			maps.Copy(result, collectUntilFlags(fval, childPrefix))
			continue
		}

		if field.Tag.Get("narg") != "until" {
			continue
		}
		name := field.Tag.Get("flag")
		if name == "" {
			continue // validation already caught this in bindFlags
		}
		fullName := joinStr(prefix, name)
		result[fullName] = true
		if short := field.Tag.Get("short"); short != "" {
			result[short] = true
		}
	}
	return result
}

// isFlag reports whether a command-line token looks like a flag: it starts
// with "-" and has at least one character after it (same heuristic as the
// flag package).
func isFlag(s string) bool {
	return len(s) > 1 && s[0] == '-'
}

// expandUntilArgs pre-processes args before flag.Parse. For each token that
// is an "until" flag, subsequent non-flag tokens are repeated as additional
// invocations of the same flag, so that the existing appendOnce machinery
// accumulates them all.
//
// Example:
//
//	untilFlags = {"files": true}
//	args       = ["-files", "a", "b", "c", "-out", "x"]
//	result     = ["-files", "a", "-files", "b", "-files", "c", "-out", "x"]
func expandUntilArgs(args []string, untilFlags map[string]bool) []string {
	result := make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		tok := args[i]

		if !isFlag(tok) {
			result = append(result, tok)
			i++
			continue
		}

		// Strip leading dashes and optional =value.
		stripped := strings.TrimLeft(tok, "-")
		flagName, _, hasEq := strings.Cut(stripped, "=")

		if !untilFlags[flagName] {
			result = append(result, tok)
			i++
			continue
		}

		// This is an until-flag. Consume it (and its = value if present).
		if hasEq {
			// -files=firstval → emit as-is, then collect following non-flag tokens
			result = append(result, tok)
			i++
		} else {
			// -files val → emit -files val, then look for more non-flag tokens
			result = append(result, tok)
			i++
			if i < len(args) && !isFlag(args[i]) {
				result = append(result, args[i])
				i++
			}
		}

		// Collect subsequent non-flag tokens, emitting each as -flagName <val>.
		for i < len(args) && !isFlag(args[i]) {
			result = append(result, joinStr("-", flagName), args[i])
			i++
		}
	}
	return result
}
