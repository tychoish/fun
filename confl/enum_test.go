package confl

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun/assert"
)

type enumLevel int

const (
	enumDebug enumLevel = iota
	enumInfo
	enumWarn
)

type enumFormat string

// enumStrict rejects the value "bad" through Validate.
type enumStrict string

func (s enumStrict) Validate() error {
	if s == "bad" {
		return errors.New("bad is not allowed")
	}
	return nil
}

// enumPtrValidate validates through a pointer receiver.
type enumPtrValidate int

func (e *enumPtrValidate) Validate() error {
	if *e == 2 {
		return errors.New("two is not allowed")
	}
	return nil
}

var enumToggleReject atomic.Bool

// enumToggle's Validate depends on package state, so it can pass for every
// tag option and still reject a value at parse time.
type enumToggle string

func (e enumToggle) Validate() error {
	if enumToggleReject.Load() {
		return errors.New("toggled off")
	}
	return nil
}

// enumText parses with UnmarshalText, accepting an upper-case alias.
type enumText int

func (e *enumText) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "a", "alpha":
		*e = 0
	case "b", "beta":
		*e = 1
	default:
		return errors.New("unknown")
	}
	return nil
}

// enumFlagValue implements flag.Value; enum: must be rejected on it.
type enumFlagValue string

func (e *enumFlagValue) Set(s string) error { *e = enumFlagValue(s); return nil }
func (e *enumFlagValue) String() string     { return string(*e) }

func Test_enum_scalar(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Level  enumLevel  `flag:"level" enum:"debug,info,warn" default:"info"`
		Format enumFormat `flag:"format" enum:"json,yaml,text"`
		Code   enumLevel  `flag:"code" enum:"debug=-4,info,warn=4"`
	}

	tests := []struct {
		name    string
		args    []string
		want    cfg
		wantErr bool
	}{
		{name: "default applies", want: cfg{Level: enumInfo}},
		{name: "valid string", args: []string{"-format", "yaml"}, want: cfg{Level: enumInfo, Format: "yaml"}},
		{name: "invalid string", args: []string{"-format", "xml"}, wantErr: true},
		{name: "folds case, stores tag spelling", args: []string{"-format=JSON", "-level=WARN"}, want: cfg{Level: enumWarn, Format: "json"}},
		{name: "integer by name", args: []string{"-level=debug"}, want: cfg{Level: enumDebug}},
		{name: "integer by number rejected", args: []string{"-level=1"}, wantErr: true},
		{name: "name=value override and continuation", args: []string{"-code=info"}, want: cfg{Level: enumInfo, Code: -3}},
		{name: "negative value", args: []string{"-code=debug"}, want: cfg{Level: enumInfo, Code: -4}},
		{name: "explicit later value", args: []string{"-code=warn"}, want: cfg{Level: enumInfo, Code: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Substring(t, err.Error(), "must be one of")
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c, tt.want)
		})
	}
}

func Test_enum_exactCase(t *testing.T) {
	t.Parallel()

	type cfg struct {
		F enumFormat `flag:"f" enum:"json,yaml" case-sensitive:"true"`
	}
	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, []string{"-f=JSON"}))
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=json"}))
	assert.Equal(t, c.F, "json")

	// Names differing only by case are legal only with the opt-in.
	type both struct {
		F enumFormat `flag:"f" enum:"a,A" case-sensitive:"true"`
	}
	var b both
	assert.NotError(t, conflagure(newTestFS(), &b, []string{"-f=A"}))
	assert.Equal(t, b.F, "A")

	type folded struct {
		F enumFormat `flag:"f" enum:"a,A"`
	}
	err := conflagure(newTestFS(), &folded{}, nil)
	assert.ErrorIs(t, err, ErrInvalidSpecification)

	type orphan struct {
		F enumFormat `flag:"f" case-sensitive:"true"`
	}
	assert.ErrorIs(t, Validate(&orphan{}), ErrInvalidSpecification)
}

