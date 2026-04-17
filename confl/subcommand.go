package confl

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// Commander is the interface for subcommand structs used with ParseCommand.
// Flags are populated before Run is called; the caller is responsible for
// invoking Run on the returned Commander.
type Commander interface {
	Run(context.Context) error
}

var commanderType = reflect.TypeFor[Commander]()

type subcommandEntry struct {
	name string
	val  reflect.Value
	fs   *flag.FlagSet
	help string
}

// collectSubcommands scans the top-level struct val for fields tagged cmd: and
// returns one entry per field with a dedicated FlagSet already bound to the
// field's flags. programName is used as the FlagSet name prefix.
func collectSubcommands(val reflect.Value, programName string) ([]subcommandEntry, error) {
	// Pre-scan for narg:"rest" fields. rest + cmd: fields are incompatible.
	t0 := val.Type()
	var restFieldName string
	hasCmdField := false
	for i := range t0.NumField() {
		f := t0.Field(i)
		if f.Tag.Get("cmd") != "" {
			hasCmdField = true
		}
		if f.Tag.Get("narg") == "rest" {
			restFieldName = f.Name
		}
	}
	if restFieldName != "" && hasCmdField {
		return nil, ers.Wrapf(ErrInvalidSpecification,
			"field %q has narg:\"rest\" but struct also has cmd: fields; they are incompatible",
			restFieldName)
	}

	var entries []subcommandEntry
	t := val.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fval := val.Field(i)

		name := field.Tag.Get("cmd")
		if name == "" {
			continue
		}
		if _, hasFlag := field.Tag.Lookup("flag"); hasFlag {
			return nil, ers.Wrapf(ErrInvalidSpecification,
				"field %q has both flag: and cmd: tags; they are mutually exclusive",
				field.Name)
		}
		if !field.IsExported() {
			return nil, ers.Wrapf(ErrInvalidSpecification,
				"field %q with cmd: tag must be exported", field.Name)
		}
		if fval.Kind() != reflect.Struct {
			return nil, ers.Wrapf(ErrInvalidSpecification,
				"field %q has cmd: tag but is not a struct (got %s)",
				field.Name, fval.Kind())
		}

		subcmdFS := flag.NewFlagSet(joinStr(programName, " ", name), flag.ContinueOnError)
		if err := bindFlags(subcmdFS, fval, "", 1); err != nil {
			return nil, err
		}
		entries = append(entries, subcommandEntry{
			name: name,
			val:  fval,
			fs:   subcmdFS,
			help: field.Tag.Get("help"),
		})
	}
	return entries, nil
}

// validateCommanders checks that every subcommand entry implements Commander,
// recursing into nested subcommand structs. Used by conflagureCmd/ParseCommand
// to enforce the Commander contract at all levels.
func validateCommanders(entries []subcommandEntry) error {
	for _, e := range entries {
		addr := e.val.Addr()
		if !addr.Type().Implements(commanderType) && !e.val.Type().Implements(commanderType) {
			return ers.Wrapf(ErrInvalidSpecification,
				"subcommand %q type %s does not implement Commander",
				e.name, e.val.Type())
		}
		innerEntries, err := collectSubcommands(e.val, e.fs.Name())
		if err != nil {
			return err
		}
		if err := validateCommanders(innerEntries); err != nil {
			return err
		}
	}
	return nil
}

