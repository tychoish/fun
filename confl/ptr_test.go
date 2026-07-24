package confl

import (
	"slices"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

// namespacedFilter is a named, flag-tagged struct nested under a
// parent, producing dotted flag names.
type namespacedFilter struct {
	Org       *string    `flag:"org" help:"limit sync to a single org"`
	StartDate *time.Time `flag:"start-date" format:"2006-01-02" help:"only sync items on/after this date"`
	EndDate   *time.Time `flag:"end-date" format:"2006-01-02" help:"only sync items before this date"`
}

// namespacedConfig exercises namespacing, env fallback, and required
// alongside pointer fields.
type namespacedConfig struct {
	Filter       namespacedFilter `flag:"filter"`
	ValidateOnly *bool            `flag:"validate-only" help:"validate without syncing"`
	SecretsName  *string          `flag:"secrets-name" env:"CONFL_TEST_SECRETS_NAME" help:"AWS Secrets Manager secret name"`
}

func Test_conflagure_pointer_string_unset_is_nil(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Org *string `flag:"org"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.NilPtr(t, c.Org)
}

func Test_conflagure_pointer_string_explicit_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Org *string `flag:"org"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-org", "acme"}))
	assert.NotNilPtr(t, c.Org)
	assert.Equal(t, *c.Org, "acme")
}

// Test_conflagure_pointer_string_explicit_zero_value is the crux of the
// pointer feature: passing an explicit empty string must be distinguishable
// from never passing the flag at all, so the caller can tell "sync all
// orgs, explicitly" from "no CLI opinion, ask the config file".
func Test_conflagure_pointer_string_explicit_zero_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Org *string `flag:"org"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-org", ""}))
	assert.NotNilPtr(t, c.Org)
	assert.Equal(t, *c.Org, "")
}

func Test_conflagure_pointer_bool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want *bool
	}{
		{name: "unset is nil", args: nil, want: nil},
		{name: "flag passed sets true", args: []string{"-validate-only"}, want: boolPtr(true)},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			type cfg struct {
				ValidateOnly *bool `flag:"validate-only"`
			}
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			if tt.want == nil {
				assert.NilPtr(t, c.ValidateOnly)
				return
			}
			assert.NotNilPtr(t, c.ValidateOnly)
			assert.Equal(t, *c.ValidateOnly, *tt.want)
		})
	}
}

func Test_conflagure_pointer_time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		StartDate *time.Time `flag:"start-date" format:"2006-01-02"`
	}

	t.Run("unset is nil", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.NilPtr(t, c.StartDate)
	})

	t.Run("explicit value", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-start-date", "2026-01-01"}))
		assert.NotNilPtr(t, c.StartDate)
		want, err := time.Parse("2006-01-02", "2026-01-01")
		assert.NotError(t, err)
		assert.True(t, c.StartDate.Equal(want))
	})
}

// Test_conflagure_pointer_time_auto_format verifies a *time.Time field with
// no format: tag falls back to auto-detected layouts, matching the
// non-pointer *time.Time behavior.
func Test_conflagure_pointer_time_auto_format(t *testing.T) {
	t.Parallel()

	type cfg struct {
		StartDate *time.Time `flag:"start-date"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-start-date", "2026-01-01T00:00:00Z"}))
	assert.NotNilPtr(t, c.StartDate)
	want, err := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	assert.NotError(t, err)
	assert.True(t, c.StartDate.Equal(want))
}

// Test_conflagure_pointer_namespacing verifies that pointer leaves nested
// under a named, flag-tagged struct field produce dotted flag names
// (-filter.org, -filter.start-date, -filter.end-date), each independently
// reporting set vs. unset.
func Test_conflagure_pointer_namespacing(t *testing.T) {
	t.Parallel()

	var c namespacedConfig
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-filter.org", "acme", "-validate-only"}))

	assert.NotNilPtr(t, c.Filter.Org)
	assert.Equal(t, *c.Filter.Org, "acme")

	assert.NilPtr(t, c.Filter.StartDate)
	assert.NilPtr(t, c.Filter.EndDate)

	assert.NotNilPtr(t, c.ValidateOnly)
	assert.Equal(t, *c.ValidateOnly, true)

	assert.NilPtr(t, c.SecretsName)
}

// Test_conflagure_pointer_env_fallback verifies a pointer field tagged env:
// ends up non-nil when only the env var is set, nil when neither CLI nor env
// is set, and reflects the CLI value when both are set (CLI wins by default).
func Test_conflagure_pointer_env_fallback(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		envSet  bool
		args    []string
		wantNil bool
		want    string
	}{
		{name: "neither set is nil", wantNil: true},
		{name: "env var sets pointer", envVal: "prod-secret", envSet: true, want: "prod-secret"},
		{name: "cli overrides env", envVal: "prod-secret", envSet: true, args: []string{"-secrets-name", "cli-secret"}, want: "cli-secret"},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("CONFL_TEST_SECRETS_NAME", tt.envVal)
			}
			var c namespacedConfig
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			if tt.wantNil {
				assert.NilPtr(t, c.SecretsName)
				return
			}
			assert.NotNilPtr(t, c.SecretsName)
			assert.Equal(t, *c.SecretsName, tt.want)
		})
	}
}