func Test_enum_specErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{"empty tag", &struct {
			F enumFormat `flag:"f" enum:""`
		}{}},
		{"empty entry", &struct {
			F enumFormat `flag:"f" enum:"a,,b"`
		}{}},
		{"duplicate name", &struct {
			F enumFormat `flag:"f" enum:"a,b,a"`
		}{}},
		{"duplicate value", &struct {
			F enumLevel `flag:"f" enum:"a=1,b=1"`
		}{}},
		{"bad integer value", &struct {
			F enumLevel `flag:"f" enum:"a=x"`
		}{}},
		{"overflow", &struct {
			F int8 `flag:"f" enum:"a=200"`
		}{}},
		{"continuation overflow", &struct {
			F uint8 `flag:"f" enum:"a=255,b"`
		}{}},
		{"unsupported kind float", &struct {
			F float64 `flag:"f" enum:"a,b"`
		}{}},
		{"unsupported kind struct slice", &struct {
			F []bool `flag:"f" enum:"a,b"`
		}{}},
		{"flag.Value", &struct {
			F enumFlagValue `flag:"f" enum:"a,b"`
		}{}},
		{"default not in set", &struct {
			F enumFormat `flag:"f" enum:"a,b" default:"c"`
		}{}},
		{"option fails Validate", &struct {
			F enumStrict `flag:"f" enum:"ok,bad"`
		}{}},
		{"slice default without sep", &struct {
			F []enumFormat `flag:"f" enum:"a,b" default:"a"`
		}{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, Validate(tt.cfg), ErrInvalidSpecification)
			assert.ErrorIs(t, conflagure(newTestFS(), tt.cfg, nil), ErrInvalidSpecification)
		})
	}
}

func Test_enum_validateOK(t *testing.T) {
	t.Parallel()
	type cfg struct {
		L enumLevel    `flag:"l" enum:"debug,info" default:"debug"`
		S []enumFormat `flag:"s" enum:"a,b" sep:"," default:"a,b"`
	}
	assert.NotError(t, Validate(&cfg{}))
}

func Test_enum_validateInterface(t *testing.T) {
	t.Parallel()

	t.Run("pointer receiver option rejected", func(t *testing.T) {
		type cfg struct {
			F enumPtrValidate `flag:"f" enum:"a,b,c"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("rejects runtime value", func(t *testing.T) {
		// Options pass at registration; Validate then starts rejecting, as a
		// type with state-dependent rules might.
		type cfg struct {
			F enumToggle `flag:"f" enum:"a,b"`
		}
		var c cfg
		assert.NotError(t, Validate(&c))
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		enumToggleReject.Store(true)
		defer enumToggleReject.Store(false)
		err := fs.Parse([]string{"-f=a"})
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "toggled off")
	})

	t.Run("text unmarshaler is used", func(t *testing.T) {
		type cfg struct {
			F enumText `flag:"f" enum:"a,b"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=BETA"}))
		assert.Equal(t, c.F, 1)
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-f=zzz"}))
	})

	t.Run("text unmarshaler still requires membership", func(t *testing.T) {
		type cfg struct {
			F enumText `flag:"f" enum:"a"`
		}
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-f=beta"}))
	})
}

func Test_enum_unsetPrefilledUnchecked(t *testing.T) {
	t.Parallel()
	type cfg struct {
		L enumLevel `flag:"l" enum:"debug,info"`
	}
	c := cfg{L: 99}
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.L, 99)
}

