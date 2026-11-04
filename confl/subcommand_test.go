package confl

import (
	"context"
	"flag"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
)

// ── conflagureCmd: basic dispatch ─────────────────────────────────────────────

func Test_conflagureCmd_basic_dispatch(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose  bool            `flag:"verbose" short:"v"`
		Deploy   testDeployCmd   `cmd:"deploy"   help:"deploy to target"`
		Rollback testRollbackCmd `cmd:"rollback" help:"roll back a release"`
	}

	tests := []struct {
		name       string
		args       []string
		wantTarget string
		wantForce  bool
		wantCmd    string
	}{
		{
			name:       "deploy subcommand",
			args:       []string{"deploy", "-target=prod"},
			wantTarget: "prod",
			wantCmd:    "deploy",
		},
		{
			name:      "rollback with force",
			args:      []string{"rollback", "-version=v1.2", "-force"},
			wantForce: true,
			wantCmd:   "rollback",
		},
		{
			name:       "global flag then deploy",
			args:       []string{"-verbose", "deploy", "-target=staging"},
			wantTarget: "staging",
			wantCmd:    "deploy",
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			cmd, err := conflagureCmd(newTestFS(), &c, tt.args)
			assert.NotError(t, err)
			assert.True(t, cmd != nil)
			switch tt.wantCmd {
			case "deploy":
				_, ok := cmd.(*testDeployCmd)
				assert.True(t, ok)
				assert.Equal(t, c.Deploy.Target, tt.wantTarget)
			case "rollback":
				_, ok := cmd.(*testRollbackCmd)
				assert.True(t, ok)
				assert.Equal(t, c.Rollback.Force, tt.wantForce)
			}
		})
	}
}

func Test_conflagureCmd_no_subcommand_optional(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool          `flag:"verbose"`
		Deploy  testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"-verbose"})
	assert.ErrorIs(t, err, ErrDispatchNoSelection)
	assert.True(t, c.Verbose)
}

func Test_conflagureCmd_root_commander(t *testing.T) {
	t.Parallel()

	var c testRootCfg
	cmd, err := conflagureCmd(newTestFS(), &c, []string{"-verbose"})
	assert.NotError(t, err)
	assert.True(t, cmd != nil)
	_, ok := cmd.(*testRootCfg)
	assert.True(t, ok)
	assert.True(t, c.Verbose)
}

func Test_conflagureCmd_no_subcommand_named(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	assert.ErrorIs(t, err, ErrDispatchNoSelection)
}

func Test_conflagureCmd_unknown_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"bogus"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func Test_conflagureCmd_subcommand_required_flag_missing(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"deploy"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func Test_conflagureCmd_subcommand_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"deploy", "-target=prod", "-n"})
	assert.NotError(t, err)
	assert.True(t, c.Deploy.DryRun)
}

func Test_conflagureCmd_conf_errors(t *testing.T) {
	t.Parallel()

	t.Run("nil conf", func(t *testing.T) {
		_, err := conflagureCmd(newTestFS(), nil, nil)
		assert.Error(t, err)
	})

	t.Run("non-pointer conf", func(t *testing.T) {
		type cfg struct{ X string }
		_, err := conflagureCmd(newTestFS(), cfg{}, nil)
		assert.Error(t, err)
	})
}

// ── conflagureCmd: usage closure ──────────────────────────────────────────────

func Test_conflagureCmd_usage_closure(t *testing.T) {
	t.Parallel()

	t.Run("with help and no-help subcommands", func(t *testing.T) {
		type cfg struct {
			Deploy   testDeployCmd   `cmd:"deploy"   help:"deploy to target"`
			Rollback testRollbackCmd `cmd:"rollback"`
		}
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		var c cfg
		_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
		assert.Error(t, err)
	})

	t.Run("nil origUsage", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy" help:"deploy"`
		}
		fs := newTestFS()
		fs.Usage = nil
		fs.SetOutput(io.Discard)
		var c cfg
		_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
		assert.Error(t, err)
	})

	t.Run("preset origUsage is called", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy" help:"deploy"`
		}
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		customUsageCalled := false
		fs.Usage = func() { customUsageCalled = true }
		var c cfg
		_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
		assert.Error(t, err)
		assert.True(t, customUsageCalled)
	})
}

