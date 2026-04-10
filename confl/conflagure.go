package confl

import (
	"flag"
	"os"

	"github.com/tychoish/fun/ers"
)

const (
	// ErrInvalidSpecification is returned when the struct
	// definition is impossible or incorrect. Always indicates an
	// error in the program.
	ErrInvalidSpecification = ers.Error("incorrect flag/configuration specification")
	// ErrInvalidInput is returned when a user has specified an
	// impossible or un-parsable input.
	ErrInvalidInput = ers.Error("received invalid/impossible flag/configuration")
)

// commandLineArgs returns the arguments for Parse and ParseCommand to use.
// When flag.CommandLine has already been parsed (i.e. a prior flag.Parse call
// was made), it returns the remaining non-flag arguments from that parse so
// that confl composes with programs that do their own flag parsing first.
// When flag.CommandLine has not been parsed yet (the common case), it returns
// os.Args[1:] directly, which ensures that -h/-help and all other flags reach
// fs.Parse as expected.
func commandLineArgs() []string {
	if flag.CommandLine.Parsed() {
		return flag.CommandLine.Args()
	}
	return os.Args[1:]
}

// Parse populates cfg from os.Args[1:] (or flag.CommandLine.Args() when a
// prior flag.Parse call has already consumed some arguments) using struct field tags,
// then validates required fields. cfg must be a pointer to a struct. Supported tags:
//
//   - flag:”name”       flag name (required to register a field)
//   - default:”val”     default value as a string
//   - help:”text”       usage description
//   - short:”x”        single-letter alias registered alongside the long name
//   - required:”true”  field must be non-zero after parsing
//
// Anonymous (embedded) struct fields are traversed with a flat namespace: only
// the `flag:` tag on each leaf field determines the flag name.
//
// Named struct-typed fields behave differently depending on whether the field
// itself carries a `flag:` tag:
//
//   - With flag:”ns” tag: all leaf flags inside are registered as “-ns.<leaf>”,
//     e.g. a field `Server net \`flag:”srv”\” containing `Host string \`flag:”host”\”
//     produces the flag “-srv.host”.
//   - Without a flag: tag: the field name is ignored and leaf flags are
//     registered with a flat namespace (same as anonymous embedding).
//
// Namespaces nest: a named struct inside another named struct accumulates
// prefixes separated by “.”.
//
// Supported field types: string, bool, int, int64, uint, uint64, float64,
// time.Time, time.Duration, and slices thereof.
//
// Fields tagged cmd: are silently skipped; use ParseCommand or Dispatch for
// subcommand-aware parsing.
func Parse(cfg any) error { return conflagure(flag.CommandLine, cfg, commandLineArgs()) }

// Dispatch identifies the subcommand from flag.CommandLine.Args() (i.e. the
// remaining arguments after a prior call to Parse) and parses that subcommand's
// flags. It returns the selected Commander, or nil if no subcommand was
// matched and none is required. cfg must be the same pointer passed to Parse.
//
// Use ParseCommand when you want a single call that does both phases.
func Dispatch(cfg any) (Commander, error) {
	return dispatch(flag.CommandLine, cfg, flag.CommandLine.Args())
}

// ParseCommand parses global flags (via commandLineArgs) and then identifies
// and parses the subcommand, returning the selected Commander. It is equivalent
// to calling Parse followed by Dispatch.
//
// Returns (nil, nil) when no cmd: fields are defined or when the invocation
// contains only global flags and no subcommand is required.
//
// If cfg itself implements Commander and no subcommand is selected, cfg is
// returned as the Commander.
func ParseCommand(cfg any) (Commander, error) {
	return conflagureCmd(flag.CommandLine, cfg, commandLineArgs())
}
