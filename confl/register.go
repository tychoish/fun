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

// registerSliceFlag registers a slice flag (and optional short alias) on fs.
// parse converts a single token to T. defStr is comma-separated default values
// pre-populated into *p before parsing begins.
func registerSliceFlag[T any](fs *flag.FlagSet, p *[]T, name, short, defStr, help string, parse func(string) (T, error)) error {
	if err := parseAndSetSliceDefault(p, defStr, parse); err != nil {
		return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
	}
	fn := appendOnce(p, parse, defStr != "")
	registerAlias(name, short, help, func(n, u string) { fs.Func(n, u, fn) })
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
	if fs.Lookup(name) != nil {
		return ers.Wrapf(ErrInvalidSpecification, "flag %q already registered (duplicate field or flat-namespace collision)", name)
	}
	if short != "" && fs.Lookup(short) != nil {
		return ers.Wrapf(ErrInvalidSpecification, "short flag %q (for %q) already registered", short, name)
	}

	switch p := ptr.(type) {
	// ── scalar types ──────────────────────────────────────────────────────────
	case *string:
		registerAlias(name, short, help, func(n, u string) { fs.StringVar(p, n, defStr, u) })
	case *bool:
		switch strings.ToLower(defStr) {
		case "true", "t", "yes", "y":
			// Inverted boolean: default is true, so the flag negates it.
			// Register "no-<name>" which sets the field to false when passed.
			*p = true
			fn := func(s string) error {
				b, err := strconv.ParseBool(s)
				if err != nil {
					return err
				}
				*p = !b
				return nil
			}
			fs.BoolFunc(joinStr("no-", name), help, fn)
			if short != "" {
				fs.BoolFunc(joinStr("no-", short), joinStr("short for -no-", name), fn)
			}
		case "false", "f", "no", "n", "":
			registerAlias(name, short, help, func(n, u string) { fs.BoolVar(p, n, false, u) })
			return nil
		default:
			return ers.Wrapf(ErrInvalidSpecification, "field %q has impossible default %q", name, defStr)
		}
	case *int:
		def, err := strconv.Atoi(defStr)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		registerAlias(name, short, help, func(n, u string) { fs.IntVar(p, n, def, u) })
	case *int64:
		def, err := strconv.ParseInt(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		registerAlias(name, short, help, func(n, u string) { fs.Int64Var(p, n, def, u) })
	case *uint:
		raw, err := strconv.ParseUint(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		registerAlias(name, short, help, func(n, u string) { fs.UintVar(p, n, uint(raw), u) })
	case *uint64:
		def, err := strconv.ParseUint(defStr, 10, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		registerAlias(name, short, help, func(n, u string) { fs.Uint64Var(p, n, def, u) })
	case *float64:
		def, err := strconv.ParseFloat(defStr, 64)
		if err != nil && defStr != "" {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		registerAlias(name, short, help, func(n, u string) { fs.Float64Var(p, n, def, u) })

	// ── slice types ───────────────────────────────────────────────────────────
	case *[]string:
		return registerSliceFlag(fs, p, name, short, defStr, help, func(s string) (string, error) { return s, nil })
	case *[]int:
		return registerSliceFlag(fs, p, name, short, defStr, help, strconv.Atoi)
	case *[]int64:
		return registerSliceFlag(fs, p, name, short, defStr, help, func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) })
	case *[]uint:
		return registerSliceFlag(fs, p, name, short, defStr, help, func(s string) (uint, error) {
			v, err := strconv.ParseUint(s, 10, 64)
			return uint(v), err
		})
	case *[]uint64:
		return registerSliceFlag(fs, p, name, short, defStr, help, func(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) })
	case *[]float64:
		return registerSliceFlag(fs, p, name, short, defStr, help, func(s string) (float64, error) { return strconv.ParseFloat(s, 64) })
	case *[]bool:
		if err := parseAndSetSliceDefault(p, defStr, strconv.ParseBool); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", name, defStr))
		}
		if defStr == "true" {
			// Inverted: register as no-<name> using BoolFunc so bare
			// -no-flag (without =value) is accepted, mirroring the scalar
			// bool inverted-flag convention. Each occurrence appends !b.
			fn := appendOnce(p, func(s string) (bool, error) {
				b, err := strconv.ParseBool(s)
				return !b, err
			}, true)
			fs.BoolFunc(joinStr("no-", name), help, fn)
			if short != "" {
				fs.BoolFunc(joinStr("no-", short), joinStr("short for -no-", name), fn)
			}
		} else {
			fn := appendOnce(p, strconv.ParseBool, defStr != "")
			registerAlias(name, short, help, func(n, u string) { fs.Func(n, u, fn) })
		}

	// ── time types ────────────────────────────────────────────────────────────
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
		registerAlias(name, short, help, func(n, u string) { fs.Func(n, u, fn) })
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
		registerAlias(name, short, help, func(n, u string) { fs.DurationVar(p, n, def, u) })
	case *[]time.Time:
		layout := format
		if layout == "" {
			layout = time.RFC3339
		}
		return registerSliceFlag(fs, p, name, short, defStr, help, parseTimeFunc(layout))
	case *[]time.Duration:
		return registerSliceFlag(fs, p, name, short, defStr, help, time.ParseDuration)

	case flag.Value:
		if defStr != "" {
			if err := p.Set(defStr); err != nil {
				return ers.Wrapf(ErrInvalidSpecification, "field %q default %q: %w", name, defStr, err)
			}
		}
		registerAlias(name, short, help, func(n, u string) { fs.Var(p, n, u) })

	default:
		return ers.Wrapf(ErrInvalidSpecification, "unsupported field type %T for flag %q", ptr, name)
	}
	return nil
}