// selectSubcommand picks and parses the subcommand identified by remaining[0].
// Returns the pointer-to-subcommand-struct as any, or (nil, ers.ErrNotFound)
// when no subcommand name appears in remaining or no cmd: fields exist.
func selectSubcommand(entries []subcommandEntry, remaining []string) (any, error) {
	if len(entries) == 0 {
		return nil, ers.ErrNotFound
	}

	if len(remaining) == 0 {
		return nil, ers.ErrNotFound
	}

	subcmdName := remaining[0]
	subArgs := remaining[1:]

	var matched *subcommandEntry
	for i := range entries {
		if entries[i].name == subcmdName {
			matched = &entries[i]
			break
		}
	}
	if matched == nil {
		return nil, ers.Wrapf(ErrInvalidInput, "unknown subcommand %q", subcmdName)
	}

	untilFlags := collectUntilFlags(matched.val, "")
	if len(untilFlags) > 0 {
		subArgs = expandUntilArgs(subArgs, untilFlags)
	}

	if err := matched.fs.Parse(subArgs); err != nil {
		callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0)
		return nil, err
	}

	if err := populateRestField(matched.val, matched.fs.Args()); err != nil {
		return nil, err
	}

	if err := checkRequired(matched.val, ""); err != nil {
		return nil, err
	}

	// Try to recurse into nested subcommands. If the matched struct has cmd:
	// fields and a further subcommand name appears in the remaining args, select
	// it. A ErrNotFound result means no nested subcommand was named; return the
	// current level instead.
	innerEntries, err := collectSubcommands(matched.val, matched.fs.Name())
	if err != nil {
		return nil, err
	}
	if len(innerEntries) > 0 {
		inner, err := selectSubcommand(innerEntries, matched.fs.Args())
		if err != nil && !errors.Is(err, ers.ErrNotFound) {
			return nil, err
		}
		if err == nil {
			return inner, nil
		}
	}

	return matched.val.Addr().Interface(), nil
}

// dispatch performs a full two-phase parse: global flags then subcommand
// selection. It returns the selected subcommand struct pointer as any, or
// (nil, nil) when no subcommand was selected.
func dispatch(fs *flag.FlagSet, conf any, args []string) (any, error) {
	val, err := unwrapConf(conf)
	if err != nil {
		return nil, err
	}

	entries, err := collectSubcommands(val, fs.Name())
	if err != nil {
		return nil, err
	}

	if err := parseAndCheck(fs, val, args); err != nil {
		return nil, err
	}

	result, err := selectSubcommand(entries, fs.Args())
	if err != nil {
		if errors.Is(err, ers.ErrNotFound) {
			return nil, erc.Join(err, ErrDispatchNoSelection)
		}
		return nil, err
	}
	return result, nil
}

// conflagureCmd performs the full two-phase parse: global flags then subcommand
// selection and flag parsing. It installs a custom Usage function on fs that
// lists registered subcommands when there are any. All cmd: fields must
// implement Commander; an error is returned if any do not.
//
// conflagureCmd is validate → dispatch → cast: it validates the Commander
// contract, installs the Usage function, delegates parsing to dispatch, then
// casts the result to Commander.
func conflagureCmd(fs *flag.FlagSet, conf any, args []string) (Commander, error) {
	val, err := unwrapConf(conf)
	if err != nil {
		return nil, err
	}

	entries, err := collectSubcommands(val, fs.Name())
	if err != nil {
		return nil, err
	}

	if err := validateCommanders(entries); err != nil {
		return nil, err
	}

	if len(entries) > 0 {
		origUsage := fs.Usage
		fs.Usage = func() {
			if origUsage != nil {
				origUsage()
			} else {
				fmt.Fprintf(fs.Output(), "Usage of %s:\n", fs.Name())
				fs.PrintDefaults()
			}
			fmt.Fprintf(fs.Output(), "\nSubcommands:\n")
			for _, e := range entries {
				if e.help != "" {
					fmt.Fprintf(fs.Output(), "  %-16s %s\n", e.name, e.help)
				} else {
					fmt.Fprintf(fs.Output(), "  %s\n", e.name)
				}
			}
		}
	}

	result, err := dispatch(fs, conf, args)
	if err != nil {
		if errors.Is(err, ErrDispatchNoSelection) {
			// No subcommand named; return root Commander if conf implements it.
			if cmd, ok := conf.(Commander); ok {
				return cmd, nil
			}
		}
		return nil, err
	}

	return tryCastCommander(result)
}

func tryCastCommander[T any](result T) (Commander, error) {
	cmd, ok := any(result).(Commander)
	if !ok {
		return nil, ers.Wrapf(ErrInvalidSpecification,
			"selected subcommand type %T does not implement Commander", result)
	}
	return cmd, nil
}
