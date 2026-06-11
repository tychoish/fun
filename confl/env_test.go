package confl

import (
	"slices"
	"testing"

	"github.com/tychoish/fun/assert"
)

// ── basic env var resolution ─────────────────────────────────────────────────

func Test_env_basic(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_HOST" flag:"host"`
	}

	tests := []struct {
		name   string
		envVal string
		envSet bool
		args   []string
		want   string
	}{
		{name: "env var set", envVal: "from-env", envSet: true, want: "from-env"},
		{name: "cli overrides env", envVal: "from-env", envSet: true, args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "neither set", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_HOST", tt.envVal)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

func Test_env_with_default(t *testing.T) {
	type cfg struct {
		Host string `default:"localhost" env:"CONFL_TEST_HOST2" flag:"host"`
	}

	tests := []struct {
		name   string
		envVal string
		envSet bool
		args   []string
		want   string
	}{
		{name: "env overrides default", envVal: "from-env", envSet: true, want: "from-env"},
		{name: "cli overrides env and default", envVal: "from-env", envSet: true, args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "default when neither set", want: "localhost"},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_HOST2", tt.envVal)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// ── multiple env vars, first-wins ────────────────────────────────────────────

func Test_env_multiple_first_wins(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_HOST_A,CONFL_TEST_HOST_B" flag:"host"`
	}

	tests := []struct {
		name string
		setA string
		setB string
		want string
	}{
		{name: "only A set", setA: "from-A", want: "from-A"},
		{name: "only B set", setB: "from-B", want: "from-B"},
		{name: "both set, A wins", setA: "from-A", setB: "from-B", want: "from-A"},
		{name: "neither set", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setA != "" {
				t.Setenv("CONFL_TEST_HOST_A", tt.setA)
			}
			if tt.setB != "" {
				t.Setenv("CONFL_TEST_HOST_B", tt.setB)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, nil))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// ── last-wins ────────────────────────────────────────────────────────────────

func Test_env_last_wins(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_LW_A,CONFL_TEST_LW_B" flag:"host" opts:"env-last-wins"`
	}

	tests := []struct {
		name string
		setA string
		setB string
		want string
	}{
		{name: "only A set", setA: "from-A", want: "from-A"},
		{name: "only B set", setB: "from-B", want: "from-B"},
		{name: "both set, B wins", setA: "from-A", setB: "from-B", want: "from-B"},
		{name: "neither set", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setA != "" {
				t.Setenv("CONFL_TEST_LW_A", tt.setA)
			}
			if tt.setB != "" {
				t.Setenv("CONFL_TEST_LW_B", tt.setB)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, nil))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// ── env-nonempty-only ────────────────────────────────────────────────────────

func Test_env_nonempty_only(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_IE_A,CONFL_TEST_IE_B" flag:"host" opts:"env-nonempty-only"`
	}

	tests := []struct {
		name string
		setA *string // nil means unset; pointer-to-string allows setting ""
		setB *string
		want string
	}{
		{name: "A empty, B set — B used", setA: ptrStr(""), setB: ptrStr("from-B"), want: "from-B"},
		{name: "A set, non-empty — A used", setA: ptrStr("from-A"), setB: ptrStr("from-B"), want: "from-A"},
		{name: "both empty — zero", setA: ptrStr(""), setB: ptrStr(""), want: ""},
		{name: "neither set — zero", want: ""},
		{name: "A empty, B unset — zero", setA: ptrStr(""), want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setA != nil {
				t.Setenv("CONFL_TEST_IE_A", *tt.setA)
			}
			if tt.setB != nil {
				t.Setenv("CONFL_TEST_IE_B", *tt.setB)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, nil))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

func Test_env_no_ignore_empty_applies_empty(t *testing.T) {
	type cfg struct {
		Host string `default:"localhost" env:"CONFL_TEST_NONIE" flag:"host"`
	}

	t.Setenv("CONFL_TEST_NONIE", "")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Host, "") // empty env var overrides default
}

// ── env-or-cli ───────────────────────────────────────────────────────────────

func Test_env_or_cli(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_EOC" flag:"host" opts:"env-or-cli"`
	}

	tests := []struct {
		name    string
		envSet  bool
		envVal  string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "only env — ok", envSet: true, envVal: "from-env", want: "from-env"},
		{name: "only cli — ok", args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "both — error", envSet: true, envVal: "from-env", args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "neither — ok", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_EOC", tt.envVal)
			}
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidInput)
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

func Test_env_or_cli_short(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_EOC_SHORT" flag:"host" opts:"env-or-cli" short:"H"`
	}

	t.Setenv("CONFL_TEST_EOC_SHORT", "from-env")
	var c cfg
	err := conflagure(newTestFS(), &c, []string{"-H", "from-cli"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── env-takes-priority ───────────────────────────────────────────────────────

func Test_env_takes_priority(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_ETP" flag:"host" opts:"env-takes-priority"`
	}

	tests := []struct {
		name   string
		envSet bool
		envVal string
		args   []string
		want   string
	}{
		{name: "env set, no cli — env wins", envSet: true, envVal: "from-env", want: "from-env"},
		{name: "env set, cli also set — env wins silently", envSet: true, envVal: "from-env", args: []string{"-host", "from-cli"}, want: "from-env"},
		{name: "env not set, cli set — cli used", args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "neither set — zero", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_ETP", tt.envVal)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

func Test_env_takes_priority_short(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_ETP_SHORT" flag:"host" opts:"env-takes-priority" short:"H"`
	}

	t.Setenv("CONFL_TEST_ETP_SHORT", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-H", "from-cli"}))
	assert.Equal(t, c.Host, "from-env")
}

// ── env-exclusive ────────────────────────────────────────────────────────────

func Test_env_exclusive(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_EX" flag:"host" opts:"env-exclusive"`
	}

	tests := []struct {
		name    string
		envSet  bool
		envVal  string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "env set, no cli — ok", envSet: true, envVal: "from-env", want: "from-env"},
		{name: "cli set, no env — error", args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "both set — error", envSet: true, envVal: "from-env", args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "neither set — ok", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_EX", tt.envVal)
			}
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidInput)
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

func Test_env_exclusive_short(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_EX_SHORT" flag:"host" opts:"env-exclusive" short:"H"`
	}

	// Short flag also rejected under env-exclusive.
	var c cfg
	err := conflagure(newTestFS(), &c, []string{"-H", "from-cli"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── option combinations ───────────────────────────────────────────────────────

// Test_env_nonempty_last_wins verifies env-nonempty-only combined with
// env-last-wins: the last *non-empty* env var in the list wins.
func Test_env_nonempty_last_wins(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_NLW_A,CONFL_TEST_NLW_B" flag:"host" opts:"env-nonempty-only,env-last-wins"`
	}

	tests := []struct {
		name string
		setA *string
		setB *string
		want string
	}{
		{name: "both non-empty — B wins (last)", setA: ptrStr("from-A"), setB: ptrStr("from-B"), want: "from-B"},
		{name: "B empty, A non-empty — A used (last non-empty)", setA: ptrStr("from-A"), setB: ptrStr(""), want: "from-A"},
		{name: "A empty, B non-empty — B wins", setA: ptrStr(""), setB: ptrStr("from-B"), want: "from-B"},
		{name: "both empty — zero", setA: ptrStr(""), setB: ptrStr(""), want: ""},
		{name: "only A set non-empty — A used", setA: ptrStr("from-A"), want: "from-A"},
		{name: "only B set non-empty — B used", setB: ptrStr("from-B"), want: "from-B"},
		{name: "neither set — zero", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setA != nil {
				t.Setenv("CONFL_TEST_NLW_A", *tt.setA)
			}
			if tt.setB != nil {
				t.Setenv("CONFL_TEST_NLW_B", *tt.setB)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, nil))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// Test_env_option_order verifies that opts order does not affect behaviour.
func Test_env_option_order(t *testing.T) {
	type cfgAB struct {
		Host string `env:"CONFL_TEST_ORD_A,CONFL_TEST_ORD_B" flag:"host" opts:"env-nonempty-only,env-last-wins"`
	}
	type cfgBA struct {
		Host string `env:"CONFL_TEST_ORD_A,CONFL_TEST_ORD_B" flag:"host" opts:"env-last-wins,env-nonempty-only"`
	}

	// A=empty, B=value; last non-empty wins → B.
	t.Setenv("CONFL_TEST_ORD_A", "")
	t.Setenv("CONFL_TEST_ORD_B", "from-B")

	var ab cfgAB
	assert.NotError(t, conflagure(newTestFS(), &ab, nil))

	var ba cfgBA
	assert.NotError(t, conflagure(newTestFS(), &ba, nil))

	assert.Equal(t, ab.Host, ba.Host)
	assert.Equal(t, ab.Host, "from-B")
}

// Test_env_nonempty_or_cli verifies env-nonempty-only combined with env-or-cli:
// an empty env var is treated as unset, so CLI is not blocked by it.
func Test_env_nonempty_or_cli(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_NOC" flag:"host" opts:"env-nonempty-only,env-or-cli"`
	}

	tests := []struct {
		name    string
		envVal  *string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "non-empty env, no cli — ok", envVal: ptrStr("from-env"), want: "from-env"},
		{name: "non-empty env + cli — error", envVal: ptrStr("from-env"), args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "empty env + cli — cli used (empty env treated as unset)", envVal: ptrStr(""), args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "only cli, no env — ok", args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "neither — zero", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != nil {
				t.Setenv("CONFL_TEST_NOC", *tt.envVal)
			}
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidInput)
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// Test_env_nonempty_takes_priority verifies env-nonempty-only combined with
// env-takes-priority: only non-empty env vars override CLI.
func Test_env_nonempty_takes_priority(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_NTP" flag:"host" opts:"env-nonempty-only,env-takes-priority"`
	}

	tests := []struct {
		name   string
		envVal *string
		args   []string
		want   string
	}{
		{name: "non-empty env + cli — env wins", envVal: ptrStr("from-env"), args: []string{"-host", "from-cli"}, want: "from-env"},
		{name: "empty env + cli — cli wins (empty env skipped)", envVal: ptrStr(""), args: []string{"-host", "from-cli"}, want: "from-cli"},
		{name: "non-empty env, no cli — env used", envVal: ptrStr("from-env"), want: "from-env"},
		{name: "neither — zero", want: ""},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != nil {
				t.Setenv("CONFL_TEST_NTP", *tt.envVal)
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// Test_env_nonempty_exclusive verifies env-nonempty-only combined with
// env-exclusive: CLI is always rejected regardless of whether the env var is empty.
func Test_env_nonempty_exclusive(t *testing.T) {
	type cfg struct {
		Host string `env:"CONFL_TEST_NEX" flag:"host" opts:"env-nonempty-only,env-exclusive"`
	}

	tests := []struct {
		name    string
		envVal  *string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "non-empty env, no cli — ok", envVal: ptrStr("from-env"), want: "from-env"},
		{name: "empty env, no cli — zero (empty skipped)", envVal: ptrStr(""), want: ""},
		{name: "cli set, no env — error", args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "cli set, empty env — error (cli still rejected)", envVal: ptrStr(""), args: []string{"-host", "from-cli"}, wantErr: true},
		{name: "cli set, non-empty env — error", envVal: ptrStr("from-env"), args: []string{"-host", "from-cli"}, wantErr: true},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != nil {
				t.Setenv("CONFL_TEST_NEX", *tt.envVal)
			}
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidInput)
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c.Host, tt.want)
		})
	}
}

// ── validation errors ─────────────────────────────────────────────────────────

func Test_env_validate_env_without_flag(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `env:"CONFL_TEST_VAL_HOST"` // no flag: tag
	}

	err := Validate(&cfg{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_env_validate_empty_env_tag(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `env:"" flag:"host"`
	}

	err := Validate(&cfg{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_env_validate_opts_without_env(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `flag:"host" opts:"env-nonempty-only"` // opts: without env: → invalid
	}

	err := Validate(&cfg{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_env_validate_unknown_opts(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Host string `env:"CONFL_TEST_V" flag:"host" opts:"nonexistent-option"`
	}

	err := Validate(&cfg{})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

func Test_env_validate_valid_opts(t *testing.T) {
	t.Parallel()

	// Exercise isKnownEnvOpt's return-true path: all known option values must
	// pass validation without error.
	type cfg struct {
		A string `env:"CONFL_TEST_VAL_A" flag:"a" opts:"env-nonempty-only"`
		B string `env:"CONFL_TEST_VAL_B" flag:"b" opts:"env-takes-priority"`
		C string `env:"CONFL_TEST_VAL_C" flag:"c" opts:"env-or-cli"`
		D string `env:"CONFL_TEST_VAL_D" flag:"d" opts:"env-exclusive"`
		E string `env:"CONFL_TEST_VAL_E" flag:"e" opts:"env-last-wins"`
	}

	assert.NotError(t, Validate(&cfg{}))
}

// ── struct walk correctness ───────────────────────────────────────────────────

func Test_env_anonymous_embedded(t *testing.T) {
	type Base struct {
		Host string `env:"CONFL_TEST_ANON_HOST" flag:"host"`
	}
	type cfg struct {
		Base
	}

	t.Setenv("CONFL_TEST_ANON_HOST", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Host, "from-env")
}

func Test_env_named_substruct_flat(t *testing.T) {
	type Sub struct {
		Host string `env:"CONFL_TEST_FLAT_HOST" flag:"host"`
	}
	type cfg struct {
		Sub Sub // no flag: tag on Sub → flat namespace
	}

	t.Setenv("CONFL_TEST_FLAT_HOST", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Sub.Host, "from-env")
}

func Test_env_named_substruct_namespaced(t *testing.T) {
	type Sub struct {
		Host string `env:"CONFL_TEST_NS_HOST" flag:"host"`
	}
	type cfg struct {
		Sub Sub `flag:"srv"` // creates prefix "srv."
	}

	t.Setenv("CONFL_TEST_NS_HOST", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Sub.Host, "from-env")
}

func Test_env_cmd_field_not_touched(t *testing.T) {
	type Sub struct {
		Host string `env:"CONFL_TEST_CMD_HOST" flag:"host"`
	}
	type cfg struct {
		Sub Sub `cmd:"sub"`
	}

	// cmd: fields are skipped by applyEnvVars.
	t.Setenv("CONFL_TEST_CMD_HOST", "should-not-apply")
	var c cfg
	_ = conflagure(newTestFS(), &c, nil)
	assert.Equal(t, c.Sub.Host, "")
}

// Test_env_unexported_struct_field covers the !field.IsExported() branch in
// applyEnvVarsWalk: an unexported embedded struct whose exported fields carry
// flag: and env: tags must still be populated from the environment.
func Test_env_unexported_struct_field(t *testing.T) {
	type inner struct {
		Host string `env:"CONFL_TEST_UNEXP_HOST" flag:"host"`
	}
	type cfg struct {
		inner // unexported anonymous embedding
	}

	t.Setenv("CONFL_TEST_UNEXP_HOST", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Host, "from-env")
}

// Test_env_flag_value_struct covers the flag.Value fall-through branch in
// applyEnvVarsWalk: a struct field whose pointer type implements flag.Value is
// treated as a leaf and the env var value is applied via its Set method.
func Test_env_flag_value_struct(t *testing.T) {
	type cfg struct {
		V testFlagValue `env:"CONFL_TEST_FV" flag:"v"`
	}

	t.Setenv("CONFL_TEST_FV", "from-env")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, len(c.V.vals), 1)
	assert.Equal(t, c.V.vals[0], "from-env")
}

// Test_env_inverted_bool_skipped covers the f == nil branch in
// applyEnvVarsWalk: a bool field with default:"true" registers as "no-<name>",
// so fs.Lookup(name) returns nil and the env var is silently ignored.
func Test_env_inverted_bool_skipped(t *testing.T) {
	type cfg struct {
		Verbose bool `default:"true" env:"CONFL_TEST_INV_BOOL" flag:"verbose"`
	}

	// Env var is set, but the flag was registered as "no-verbose" (inverted).
	// The walk skips silently; the field keeps its default value of true.
	t.Setenv("CONFL_TEST_INV_BOOL", "false")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Verbose, true)
}

// Test_env_unexported_struct_error covers the return err path inside the
// !field.IsExported() branch of applyEnvVarsWalk: an invalid env var value for
// a typed field inside an unexported struct must propagate the error up.
func Test_env_unexported_struct_error(t *testing.T) {
	type inner struct {
		Count int `env:"CONFL_TEST_UNEXP_ERR" flag:"count"`
	}
	type cfg struct {
		inner
	}

	t.Setenv("CONFL_TEST_UNEXP_ERR", "not-a-number")
	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// Test_env_exported_struct_error covers the return err path inside the
// exported-struct recursion branch of applyEnvVarsWalk: an invalid env var
// value for a typed field inside a named exported struct must propagate up.
func Test_env_exported_struct_error(t *testing.T) {
	type Sub struct {
		Count int `env:"CONFL_TEST_EXP_ERR" flag:"count"`
	}
	type cfg struct {
		Sub Sub
	}

	t.Setenv("CONFL_TEST_EXP_ERR", "not-a-number")
	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── required: integration ─────────────────────────────────────────────────────

func Test_env_satisfies_required(t *testing.T) {
	type cfg struct {
		Token string `env:"CONFL_TEST_REQ_TOKEN" flag:"token" required:"true"`
	}

	t.Setenv("CONFL_TEST_REQ_TOKEN", "secret")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Token, "secret")
}

func Test_env_required_neither_set(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Token string `env:"CONFL_TEST_REQ_MISS" flag:"token" required:"true"`
	}

	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── type coverage ─────────────────────────────────────────────────────────────

func Test_env_int(t *testing.T) {
	type cfg struct {
		Count int `env:"CONFL_TEST_INT_COUNT" flag:"count"`
	}

	t.Setenv("CONFL_TEST_INT_COUNT", "42")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.Count, 42)
}

func Test_env_int_invalid(t *testing.T) {
	type cfg struct {
		Count int `env:"CONFL_TEST_INT_BAD" flag:"count"`
	}

	t.Setenv("CONFL_TEST_INT_BAD", "not-a-number")
	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func Test_env_bool(t *testing.T) {
	type cfg struct {
		Verbose bool `env:"CONFL_TEST_BOOL_V" flag:"verbose"`
	}

	tests := []struct {
		name string
		val  string
		want bool
	}{
		{name: "true", val: "true", want: true},
		{name: "1", val: "1", want: true},
		{name: "false", val: "false", want: false},
		{name: "0", val: "0", want: false},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFL_TEST_BOOL_V", tt.val)
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, nil))
			assert.Equal(t, c.Verbose, tt.want)
		})
	}
}

func Test_env_string_slice(t *testing.T) {
	type cfg struct {
		Tags []string `env:"CONFL_TEST_TAGS" flag:"tags" sep:","`
	}

	t.Setenv("CONFL_TEST_TAGS", "a,b,c")
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, len(c.Tags), 3)
	assert.Equal(t, c.Tags[0], "a")
	assert.Equal(t, c.Tags[1], "b")
	assert.Equal(t, c.Tags[2], "c")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptrStr(s string) *string { return &s }
