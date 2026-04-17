package confl

import (
	"testing"

	"github.com/tychoish/fun/assert"
)

// ── joinStr ───────────────────────────────────────────────────────────────────

func Test_joinStr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty args returns empty string", args: nil, want: ""},
		{name: "single element", args: []string{"hello"}, want: "hello"},
		{name: "two elements concatenated", args: []string{"foo", "bar"}, want: "foobar"},
		{name: "three elements", args: []string{"a", "b", "c"}, want: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, joinStr(tt.args...), tt.want)
		})
	}
}

// ── callWhen ─────────────────────────────────────────────────────────────────

func Test_callWhen(t *testing.T) {
	t.Parallel()

	t.Run("true condition calls op", func(t *testing.T) {
		called := false
		callWhen(true, func(_ int) { called = true }, 0)
		assert.True(t, called)
	})

	t.Run("false condition skips op", func(t *testing.T) {
		called := false
		callWhen(false, func(_ int) { called = true }, 0)
		assert.True(t, !called)
	})
}

// ── checkNargTags ─────────────────────────────────────────────────────────────

func Test_checkNargTags(t *testing.T) {
	t.Parallel()

	// We test checkNargTags indirectly via conflagure, but it's also exercised
	// directly here for thorough coverage.

	t.Run("empty narg is no-op", func(t *testing.T) {
		type cfg struct {
			Name string `flag:"name"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("unknown narg value returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"bad"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("narg rest on non-slice returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			File string `narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("narg rest with flag: tag returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Files []string `flag:"files" narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("narg until without flag: tag returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"until"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── checkSepTag ───────────────────────────────────────────────────────────────

func Test_checkSepTag(t *testing.T) {
	t.Parallel()

	t.Run("sep on string scalar returns error", func(t *testing.T) {
		type cfg struct {
			Name string `flag:"name" sep:","`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("sep on slice is valid", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── checkExportedFlag ─────────────────────────────────────────────────────────

func Test_checkExportedFlag(t *testing.T) {
	t.Parallel()

	t.Run("unexported field with flag: tag returns error", func(t *testing.T) {
		type cfg struct {
			//nolint:unused
			secret string `flag:"secret"` //nolint:structcheck
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("exported field with flag: tag is valid", func(t *testing.T) {
		type cfg struct {
			Name string `flag:"name"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── flagSpec.validate ─────────────────────────────────────────────────────────

func Test_flagSpec_validate(t *testing.T) {
	t.Parallel()

	t.Run("valid spec with single-char short", func(t *testing.T) {
		fs := newTestFS()
		spec := flagSpec{Name: "verbose", Short: "v", Help: "verbose"}
		assert.NotError(t, spec.validate(fs))
	})

	t.Run("empty short is valid", func(t *testing.T) {
		fs := newTestFS()
		spec := flagSpec{Name: "verbose", Short: "", Help: "verbose"}
		assert.NotError(t, spec.validate(fs))
	})

	t.Run("multi-char short returns error", func(t *testing.T) {
		fs := newTestFS()
		spec := flagSpec{Name: "verbose", Short: "vv", Help: "verbose"}
		err := spec.validate(fs)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("duplicate flag name returns error", func(t *testing.T) {
		fs := newTestFS()
		var s string
		fs.StringVar(&s, "dup", "", "first")
		spec := flagSpec{Name: "dup", Help: "second"}
		err := spec.validate(fs)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("duplicate short flag returns error", func(t *testing.T) {
		fs := newTestFS()
		var s string
		fs.StringVar(&s, "x", "", "first")
		spec := flagSpec{Name: "new-flag", Short: "x", Help: "help"}
		err := spec.validate(fs)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── registerAlias ─────────────────────────────────────────────────────────────

func Test_registerAlias(t *testing.T) {
	t.Parallel()

	t.Run("registers long name with help", func(t *testing.T) {
		spec := flagSpec{Name: "verbose", Help: "be verbose"}
		registered := make(map[string]string)
		registerAlias(spec, func(name, usage string) {
			registered[name] = usage
		})
		assert.Equal(t, len(registered), 1)
		assert.Equal(t, registered["verbose"], "be verbose")
	})

	t.Run("registers short alias with generated help", func(t *testing.T) {
		spec := flagSpec{Name: "verbose", Short: "v", Help: "be verbose"}
		registered := make(map[string]string)
		registerAlias(spec, func(name, usage string) {
			registered[name] = usage
		})
		assert.Equal(t, len(registered), 2)
		assert.Equal(t, registered["verbose"], "be verbose")
		assert.Equal(t, registered["v"], "short for -verbose")
	})

	t.Run("no short means only long registered", func(t *testing.T) {
		spec := flagSpec{Name: "output", Short: "", Help: "output file"}
		count := 0
		registerAlias(spec, func(name, usage string) { count++ })
		assert.Equal(t, count, 1)
	})
}
