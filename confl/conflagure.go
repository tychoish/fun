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
//   - env:"VAR1,VAR2"  env var names to check; requires flag:
//   - opts:"..."       comma-separated options controlling env var behaviour
//
// # Environment variables
//
// A field tagged env:"VAR" is populated from the environment when no CLI flag
// was provided. Multiple names may be listed: env:"PRIMARY,FALLBACK". By
// default the first variable that is set (as reported by os.LookupEnv) wins,
// regardless of its value. The CLI flag always takes priority over any env var.
//
// The opts: tag accepts a comma-separated list of the following options; order
// does not matter, and options may be freely combined:
//
//   - env-nonempty-only      skip env vars that are set but have an empty value;
//     the search continues to the next name in the list
//   - env-last-wins          use the last set var in the list instead of the first;
//     combined with env-nonempty-only this gives the last
//     non-empty var
//   - env-takes-priority     env var wins over the CLI flag when both are provided;
//     the CLI value is silently ignored
//   - env-or-cli             either source may be used, but providing both is an
//     error; an empty env var counts as unset when combined
//     with env-nonempty-only
//   - env-exclusive          the CLI flag is never accepted; any CLI value is an
//     error regardless of whether the env var is set
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
//
// # Composing multiple config owners
//
// Parse binds and parses flag.CommandLine in one call, which is only safe
// when a single struct owns the entire command line: calling Parse a second
// time for an unrelated struct parses argv again before the second struct's
// flags exist, so any of its flags present on the command line fail with
// "flag provided but not defined". When two independent packages each need
// to contribute flags to the same process, either embed both structs into
// one and call Parse once, or, when the structs cannot be embedded (e.g. one
// is owned by a shared library that already has its own Config type), collect
// them in a Registry and finish with a single ParseAll:
//
//	var reg confl.Registry
//	if err := reg.Register(&dbCfg); err != nil { ... }
//	if err := reg.Register(&serverCfg); err != nil { ... }
//	if err := reg.ParseAll(); err != nil { ... }
//
// Register only binds flags; it never touches argv, so calling it any number
// of times in any order, on the zero value of Registry, is safe. ParseAll
// parses flag.CommandLine exactly once and then finalizes (narg:"rest", env
// vars, required-field checks) every struct the Registry collected, in
// registration order.
package confl

import (
	"flag"
	"os"
	"reflect"

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

// Registry collects structs bound via Register for later finalization by
// ParseAll. The zero value is ready to use; a Registry is not safe for
// concurrent use, matching every other confl entry point (all of them expect
// to run once, sequentially, during process startup).
type Registry struct {
	vals []reflect.Value
}

// Register binds cfg's flag:-tagged fields onto flag.CommandLine without
// parsing the command line, and queues cfg in r for finalization by the next
// ParseAll call. cfg must be a pointer to a struct; see the package
// documentation for supported struct tags and types. Call Register once per
// independent config struct that needs to share the process's command line —
// unlike Parse, Register never touches argv, so calling it repeatedly on the
// same Registry, for distinct structs, is safe regardless of order.
func (r *Registry) Register(cfg any) error {
	val, err := unwrapConf(cfg)
	if err != nil {
		return err
	}
	if err := bindFlags(flag.CommandLine, val, "", 0); err != nil {
		return err
	}
	r.vals = append(r.vals, val)
	return nil
}

// ParseAll parses flag.CommandLine's arguments and finalizes every struct r
// collected: applying narg:"rest" and env: tag values, and checking
// required:"true" fields, in registration order. Call it exactly once, after
// every participant has called Register on r. A struct that declares
// narg:"rest" receives the full set of leftover positional arguments; when
// more than one registered struct declares narg:"rest", each gets an
// independent copy.
func (r *Registry) ParseAll() error {
	return parseAndFinalize(flag.CommandLine, r.vals, commandLineArgs())
}

// Parse populates cfg from command-line arguments. cfg must be a pointer to a
// struct. See the package documentation for supported struct tags and types.
// confl ignores fields tagged cmd:; use ParseCommand or Dispatch for
// subcommand dispatch. Parse binds and parses in a single call, so it is only
// safe when cfg is the sole owner of the command line; see Registry for
// composing multiple independent config structs.
func Parse(cfg any) error {
	var reg Registry
	if err := reg.Register(cfg); err != nil {
		return err
	}
	return reg.ParseAll()
}

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
