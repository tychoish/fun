package confl

import (
	"io"
	"slices"
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
