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

// registerFuncFlag registers a scalar flag for types with no native FlagSet
// method. The default in spec is applied immediately via parse if non-empty.
func registerFuncFlag[T any](fs *flag.FlagSet, p *T, spec flagSpec, parse func(string) (T, error)) error {
	if spec.Default != "" {
		def, err := parse(spec.Default)
		if err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", spec.Name, spec.Default))
		}
		*p = def
	}
	registerAlias(spec, func(n, u string) {
		fs.Func(n, u, func(s string) error {
			v, err := parse(s)
			if err != nil {
				return err
			}
			*p = v
			return nil
		})
	})
	return nil
}

// registerSliceFlag registers a slice flag. The default in spec is a
// comma-separated list pre-populated into *p before parsing begins.
func registerSliceFlag[T any](fs *flag.FlagSet, p *[]T, spec flagSpec, parse func(string) (T, error)) error {
	if err := parseAndSetSliceDefault(p, spec.Default, parse); err != nil {
		return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", spec.Name, spec.Default))
	}
	fn := appendOnce(p, parse, spec.Default != "")
	registerAlias(spec, func(n, u string) { fs.Func(n, u, fn) })
	return nil
}

// registerFlag registers a single flag (and optional short alias) on fs.
// ptr must be the result of fval.Addr().Interface() for the struct field.
// format is the value of the `format:` struct tag; it is only meaningful for
// *time.Time fields (Go reference-time layout) and ignored for all other types.
func registerFlag(fs *flag.FlagSet, ptr any, spec flagSpec) error {
	if err := spec.validate(fs); err != nil {
		return err
	}

	switch p := ptr.(type) {
	// ── scalar types ──────────────────────────────────────────────────────────
	case *string:
		registerAlias(spec, func(n, u string) { fs.StringVar(p, n, spec.Default, u) })
	case *bool:
		switch strings.ToLower(spec.Default) {
		case "true", "t", "yes", "y", "1":
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
			fs.BoolFunc(joinStr("no-", spec.Name), spec.Help, fn)
			if spec.Short != "" {
				fs.BoolFunc(joinStr("no-", spec.Short), joinStr("short for -no-", spec.Name), fn)
			}
		case "false", "f", "no", "n", "", "0", "-1":
			registerAlias(spec, func(n, u string) { fs.BoolVar(p, n, false, u) })
			return nil
		default:
			return ers.Wrapf(ErrInvalidSpecification, "field %q has impossible default %q", spec.Name, spec.Default)
		}
	case *int:
		return registerFuncFlag(fs, p, spec, parseInt[int](strconv.IntSize))
	case *int64:
		return registerFuncFlag(fs, p, spec, parseInt[int64](64))
	case *uint:
		return registerFuncFlag(fs, p, spec, parseUint[uint](strconv.IntSize))
	case *uint64:
		return registerFuncFlag(fs, p, spec, parseUint[uint64](64))
	case *float64:
		return registerFuncFlag(fs, p, spec, parseFloat[float64](64))
	case *float32:
		return registerFuncFlag(fs, p, spec, parseFloat[float32](32))
	case *int32:
		return registerFuncFlag(fs, p, spec, parseInt[int32](32))
	case *int16:
		return registerFuncFlag(fs, p, spec, parseInt[int16](16))
	case *int8:
		return registerFuncFlag(fs, p, spec, parseInt[int8](8))
	case *uint32:
		return registerFuncFlag(fs, p, spec, parseUint[uint32](32))
	case *uint16:
		return registerFuncFlag(fs, p, spec, parseUint[uint16](16))
	case *uint8:
		return registerFuncFlag(fs, p, spec, parseUint[uint8](8))

	// ── slice types ───────────────────────────────────────────────────────────
	case *[]string:
		return registerSliceFlag(fs, p, spec, func(s string) (string, error) { return s, nil })
	case *[]int:
		return registerSliceFlag(fs, p, spec, parseInt[int](strconv.IntSize))
	case *[]int64:
		return registerSliceFlag(fs, p, spec, parseInt[int64](64))
	case *[]uint:
		return registerSliceFlag(fs, p, spec, parseUint[uint](strconv.IntSize))
	case *[]uint64:
		return registerSliceFlag(fs, p, spec, parseUint[uint64](64))
	case *[]float64:
		return registerSliceFlag(fs, p, spec, parseFloat[float64](64))
	case *[]float32:
		return registerSliceFlag(fs, p, spec, parseFloat[float32](32))
	case *[]int32:
		return registerSliceFlag(fs, p, spec, parseInt[int32](32))
	case *[]int16:
		return registerSliceFlag(fs, p, spec, parseInt[int16](16))
	case *[]int8:
		return registerSliceFlag(fs, p, spec, parseInt[int8](8))
	case *[]uint32:
		return registerSliceFlag(fs, p, spec, parseUint[uint32](32))
	case *[]uint16:
		return registerSliceFlag(fs, p, spec, parseUint[uint16](16))
	case *[]uint8:
		return registerSliceFlag(fs, p, spec, parseUint[uint8](8))
	case *[]bool:
		// Normalize aliases that strconv.ParseBool doesn't accept.
		switch strings.ToLower(spec.Default) {
		case "yes", "y", "true", "t", "1":
			spec.Default = "true"
		case "no", "n", "false", "f", "0", "-1":
			spec.Default = "false"
		}
		if err := parseAndSetSliceDefault(p, spec.Default, strconv.ParseBool); err != nil {
			return erc.Join(ErrInvalidSpecification, err, fmt.Errorf("field %q had default %q", spec.Name, spec.Default))
		}
		switch spec.Default {
		case "true":
			// Inverted: register as no-<name> using BoolFunc so bare
			// -no-flag (without =value) is accepted, mirroring the scalar
			// bool inverted-flag convention. Each occurrence appends !b.
			fn := appendOnce(p, func(s string) (bool, error) {
				b, err := strconv.ParseBool(s)
				return !b, err
			}, true)
			fs.BoolFunc(joinStr("no-", spec.Name), spec.Help, fn)
			if spec.Short != "" {
				fs.BoolFunc(joinStr("no-", spec.Short), joinStr("short for -no-", spec.Name), fn)
			}
		case "false":
			fallthrough
		default:
			fn := appendOnce(p, strconv.ParseBool, spec.Default != "")
			registerAlias(spec, func(n, u string) { fs.Func(n, u, fn) })
		}

	// ── time types ────────────────────────────────────────────────────────────
	case *time.Time:
		var parse func(string) (time.Time, error)
		if spec.Format != "" {
			parse = parseTimeFunc(spec.Format)
		} else {
			parse = parseTimeFuncAuto()
		}
		if spec.Default != "" {
			t, err := parse(spec.Default)
			if err != nil {
				return erc.Join(ErrInvalidSpecification, err,
					fmt.Errorf("field %q had default %q", spec.Name, spec.Default))
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
		registerAlias(spec, func(n, u string) { fs.Func(n, u, fn) })
	case *time.Duration:
		var def time.Duration
		if spec.Default != "" {
			d, err := time.ParseDuration(spec.Default)
			if err != nil {
				return erc.Join(ErrInvalidSpecification, err,
					fmt.Errorf("field %q had default %q", spec.Name, spec.Default))
			}
			def = d
		}
		registerAlias(spec, func(n, u string) { fs.DurationVar(p, n, def, u) })
	case *[]time.Time:
		var parse func(string) (time.Time, error)
		if spec.Format != "" {
			parse = parseTimeFunc(spec.Format)
		} else {
			parse = parseTimeFuncAuto()
		}
		return registerSliceFlag(fs, p, spec, parse)
	case *[]time.Duration:
		return registerSliceFlag(fs, p, spec, time.ParseDuration)

	case flag.Value:
		if spec.Default != "" {
			if err := p.Set(spec.Default); err != nil {
				return ers.Wrapf(ErrInvalidSpecification, "field %q default %q: %w", spec.Name, spec.Default, err)
			}
		}
		registerAlias(spec, func(n, u string) { fs.Var(p, n, u) })

	default:
		return ers.Wrapf(ErrInvalidSpecification, "unsupported field type %T for flag %q", ptr, spec.Name)
	}
	return nil
}

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

// parseTimeFuncAuto returns a parser that tries RFC3339, then DateTime, then
// DateOnly in order, accepting the first layout that succeeds. The special
// value "now" resolves to time.Now().UTC() at call time.
func parseTimeFuncAuto() func(string) (time.Time, error) {
	layouts := []string{time.RFC3339, time.DateTime, time.DateOnly}
	return func(s string) (time.Time, error) {
		if s == "now" {
			return time.Now().UTC(), nil
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("parsing time %q: tried layouts %v", s, layouts)
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

func parseInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](bits int) func(string) (T, error) {
	return func(s string) (T, error) { v, err := strconv.ParseInt(s, 10, bits); return T(v), err }
}

func parseUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](bits int) func(string) (T, error) {
	return func(s string) (T, error) { v, err := strconv.ParseUint(s, 10, bits); return T(v), err }
}

func parseFloat[T ~float32 | ~float64](bits int) func(string) (T, error) {
	return func(s string) (T, error) { v, err := strconv.ParseFloat(s, bits); return T(v), err }
}
