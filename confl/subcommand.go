package confl

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/tychoish/fun/ers"
)

// Commander is implemented by every subcommand struct that can be dispatched
// to. Parse/conflagure populate the struct's flags first, then the caller is
// responsible for invoking Run.
type Commander interface {
	Run(context.Context) error
}

var commanderType = reflect.TypeFor[Commander]()

type subcommandEntry struct {
	name     string
	val      reflect.Value
	fs       *flag.FlagSet
	required bool
	help     string
}

// collectSubcommands scans the top-level struct val for fields tagged cmd: and
// returns one entry per field with a dedicated FlagSet already bound to the
// field's flags. programName is used as the FlagSet name prefix.
func collectSubcommands(val reflect.Value, programName string) ([]subcommandEntry, error) {
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
		addr := fval.Addr()
		if !addr.Type().Implements(commanderType) && !fval.Type().Implements(commanderType) {
			return nil, ers.Wrapf(ErrInvalidSpecification,
				"field %q has cmd: tag but type %s does not implement Commander",
				field.Name, fval.Type())
		}

		subcmdFS := flag.NewFlagSet(joinStr(programName, " ", name), flag.ContinueOnError)
		if err := bindFlags(subcmdFS, fval, "", 1); err != nil {
			return nil, err
		}
		entries = append(entries, subcommandEntry{
			name:     name,
			val:      fval,
			fs:       subcmdFS,
			required: field.Tag.Get("required") == "true",
			help:     field.Tag.Get("help"),
		})
	}
	return entries, nil
}

// dispatchEntries selects and parses the subcommand identified by remaining[0],
// then validates required fields. Returns the matched Commander, the root
// Commander (if conf implements it and no subcommand was selected), or nil.
func dispatchEntries(conf any, entries []subcommandEntry, remaining []string) (Commander, error) {
	if len(entries) == 0 {
		return nil, ers.ErrNotFound
	}

	if len(remaining) == 0 {
		for _, e := range entries {
			if e.required {
				names := make([]string, len(entries))
				for i, entry := range entries {
					names[i] = entry.name
				}
				return nil, ers.Wrapf(ErrInvalidInput,
					"subcommand required; one of: %s", strings.Join(names, ", "))
			}
		}
		// No subcommand selected; return root Commander if the struct implements it.
		if cmd, ok := conf.(Commander); ok {
			return cmd, nil
		}
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

	if err := matched.fs.Parse(subArgs); err != nil {
		callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0)
		return nil, err
	}

	if err := checkRequired(matched.val, ""); err != nil {
		return nil, err
	}

	// addr is always a pointer; since T implements Commander implies *T does too,
	// this type assertion is always safe after the collectSubcommands check.
	return matched.val.Addr().Interface().(Commander), nil
}

// dispatch validates conf, collects subcommands, and delegates to
// dispatchEntries. remaining should be fs.Args() after the global parse.
func dispatch(fs *flag.FlagSet, conf any, remaining []string) (Commander, error) {
	if conf == nil {
		return nil, ers.Wrap(ErrInvalidSpecification, "conf must be a pointer to a struct, got nil")
	}
	t := reflect.TypeOf(conf)
	v := reflect.ValueOf(conf)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil, ers.Wrapf(ErrInvalidSpecification, "conf must be a pointer to a struct, got %T", conf)
	}

	entries, err := collectSubcommands(v.Elem(), fs.Name())
	if err != nil {
		return nil, err
	}

	return dispatchEntries(conf, entries, remaining)
}

// conflagureCmd performs the full two-phase parse: global flags then subcommand
// selection and flag parsing. It installs a custom Usage function on fs that
// lists registered subcommands when there are any.
func conflagureCmd(fs *flag.FlagSet, conf any, args []string) (Commander, error) {
	if conf == nil {
		return nil, ers.Wrap(ErrInvalidSpecification, "conf must be a pointer to a struct, got nil")
	}
	t := reflect.TypeOf(conf)
	v := reflect.ValueOf(conf)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil, ers.Wrapf(ErrInvalidSpecification, "conf must be a pointer to a struct, got %T", conf)
	}
	val := v.Elem()

	entries, err := collectSubcommands(val, fs.Name())
	if err != nil {
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

	if err := bindFlags(fs, val, "", 0); err != nil {
		return nil, err
	}

	if err := fs.Parse(args); err != nil {
		callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0)
		return nil, err
	}

	if err := checkRequired(val, ""); err != nil {
		return nil, err
	}

	return dispatchEntries(conf, entries, fs.Args())
}