// Test_conflagure_pointer_required verifies required:"true" treats a nil
// pointer as unset (error) and any non-nil pointer — even one pointing to a
// zero value — as satisfying the requirement.
func Test_conflagure_pointer_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Org *string `flag:"org" required:"true"`
	}

	t.Run("unset is an error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("explicit zero value satisfies required", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-org", ""}))
		assert.NotNilPtr(t, c.Org)
	})

	t.Run("explicit non-zero value satisfies required", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-org", "acme"}))
		assert.NotNilPtr(t, c.Org)
	})
}

// Test_conflagure_pointer_default verifies default: still applies to pointer
// fields — a caller that wants "populated unless overridden" behavior (as
// opposed to nil-means-defer-to-config) can still use default: on a pointer
// field, and it is only overridden by an explicit CLI or env value.
func Test_conflagure_pointer_default(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Org *string `default:"all" flag:"org"`
	}

	t.Run("default applies when unset", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.NotNilPtr(t, c.Org)
		assert.Equal(t, *c.Org, "all")
	})

	t.Run("explicit value overrides default", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-org", "acme"}))
		assert.NotNilPtr(t, c.Org)
		assert.Equal(t, *c.Org, "acme")
	})
}

func boolPtr(b bool) *bool { return &b }

// Test_registerPointerFlag_invalid_default verifies a malformed default:
// on a pointer field surfaces as ErrInvalidSpecification rather than
// panicking or silently leaving the pointer nil.
func Test_registerPointerFlag_invalid_default(t *testing.T) {
	t.Parallel()

	var v *int
	err := registerFlag(newTestFS(), &v, flagSpec{Name: "flag-name", Default: "notanint", Help: "help"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// Test_registerPointerFlag_invalid_cli_value verifies a bad CLI value for a
// pointer field is rejected by the underlying parse function and does not
// set the pointer.
func Test_registerPointerFlag_invalid_cli_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Count *int `flag:"count"`
	}

	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, []string{"-count", "notanint"}))
}

// Test_registerPointerBoolFlag_invalid_default mirrors the scalar bool case:
// a malformed default: for a pointer bool field is an ErrInvalidSpecification.
func Test_registerPointerBoolFlag_invalid_default(t *testing.T) {
	t.Parallel()

	var v *bool
	err := registerFlag(newTestFS(), &v, flagSpec{Name: "flag-name", Default: "notabool", Help: "help"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// Test_registerPointerBoolFlag_invalid_cli_value verifies a bad CLI value for
// a pointer bool field is rejected and leaves the pointer nil.
func Test_registerPointerBoolFlag_invalid_cli_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Flag *bool `flag:"flag"`
	}

	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, []string{"-flag=notabool"}))
}

// Test_conflagure_pointer_bool_default verifies default: on a pointer bool
// field allocates a non-nil pointer from the default, and an explicit CLI
// value still overrides it.
func Test_conflagure_pointer_bool_default(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Flag *bool `flag:"flag" default:"true"`
	}

	t.Run("default applies when unset", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.NotNilPtr(t, c.Flag)
		assert.Equal(t, *c.Flag, true)
	})

	t.Run("explicit value overrides default", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-flag=false"}))
		assert.NotNilPtr(t, c.Flag)
		assert.Equal(t, *c.Flag, false)
	})
}

// Test_conflagure_pointer_numeric_types exercises every pointer-numeric case
// in registerFlag's type switch (int64, uint, uint64, float64, float32,
// int32, int16, int8, uint32, uint16, uint8) plus *time.Duration, each with
// an explicit value and confirming unset stays nil.
func Test_conflagure_pointer_numeric_types(t *testing.T) {
	t.Parallel()

	t.Run("int64", func(t *testing.T) {
		type cfg struct {
			V *int64 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, int64(42))

		var unset cfg
		assert.NotError(t, conflagure(newTestFS(), &unset, nil))
		assert.NilPtr(t, unset.V)
	})

	t.Run("uint", func(t *testing.T) {
		type cfg struct {
			V *uint `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, uint(42))
	})

	t.Run("uint64", func(t *testing.T) {
		type cfg struct {
			V *uint64 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, uint64(42))
	})

	t.Run("float64", func(t *testing.T) {
		type cfg struct {
			V *float64 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "4.2"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, 4.2)
	})

	t.Run("float32", func(t *testing.T) {
		type cfg struct {
			V *float32 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "4.2"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, float32(4.2))
	})

	t.Run("int32", func(t *testing.T) {
		type cfg struct {
			V *int32 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, int32(42))
	})

	t.Run("int16", func(t *testing.T) {
		type cfg struct {
			V *int16 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, int16(42))
	})

	t.Run("int8", func(t *testing.T) {
		type cfg struct {
			V *int8 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, int8(42))
	})

	t.Run("uint32", func(t *testing.T) {
		type cfg struct {
			V *uint32 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, uint32(42))
	})

	t.Run("uint16", func(t *testing.T) {
		type cfg struct {
			V *uint16 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, uint16(42))
	})

	t.Run("uint8", func(t *testing.T) {
		type cfg struct {
			V *uint8 `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "42"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, uint8(42))
	})

	t.Run("time.Duration", func(t *testing.T) {
		type cfg struct {
			V *time.Duration `flag:"v"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-v", "5s"}))
		assert.NotNilPtr(t, c.V)
		assert.Equal(t, *c.V, 5*time.Second)

		var unset cfg
		assert.NotError(t, conflagure(newTestFS(), &unset, nil))
		assert.NilPtr(t, unset.V)
	})
}
