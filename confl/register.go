package confl

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// parseTimeFunc returns a parser for time.Time values using the given layout.
// The special value "now" resolves to time.Now().UTC() at call time.
func parseTimeFunc(layout string) func(string) (time.Time, error) {
	return func(s string) (time.Time, error) {
		if s == "now" {
			return time.Now().UTC(), nil
		}
		t, err := time.Parse(layout, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing time %q with layout %q: %w", s, layout, err)
		}
		return t, nil
	}
}

// appendOnce returns a flag.Func handler that appends parsed values to *p.
// If clearOnFirst is true (i.e. defaults were pre-populated), the slice is
// cleared before the first command-line value is appended, discarding the
// defaults. Both the long and short aliases for the same flag must share the
// same returned function so that whichever alias fires first clears the
// defaults exactly once.
func appendOnce[T any](p *[]T, parse func(string) (T, error), clearOnFirst bool) func(string) error {
	return func(s string) error {
		if clearOnFirst {
			*p = (*p)[:0]
			clearOnFirst = false
		}
		v, err := parse(s)
		if err != nil {
			return err
		}
		*p = append(*p, v)
		return nil
	}
}

// parseAndSetSliceDefault splits defStr on commas, parses each element with
// parse, and appends the results to *p. Returns an error if any element fails
// to parse. A blank defStr is a no-op.
func parseAndSetSliceDefault[T any](p *[]T, defStr string, parse func(string) (T, error)) error {
	if defStr == "" {
		return nil
	}
	for part := range strings.SplitSeq(defStr, ",") {
		v, err := parse(strings.TrimSpace(part))
		if err != nil {
			return err
		}
		*p = append(*p, v)
	}
	return nil
}

