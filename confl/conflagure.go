// Package confl populates Go structs from command-line flags using struct tags.
// Use Parse for flat flag parsing, ParseCommand or Dispatch for subcommand
// dispatch, and Validate to catch tag errors at program startup.
//
// # Struct tags
//
//   - flag:"name"      register the field as a flag (required to participate)
//   - default:"val"    default value; confl parses it like any flag value
//   - help:"text"      usage string; -help output displays it
//   - short:"x"        single-character alias (e.g. short:"v" adds -v)
//   - required:"true"  field must be non-zero after parsing
//   - sep:"<s>"        element separator for slice flags and their defaults
//   - narg:"rest"      collect all remaining positional args into a slice field
//   - narg:"until"     collect args until the next flag (requires flag:)
//   - cmd:"name"       declare a subcommand field (use ParseCommand or Dispatch)
//   - format:"layout"  Go reference-time layout string for time.Time fields
//
// # Supported types
//
// string, bool, int, int64, uint, uint64, float64, time.Duration, time.Time,
// any type implementing flag.Value, and slices of each scalar type.
//
// # Namespacing
//
// A named struct field without a flag: tag merges its leaf flags into the
// parent prefix (flat traversal). A named struct field with a flag: tag creates
// a namespace: flag:"srv" with a nested flag:"host" produces -srv.host.
//
// # Slices
//
// Without sep:, each flag invocation appends one element. With a non-empty
// sep:, a single invocation like -tags a:b:c (sep:":") appends three elements.
// Slice fields with a non-empty default: must carry a sep: tag. The first
// command-line invocation of a slice flag clears its defaults.
//
// # Subcommands
//
// Tag exported struct fields cmd:"<name>" to declare subcommands. Global flags
// belong to the root struct; each subcommand struct carries its own flag: tags.
//
//	type CLI struct {
//	    Verbose bool  `flag:"verbose" short:"v"`
//	    Serve   `cmd:"serve"   help:"start the server"`
//	}
//
// Use ParseCommand when all subcommand structs implement Commander:
//
//	cmd, err := confl.ParseCommand(&CLI{})
//	if err != nil { log.Fatal(err) }
//	cmd.Run(ctx)
//
// Use Dispatch when subcommand structs do not implement Commander; it returns
// the selected subcommand as any for the caller to type-switch on.
//
// Dispatch and ParseCommand return ErrDispatchNoSelection when the user
// provides no subcommand name. Subcommands nest to arbitrary depth.
package confl

import (
	"flag"
	"os"

	"github.com/tychoish/fun/ers"
)

const (
	// ErrInvalidSpecification signals a programming error in struct tags.
	ErrInvalidSpecification = ers.Error("incorrect flag/configuration specification")
	// ErrInvalidInput signals an unparseable user-supplied flag value.
	ErrInvalidInput = ers.Error("received invalid/impossible flag/configuration")
	// ErrDispatchNoSelection is returned by Dispatch and ParseCommand when
	// the caller provided no subcommand name.
	ErrDispatchNoSelection = ers.Error("no subcommand selected")
)

// commandLineArgs returns os.Args[1:], or flag.CommandLine.Args() when a prior
// flag.Parse call already parsed flag.CommandLine.
func commandLineArgs() []string {
	if flag.CommandLine.Parsed() {
		return flag.CommandLine.Args()
	}
	return os.Args[1:]
}

// Parse populates cfg from command-line arguments. cfg must be a pointer to a
// struct. See the package documentation for supported struct tags and types.
// confl ignores fields tagged cmd:; use ParseCommand or Dispatch for
// subcommand dispatch.
func Parse(cfg any) error { return conflagure(flag.CommandLine, cfg, commandLineArgs()) }

// Dispatch parses global flags and selects a subcommand, returning the chosen
// subcommand struct pointer as any. Callers type-switch on the result to
// identify the selected subcommand. Subcommand types need not implement
// Commander; use ParseCommand to require that guarantee.
func Dispatch(cfg any) (any, error) {
	return dispatch(flag.CommandLine, cfg, commandLineArgs())
}

// ParseCommand parses global flags and selects a subcommand, returning it as a
// Commander. All cmd: fields must implement Commander; ParseCommand returns an
// error if any do not. When no subcommand is named and cfg itself implements
// Commander, ParseCommand returns cfg. ParseCommand returns ErrDispatchNoSelection
// when no subcommand is named and cfg does not implement Commander.
func ParseCommand(cfg any) (Commander, error) {
	return conflagureCmd(flag.CommandLine, cfg, commandLineArgs())
}
