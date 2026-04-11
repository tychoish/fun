package confl

import (
	"flag"
	"os"

	"github.com/tychoish/fun/ers"
)

const (
	// ErrInvalidSpecification indicates a programming error in the struct definition.
	ErrInvalidSpecification = ers.Error("incorrect flag/configuration specification")
	// ErrInvalidInput indicates the user supplied an impossible or unparsable value.
	ErrInvalidInput = ers.Error("received invalid/impossible flag/configuration")
)

// commandLineArgs returns os.Args[1:], or flag.CommandLine.Args() when
// flag.CommandLine has already been parsed by a prior flag.Parse call.
func commandLineArgs() []string {
	if flag.CommandLine.Parsed() {
		return flag.CommandLine.Args()
	}
	return os.Args[1:]
}

// Parse populates cfg from command-line arguments using struct field tags.
// cfg must be a pointer to a struct. Supported tags:
//
//   - flag:"name"      flag name (required to register a field)
//   - default:"val"    default value
//   - help:"text"      usage description
//   - short:"x"        single-letter alias
//   - required:"true"  field must be non-zero after parsing
//
// Struct fields are registered with a flat namespace unless the struct field
// itself carries a flag: tag, in which case all nested leaf flags are prefixed
// with that name (e.g. flag:"srv" + nested flag:"host" → "-srv.host").
// Namespaces nest and are separated by ".".
//
// A named struct field without a flag: tag is traversed with the same
// prefix as its parent — identical to anonymous embedding. If any of
// its leaf flags share a name with a flag already registered at the
// enclosing level, Parse returns ErrInvalidSpecification. Use a 'flag'
// tag to give the nested struct its own namespace and avoid collisions.
//
// Supported underlying types: string, bool, int, int64, uint, uint64,
// float64, time.Time, time.Duration, and slices thereof.
//
// Fields tagged cmd: are skipped; use ParseCommand or Dispatch for subcommands.
func Parse(cfg any) error { return conflagure(flag.CommandLine, cfg, commandLineArgs()) }

// Dispatch parses global flags and selects a subcommand, returning the chosen
// subcommand struct pointer as any. Callers must type-switch on the result to
// identify which subcommand was selected. Returns an error if no subcommand
// was selected or if parsing fails. Subcommand types need not implement
// Commander; use ParseCommand if you need that guarantee.
func Dispatch(cfg any) (any, error) {
	return dispatch(flag.CommandLine, cfg, commandLineArgs())
}

// ParseCommand parses global flags and selects a subcommand, returning it as a
// Commander. All 'cmd' fields must implement Commander or an error is returned.
// If cfg itself implements Commander and no subcommand is selected, cfg is
// returned. Returns an error if no subcommand is selected and cfg does not
// implement Commander.
func ParseCommand(cfg any) (Commander, error) {
	return conflagureCmd(flag.CommandLine, cfg, commandLineArgs())
}