func Test_enum_env(t *testing.T) {
	t.Setenv("CONFL_ENUM_LEVEL", "WARN")
	type cfg struct {
		L enumLevel `flag:"l" enum:"debug,info,warn" env:"CONFL_ENUM_LEVEL"`
	}
	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.L, enumWarn)

	t.Setenv("CONFL_ENUM_LEVEL", "loud")
	err := conflagure(newTestFS(), &c, nil)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func Test_enum_sliceAndPointer(t *testing.T) {
	t.Parallel()

	t.Run("slice with sep", func(t *testing.T) {
		type cfg struct {
			S []enumLevel `flag:"s" enum:"debug,info,warn" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=debug, warn", "-s", "info"}))
		assert.EqualItems(t, c.S, []enumLevel{enumDebug, enumWarn, enumInfo})
	})

	t.Run("slice element rejected", func(t *testing.T) {
		type cfg struct {
			S []enumLevel `flag:"s" enum:"debug,info" sep:","`
		}
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-s=debug,nope"}))
	})

	t.Run("slice default cleared on first use", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `flag:"s" short:"x" enum:"a,b,c" sep:"," default:"a,b"`
		}
		var d cfg
		assert.NotError(t, conflagure(newTestFS(), &d, nil))
		assert.EqualItems(t, d.S, []enumFormat{"a", "b"})

		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=c", "-x=a"}))
		assert.EqualItems(t, c.S, []enumFormat{"c", "a"})
	})

	t.Run("pointer nil vs set", func(t *testing.T) {
		type cfg struct {
			P *enumLevel `flag:"p" enum:"debug,info,warn"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Nil(t, c.P)
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-p=debug"}))
		assert.NotNil(t, c.P)
		assert.Equal(t, *c.P, enumDebug)
	})

	t.Run("pointer default", func(t *testing.T) {
		type cfg struct {
			P *enumFormat `flag:"p" enum:"a,b" default:"b"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, *c.P, "b")
	})
}

func Test_enum_help(t *testing.T) {
	t.Parallel()
	type cfg struct {
		L enumLevel `flag:"level" short:"l" enum:"debug,info,warn" default:"info" help:"log level"`
		N enumLevel `flag:"nohelp" enum:"debug,info"`
	}
	fs := newTestFS()
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	var c cfg
	assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
	fs.PrintDefaults()
	out := buf.String()
	assert.Substring(t, out, "log level (one of: debug, info, warn) (default info)")
	assert.Substring(t, out, "one of: debug, info")
	// An iota enum whose zero value is a name shows no default unless tagged.
	assert.NotSubstring(t, out, "(one of: debug, info) (default")
	assert.True(t, fs.Lookup("level").Value.String() == "info")
}

func Test_enum_zeroValueString(t *testing.T) {
	t.Parallel()
	var e enumValue
	assert.Equal(t, e.String(), "")
	var nilE *enumValue
	assert.Equal(t, nilE.String(), "")
	var _ flag.Value = &e
}

func Test_enum_subcommand(t *testing.T) {
	t.Parallel()
	type sub struct {
		L enumLevel `flag:"level" enum:"debug,info,warn"`
	}
	type root struct {
		Sub sub `cmd:"run"`
	}
	var r root
	assert.NotError(t, Validate(&r))
	assert.NotError(t, conflagure(newTestFS(), &r.Sub, []string{"-level=warn"}))
	assert.Equal(t, r.Sub.L, enumWarn)
}

func Test_enum_reviewRegressions(t *testing.T) {
	t.Parallel()

	t.Run("string kind treats = literally", func(t *testing.T) {
		type dup struct {
			F enumFormat `flag:"f" enum:"k=v,k=v"`
		}
		assert.ErrorIs(t, Validate(&dup{}), ErrInvalidSpecification)
		type ok struct {
			F enumFormat `flag:"f" enum:"a=b,a=c,=x"`
		}
		var c ok
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=a=c"}))
		assert.Equal(t, c.F, "a=c")
	})

	t.Run("64-bit continuation overflow", func(t *testing.T) {
		type u struct {
			F uint64 `flag:"f" enum:"x=18446744073709551615,y"`
		}
		type i struct {
			F int64 `flag:"f" enum:"x=9223372036854775807,y"`
		}
		type okU struct {
			F uint64 `flag:"f" enum:"x=18446744073709551615,y=1"`
		}
		assert.ErrorIs(t, Validate(&u{}), ErrInvalidSpecification)
		assert.ErrorIs(t, Validate(&i{}), ErrInvalidSpecification)
		assert.NotError(t, Validate(&okU{}))
	})

	t.Run("bad uint value", func(t *testing.T) {
		type cfg struct {
			F uint `flag:"f" enum:"x=-1"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("bad case-sensitive value", func(t *testing.T) {
		type cfg struct {
			F enumFormat `flag:"f" enum:"a" case-sensitive:"yes"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("failed Set leaves slice and default intact", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `flag:"s" enum:"a,b,c" sep:"," default:"a"`
		}
		var c cfg
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		assert.Error(t, fs.Set("s", "b,nope"))
		assert.EqualItems(t, c.S, []enumFormat{"a"})
		assert.NotError(t, fs.Set("s", "b,c"))
		assert.EqualItems(t, c.S, []enumFormat{"b", "c"})
	})
}