// registerFlag registers a single flag (and optional short alias) on fs.
// ptr must be the result of fval.Addr().Interface() for the struct field.
// format is the value of the `format:` struct tag; it is only meaningful for
// *time.Time fields (Go reference-time layout) and ignored for all other types.
func registerFlag(fs *flag.FlagSet, ptr any, name, short, defStr, format, help string) error {
	if short != "" && len(short) != 1 {
		return ers.Wrapf(ErrInvalidSpecification, "short flag for %q must be exactly one character, got %q", name, short)
	}

	switch p := ptr.(type) {
	// ── scalar types ──────────────────────────────────────────────────────────
	case *string:
		fs.StringVar(p, name, defStr, help)
		if short != "" {
			fs.StringVar(p, short, defStr, joinStr("short for -", name))
		}
	case *bool:
		switch strings.ToLower(defStr) {
		case "true", "t", "yes", "y":
			// Inverted boolean: default is true, so the flag negates it.
			// Register "no-<name>" which sets the field to false when passed.
			*p = true
			fs.BoolFunc(joinStr("no-", name), help, func(s string) error {
				b, err := strconv.ParseBool(s)
				if err != nil {
					return err
				}
				*p = !b
				return nil
			})
			if short == "" {
				return nil
			}
			fs.BoolFunc(
				joinStr("no-", short),
				joinStr("short for -no-", name),
				func(s string) error {
					b, err := strconv.ParseBool(s)
					if err != nil {
						return err
					}
					*p = !b
					return nil
				},
			)
		case "false", "f", "no", "n", "":
			fs.BoolVar(p, name, false, help)
			if short != "" {
				fs.BoolVar(p, short, false, joinStr("short for -", name))
			}
			return nil
		default:
			return ers.Wrapf(ErrInvalidSpecification, "field %q has impossible default %q", name, defStr)
		}
	case *int:
		def, err := strconv.Atoi(defStr)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fs.IntVar(p, name, def, help)
		if short != "" {
			fs.IntVar(p, short, def, joinStr("short for -", name))
		}
	case *int64:
		def, err := strconv.ParseInt(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fs.Int64Var(p, name, def, help)
		if short != "" {
			fs.Int64Var(p, short, def, joinStr("short for -", name))
		}
	case *uint:
		raw, err := strconv.ParseUint(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fs.UintVar(p, name, uint(raw), help)
		if short != "" {
			fs.UintVar(p, short, uint(raw), joinStr("short for -", name))
		}
	case *uint64:
		def, err := strconv.ParseUint(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fs.Uint64Var(p, name, def, help)
		if short != "" {
			fs.Uint64Var(p, short, def, joinStr("short for -", name))
		}
	case *float64:
		def, err := strconv.ParseFloat(defStr, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fs.Float64Var(p, name, def, help)
		if short != "" {
			fs.Float64Var(p, short, def, joinStr("short for -", name))
		}

	// ── slice types ───────────────────────────────────────────────────────────
	case *[]string:
		parse := func(s string) (string, error) { return s, nil }
		erc.Invariant(parseAndSetSliceDefault(p, defStr, parse),
			ErrInvalidSpecification,
			"field default for", name, "had default", defStr,
		)

		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]int:
		parse := strconv.Atoi
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]int64:
		parse := func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]uint:
		parse := func(s string) (uint, error) {
			v, err := strconv.ParseUint(s, 10, 64)
			return uint(v), err
		}
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]uint64:
		parse := func(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]float64:
		parse := func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]bool:
		if err := parseAndSetSliceDefault(p, defStr, strconv.ParseBool); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		if defStr == "true" {
			// Inverted: register as no-<name> using BoolFunc so bare
			// -no-flag (without =value) is accepted, mirroring the scalar
			// bool inverted-flag convention. Each occurrence appends !b.
			invertParse := func(s string) (bool, error) {
				b, err := strconv.ParseBool(s)
				return !b, err
			}
			fn := appendOnce(p, invertParse, true)
			fs.BoolFunc(joinStr("no-", name), help, fn)
			if short != "" {
				fs.BoolFunc(joinStr("no-", short), joinStr("short for -no-", name), fn)
			}
		} else {
			fn := appendOnce(p, strconv.ParseBool, defStr != "")
			fs.Func(name, help, fn)
			if short != "" {
				fs.Func(short, joinStr("short for -", name), fn)
			}
		}
	// ── time types ───────────────────────────────────────────────────────────
	case *time.Time:
		layout := format
		if layout == "" {
			layout = time.RFC3339
		}
		parse := parseTimeFunc(layout)
		if defStr != "" {
			t, err := parse(defStr)
			if err != nil {
				return erc.Join(ErrInvalidSpecification, err,
					fmt.Errorf("field %q had default %q", name, defStr))
			}
			*p = t
		}
		fn := func(s string) error {
			t, err := parse(s)
			if err != nil {
				return err
			}
			*p = t
			return nil
		}
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *time.Duration:
		var def time.Duration
		if defStr != "" {
			d, err := time.ParseDuration(defStr)
			if err != nil {
				return erc.Join(ErrInvalidSpecification, err,
					fmt.Errorf("field %q had default %q", name, defStr))
			}
			def = d
		}
		fs.DurationVar(p, name, def, help)
		if short != "" {
			fs.DurationVar(p, short, def, joinStr("short for -", name))
		}
	case *[]time.Time:
		layout := format
		if layout == "" {
			layout = time.RFC3339
		}
		parse := parseTimeFunc(layout)
		if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
			return erc.Join(ErrInvalidSpecification, err,
				fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, parse, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}
	case *[]time.Duration:
		if err := parseAndSetSliceDefault(p, defStr, time.ParseDuration); err != nil {
			return erc.Join(ErrInvalidSpecification, err,
				fmt.Errorf("field %q had default %q", name, defStr))
		}
		fn := appendOnce(p, time.ParseDuration, defStr != "")
		fs.Func(name, help, fn)
		if short != "" {
			fs.Func(short, joinStr("short for -", name), fn)
		}

	default:
		return ers.Wrapf(ErrInvalidSpecification, "unsupported field type %T for flag %q", ptr, name)
	}
	return nil
}