// ── conflagureCmd: error paths ────────────────────────────────────────────────

func Test_conflagureCmd_global_required_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Username string        `flag:"username" required:"true"`
		Deploy   testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func Test_conflagureCmd_errors(t *testing.T) {
	t.Parallel()

	t.Run("collectSubcommands error", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
		}
		var c cfg
		_, err := conflagureCmd(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("bindFlags error — invalid short flag", func(t *testing.T) {
		type cfg struct {
			Verbose bool          `flag:"verbose" short:"ab"` // short must be 1 char
			Deploy  testDeployCmd `cmd:"deploy"`
		}
		var c cfg
		_, err := conflagureCmd(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("global required flag missing", func(t *testing.T) {
		type cfg struct {
			Username string        `flag:"username" required:"true"`
			Deploy   testDeployCmd `cmd:"deploy"`
		}
		var c cfg
		_, err := conflagureCmd(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("subcommand parse error — unknown subcmd flag", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy"`
		}
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		var c cfg
		_, err := conflagureCmd(fs, &c, []string{"deploy", "-unknown-subcmd-flag"})
		assert.Error(t, err)
	})
}

// ── conflagureCmd: no commander ───────────────────────────────────────────────

func Test_conflagureCmd_no_commander(t *testing.T) {
	t.Parallel()

	type notCmd struct {
		Target string `flag:"target"`
	}
	type cfg struct {
		Deploy notCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// ── conflagureCmd: nested subcommands ─────────────────────────────────────────

func Test_conflagureCmd_nested_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool                 `flag:"verbose"`
		Deploy  testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	cmd, err := conflagureCmd(newTestFS(), &c, []string{"deploy", "sub", "-target=prod"})
	assert.NotError(t, err)
	_, ok := cmd.(*testDeployCmd)
	assert.True(t, ok)
	assert.Equal(t, c.Deploy.Sub.Target, "prod")
}

// ── collectSubcommands ────────────────────────────────────────────────────────

func Test_collectSubcommands_errors(t *testing.T) {
	t.Parallel()

	t.Run("field with both flag: and cmd: tags", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
		}
		var c cfg
		_, err := collectSubcommands(reflectVal(&c), "test")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("unexported cmd: field", func(t *testing.T) {
		type cfg struct {
			deploy testDeployCmd `cmd:"deploy"` //nolint:unused
		}
		var c cfg
		_, err := collectSubcommands(reflectVal(&c), "test")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("non-struct cmd: field", func(t *testing.T) {
		type cfg struct {
			Deploy string `cmd:"deploy"`
		}
		var c cfg
		_, err := collectSubcommands(reflectVal(&c), "test")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("non-Commander struct is accepted by collectSubcommands", func(t *testing.T) {
		type notCmd struct {
			Target string `flag:"target"`
		}
		type cfg struct {
			Deploy notCmd `cmd:"deploy"`
		}
		var c cfg
		entries, err := collectSubcommands(reflectVal(&c), "test")
		assert.NotError(t, err)
		assert.Equal(t, len(entries), 1)
	})
}

func Test_collectSubcommands_nested_cmd(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	entries, err := collectSubcommands(reflectVal(&c), "test")
	assert.NotError(t, err)
	assert.Equal(t, len(entries), 1)
	assert.Equal(t, entries[0].name, "deploy")
}

func Test_collectSubcommands_bindflags_err(t *testing.T) {
	t.Parallel()

	type badSub struct {
		//nolint:unused
		secret string `flag:"secret"` //nolint:structcheck
	}
	type cfg struct {
		Deploy badSub `cmd:"deploy"`
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// ── validateCommanders ────────────────────────────────────────────────────────

func Test_validateCommanders_inner_collectSubcommands_err(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testRestAndCmdSub `cmd:"deploy"`
	}
	var c cfg
	entries, err := collectSubcommands(reflectVal(&c), "test")
	assert.NotError(t, err)
	err = validateCommanders(entries)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_validateCommanders_recursive_err(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Mid testL2WithDeepBadSub `cmd:"mid"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// ── dispatch ──────────────────────────────────────────────────────────────────

func Test_dispatch_conf_errors(t *testing.T) {
	t.Parallel()

	t.Run("nil conf", func(t *testing.T) {
		_, err := dispatch(newTestFS(), nil, nil)
		assert.Error(t, err)
	})

	t.Run("non-pointer conf", func(t *testing.T) {
		type cfg struct{ X string }
		_, err := dispatch(newTestFS(), cfg{}, nil)
		assert.Error(t, err)
	})

	t.Run("no subcommands returns ErrDispatchNoSelection", func(t *testing.T) {
		type cfg struct {
			X string `flag:"x"`
		}
		var c cfg
		_, err := dispatch(newTestFS(), &c, nil)
		assert.ErrorIs(t, err, ErrDispatchNoSelection)
	})
}

func Test_dispatch_errors(t *testing.T) {
	t.Parallel()

	t.Run("collectSubcommands error", func(t *testing.T) {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
		}
		var c cfg
		_, err := dispatch(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("global parse error — unknown flag", func(t *testing.T) {
		type sub struct {
			X string `flag:"x"`
		}
		type cfg struct {
			Deploy sub `cmd:"deploy"`
		}
		var c cfg
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		_, err := dispatch(fs, &c, []string{"-no-such-flag"})
		assert.Error(t, err)
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		type sub struct {
			X string `flag:"x"`
		}
		type cfg struct {
			Deploy sub `cmd:"deploy"`
		}
		var c cfg
		_, err := dispatch(newTestFS(), &c, []string{"nosuchcmd"})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
}

// ── selectSubcommand: error paths ─────────────────────────────────────────────

func Test_selectSubcommand_inner_collectSubcommands_err(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testFlagAndCmdField `cmd:"deploy"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"deploy"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_selectSubcommand_recursive_unknown_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"deploy", "bogus"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── tryCastCommander ──────────────────────────────────────────────────────────

func Test_tryCastCommander(t *testing.T) {
	t.Parallel()

	t.Run("implements Commander returns it", func(t *testing.T) {
		var d testDeployCmd
		cmd, err := tryCastCommander(&d)
		assert.NotError(t, err)
		assert.True(t, cmd != nil)
		_, ok := cmd.(*testDeployCmd)
		assert.True(t, ok)
	})

	t.Run("does not implement Commander returns ErrInvalidSpecification", func(t *testing.T) {
		type plain struct{ X string }
		_, err := tryCastCommander(&plain{})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── built-in help subcommand ──────────────────────────────────────────────────

func Test_conflagureCmd_help_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose  bool            `flag:"verbose" help:"chatty output"`
		Deploy   testDeployCmd   `cmd:"deploy"   help:"deploy to target"`
		Rollback testRollbackCmd `cmd:"rollback" help:"roll back a release"`
	}

	tests := []struct {
		name    string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name: "all subcommands, each under a heading",
			args: []string{"help"},
			want: []string{"-verbose", "Subcommands:", "\n## deploy\n", "-target", "\n## rollback\n", "-version"},
		},
		{
			name:    "one subcommand",
			args:    []string{"help", "deploy"},
			want:    []string{"-target"},
			notWant: []string{"## ", "-verbose", "-version"},
		},
		{
			name: "after global flags",
			args: []string{"-verbose", "help", "rollback"},
			want: []string{"-version"},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			fs := newTestFS()
			fs.SetOutput(&out)
			var c cfg
			_, err := conflagureCmd(fs, &c, tt.args)
			assert.ErrorIs(t, err, ErrHelpShown)
			for want := range slices.Values(tt.want) {
				assert.True(t, strings.Contains(out.String(), want))
			}
			for bad := range slices.Values(tt.notWant) {
				assert.True(t, !strings.Contains(out.String(), bad))
			}
		})
	}
}

func Test_conflagureCmd_help_unknown_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}
	var c cfg
	fs := newTestFS()
	fs.SetOutput(io.Discard)
	_, err := conflagureCmd(fs, &c, []string{"help", "nope"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// A program's own help subcommand is dispatched, not shadowed.
func Test_conflagureCmd_help_user_defined(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Help testDeployCmd `cmd:"help"`
	}
	var c cfg
	cmd, err := conflagureCmd(newTestFS(), &c, []string{"help", "-target=x"})
	assert.NotError(t, err)
	_, ok := cmd.(*testDeployCmd)
	assert.True(t, ok)
	assert.Equal(t, c.Help.Target, "x")
}

// With no subcommands declared, "help" is an ordinary positional argument.
func Test_dispatch_help_without_subcommands(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool `flag:"verbose"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"help"})
	assert.True(t, !isErr(err, ErrHelpShown))
}

// The program name in help output is the base name, so a binary built into a
// temporary directory by go run produces the same text on every run.
func Test_conflagureCmd_help_stable_program_name(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}
	for _, args := range [][]string{{"help"}, {"help", "deploy"}} {
		var out strings.Builder
		fs := flag.NewFlagSet("/tmp/go-build123/b001/exe/prog", flag.ContinueOnError)
		fs.SetOutput(&out)
		var c cfg
		_, err := conflagureCmd(fs, &c, args)
		assert.ErrorIs(t, err, ErrHelpShown)
		assert.True(t, strings.Contains(out.String(), "Usage of prog"))
		assert.True(t, !strings.Contains(out.String(), "/tmp"))
	}
}

type testGroupCmd struct {
	Name string        `flag:"name" help:"group name"`
	Sub  testDeployCmd `cmd:"sub"`
}

func (testGroupCmd) Run(context.Context) error { return nil }

func Test_help_nested_subcommands(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Group testGroupCmd `cmd:"group" help:"a group"`
	}
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"help"}, []string{"\n## group\n", "-name", "\n## group sub\n", "-target"}},
		{[]string{"help", "group"}, []string{"Usage of prog group:", "-name"}},
		{[]string{"help", "group", "sub"}, []string{"Usage of prog group sub:", "-target"}},
	}
	for tt := range slices.Values(tests) {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var out strings.Builder
			fs := flag.NewFlagSet("/tmp/x/prog", flag.ContinueOnError)
			fs.SetOutput(&out)
			_, err := conflagureCmd(fs, &cfg{}, tt.args)
			assert.ErrorIs(t, err, ErrHelpShown)
			for want := range slices.Values(tt.want) {
				assert.True(t, strings.Contains(out.String(), want))
			}
		})
	}

	var out strings.Builder
	fs := newTestFS()
	fs.SetOutput(&out)
	_, err := conflagureCmd(fs, &cfg{}, []string{"help", "group", "nope"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// dispatch does not validate nested specifications, so help meets them.
func Test_help_invalid_nested_specification(t *testing.T) {
	t.Parallel()

	type group struct {
		Bad int `cmd:"bad"`
	}
	type mid struct {
		Bad int `cmd:"bad"`
	}
	type deep struct {
		Mid mid `cmd:"mid"`
	}
	type shallowCfg struct {
		Group group `cmd:"group"`
	}
	type deepCfg struct {
		Deep deep `cmd:"deep"`
	}
	tests := []struct {
		conf any
		args []string
	}{
		{&shallowCfg{}, []string{"help"}},
		{&shallowCfg{}, []string{"help", "group", "bad"}},
		{&deepCfg{}, []string{"help"}},
	}
	for tt := range slices.Values(tests) {
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		_, err := dispatch(fs, tt.conf, tt.args)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	}
}

// ParseCommand and Dispatch exit with status 0 after the help subcommand
// prints, as for -help. The test re-runs itself to observe the exit; the
// testing package has already parsed flag.CommandLine, so "help" is left in
// its arguments.
func Test_help_subcommand_exits_zero(t *testing.T) {
	t.Parallel()

	if entry := os.Getenv("CONFL_HELP_EXIT"); entry != "" {
		type cfg struct {
			Deploy testDeployCmd `cmd:"deploy"`
		}
		if entry == "dispatch" {
			_, _ = Dispatch(&cfg{})
		} else {
			_, _ = ParseCommand(&cfg{})
		}
		os.Exit(3)
	}

	for _, entry := range []string{"dispatch", "parsecommand"} {
		cmd := exec.Command(os.Args[0], "-test.run=^Test_help_subcommand_exits_zero$", "help")
		cmd.Env = append(os.Environ(), "CONFL_HELP_EXIT="+entry)
		out, err := cmd.CombinedOutput()
		assert.NotError(t, err)
		assert.True(t, strings.Contains(string(out), "## deploy"))
	}
}
