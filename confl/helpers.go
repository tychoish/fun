package confl

import (
	"flag"
	"strings"

	"github.com/tychoish/fun/ers"
)

func joinStr(args ...string) string { return strings.Join(args, "") }
func callWhen[T any](cond bool, op func(T), num T) {
	if cond {
		op(num)
	}
}

// flagSpec holds the tag-derived metadata for a single flag registration.
// It is constructed from struct field tags in bindFlags and passed through
// registerFlag, registerFuncFlag, and registerSliceFlag to avoid long argument
// lists. The Format field is only meaningful for time.Time fields.
type flagSpec struct {
	Name    string
	Short   string
	Default string
	Help    string
	Format  string
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
