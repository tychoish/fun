package confl

import (
	"bytes"
	"context"
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
		Level  enumLevel  `default:"info"              enum:"debug,info,warn" flag:"level"`
		Format enumFormat `enum:"json,yaml,text"       flag:"format"`
		Code   enumLevel  `enum:"debug=-4,info,warn=4" flag:"code"`
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
		F enumFormat `case-sensitive:"true" enum:"json,yaml" flag:"f"`
	}
	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, []string{"-f=JSON"}))
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=json"}))
	assert.Equal(t, c.F, "json")

	// Names differing only by case are legal only with the opt-in.
	type both struct {
		F enumFormat `case-sensitive:"true" enum:"a,A" flag:"f"`
	}
	var b both
	assert.NotError(t, conflagure(newTestFS(), &b, []string{"-f=A"}))
	assert.Equal(t, b.F, "A")

	type folded struct {
		F enumFormat `enum:"a,A" flag:"f"`
	}
	err := conflagure(newTestFS(), &folded{}, nil)
	assert.ErrorIs(t, err, ErrInvalidSpecification)

	type orphan struct {
		F enumFormat `case-sensitive:"true" flag:"f"`
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
			F enumFormat `enum:"" flag:"f"`
		}{}},
		{"empty entry", &struct {
			F enumFormat `enum:"a,,b" flag:"f"`
		}{}},
		{"duplicate name", &struct {
			F enumFormat `enum:"a,b,a" flag:"f"`
		}{}},
		{"duplicate value", &struct {
			F enumLevel `enum:"a=1,b=1" flag:"f"`
		}{}},
		{"bad integer value", &struct {
			F enumLevel `enum:"a=x" flag:"f"`
		}{}},
		{"overflow", &struct {
			F int8 `enum:"a=200" flag:"f"`
		}{}},
		{"continuation overflow", &struct {
			F uint8 `enum:"a=255,b" flag:"f"`
		}{}},
		{"unsupported kind float", &struct {
			F float64 `enum:"a,b" flag:"f"`
		}{}},
		{"unsupported kind struct slice", &struct {
			F []bool `enum:"a,b" flag:"f"`
		}{}},
		{"flag.Value", &struct {
			F enumFlagValue `enum:"a,b" flag:"f"`
		}{}},
		{"default not in set", &struct {
			F enumFormat `default:"c" enum:"a,b" flag:"f"`
		}{}},
		{"option fails Validate", &struct {
			F enumStrict `enum:"ok,bad" flag:"f"`
		}{}},
		{"slice default without sep", &struct {
			F []enumFormat `default:"a" enum:"a,b" flag:"f"`
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
		L enumLevel    `default:"debug" enum:"debug,info" flag:"l"`
		S []enumFormat `default:"a,b"   enum:"a,b"        flag:"s" sep:","`
	}
	assert.NotError(t, Validate(&cfg{}))
}

func Test_enum_validateInterface(t *testing.T) {
	t.Parallel()

	t.Run("pointer receiver option rejected", func(t *testing.T) {
		type cfg struct {
			F enumPtrValidate `enum:"a,b,c" flag:"f"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("rejects runtime value", func(t *testing.T) {
		// Options pass at registration; Validate then starts rejecting, as a
		// type with state-dependent rules might.
		type cfg struct {
			F enumToggle `enum:"a,b" flag:"f"`
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
			F enumText `enum:"a,b" flag:"f"`
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
			F enumText `enum:"a" flag:"f"`
		}
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-f=beta"}))
	})
}

func Test_enum_unsetPrefilledUnchecked(t *testing.T) {
	t.Parallel()
	type cfg struct {
		L enumLevel `enum:"debug,info" flag:"l"`
	}
	c := cfg{L: 99}
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.L, 99)
}

func Test_enum_env(t *testing.T) {
	t.Setenv("CONFL_ENUM_LEVEL", "WARN")
	type cfg struct {
		L enumLevel `enum:"debug,info,warn" env:"CONFL_ENUM_LEVEL" flag:"l"`
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
			S []enumLevel `enum:"debug,info,warn" flag:"s" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=debug, warn", "-s", "info"}))
		assert.EqualItems(t, c.S, []enumLevel{enumDebug, enumWarn, enumInfo})
	})

	t.Run("slice element rejected", func(t *testing.T) {
		type cfg struct {
			S []enumLevel `enum:"debug,info" flag:"s" sep:","`
		}
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-s=debug,nope"}))
	})

	t.Run("slice default cleared on first use", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `default:"a,b" enum:"a,b,c" flag:"s" sep:"," short:"x"`
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
			P *enumLevel `enum:"debug,info,warn" flag:"p"`
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
			P *enumFormat `default:"b" enum:"a,b" flag:"p"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, *c.P, "b")
	})
}

func Test_enum_help(t *testing.T) {
	t.Parallel()
	type cfg struct {
		L enumLevel `default:"info"    enum:"debug,info,warn" flag:"level" help:"log level" short:"l"`
		N enumLevel `enum:"debug,info" flag:"nohelp"`
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
		L enumLevel `enum:"debug,info,warn" flag:"level"`
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
			F enumFormat `enum:"k=v,k=v" flag:"f"`
		}
		assert.ErrorIs(t, Validate(&dup{}), ErrInvalidSpecification)
		type ok struct {
			F enumFormat `enum:"a=b,a=c,=x" flag:"f"`
		}
		var c ok
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=a=c"}))
		assert.Equal(t, c.F, "a=c")
	})

	t.Run("64-bit continuation overflow", func(t *testing.T) {
		type u struct {
			F uint64 `enum:"x=18446744073709551615,y" flag:"f"`
		}
		type i struct {
			F int64 `enum:"x=9223372036854775807,y" flag:"f"`
		}
		type okU struct {
			F uint64 `enum:"x=18446744073709551615,y=1" flag:"f"`
		}
		assert.ErrorIs(t, Validate(&u{}), ErrInvalidSpecification)
		assert.ErrorIs(t, Validate(&i{}), ErrInvalidSpecification)
		assert.NotError(t, Validate(&okU{}))
	})

	t.Run("bad uint value", func(t *testing.T) {
		type cfg struct {
			F uint `enum:"x=-1" flag:"f"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("bad case-sensitive value", func(t *testing.T) {
		type cfg struct {
			F enumFormat `case-sensitive:"yes" enum:"a" flag:"f"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("failed Set leaves slice and default intact", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `default:"a" enum:"a,b,c" flag:"s" sep:","`
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

type enumRunCmd struct {
	Level enumLevel `enum:"debug,info,warn" flag:"level"`
}

func (c *enumRunCmd) Run(context.Context) error { return nil }

type enumUint uint8

func Test_enum_edgeCases(t *testing.T) {
	t.Run("bad case-sensitive value through bind", func(t *testing.T) {
		type cfg struct {
			F enumFormat `case-sensitive:"maybe" enum:"a" flag:"f"`
		}
		assert.ErrorIs(t, conflagure(newTestFS(), &cfg{}, nil), ErrInvalidSpecification)
	})

	t.Run("case-sensitive false folds", func(t *testing.T) {
		type cfg struct {
			F enumFormat `case-sensitive:"false" enum:"a" flag:"f"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f=A"}))
		assert.Equal(t, c.F, "a")
	})

	t.Run("case-sensitive without enum through bind", func(t *testing.T) {
		type cfg struct {
			F enumFormat `case-sensitive:"true" flag:"f"`
		}
		assert.ErrorIs(t, conflagure(newTestFS(), &cfg{}, nil), ErrInvalidSpecification)
	})

	t.Run("String after the pointer is reset", func(t *testing.T) {
		type cfg struct {
			P *enumLevel `enum:"debug,info" flag:"p"`
		}
		var c cfg
		fs := newTestFS()
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		assert.NotError(t, fs.Set("p", "info"))
		assert.Equal(t, fs.Lookup("p").Value.String(), "info")
		c.P = nil
		assert.Equal(t, fs.Lookup("p").Value.String(), "")
	})

	t.Run("String of unset and set slice", func(t *testing.T) {
		type cfg struct {
			S []enumLevel `enum:"debug,info,warn" flag:"s"`
		}
		var c cfg
		fs := newTestFS()
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		assert.Equal(t, fs.Lookup("s").Value.String(), "")
		assert.NotError(t, fs.Set("s", "warn"))
		assert.NotError(t, fs.Set("s", "debug"))
		assert.Equal(t, fs.Lookup("s").Value.String(), "warn,debug")
	})

	t.Run("String shows only the default before parsing", func(t *testing.T) {
		type cfg struct {
			L enumLevel `default:"warn"         enum:"debug,info,warn" flag:"l"`
			N enumLevel `enum:"debug,info,warn" flag:"n"`
		}
		var c cfg
		fs := newTestFS()
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		assert.Equal(t, fs.Lookup("l").Value.String(), "warn")
		assert.Equal(t, fs.Lookup("n").Value.String(), "")
		assert.Equal(t, fs.Lookup("l").DefValue, "warn")
		assert.Equal(t, fs.Lookup("n").DefValue, "")
	})

	t.Run("unsigned enum with iota and explicit values", func(t *testing.T) {
		type cfg struct {
			U enumUint `enum:"a,b=10,c" flag:"u"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-u=c"}))
		assert.Equal(t, c.U, 11)
	})

	t.Run("whitespace around entries and values", func(t *testing.T) {
		type cfg struct {
			L enumLevel `enum:" a = 5 , b " flag:"l"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-l=b"}))
		assert.Equal(t, c.L, 6)
	})

	t.Run("empty name with a value", func(t *testing.T) {
		type cfg struct {
			L enumLevel `enum:"=5" flag:"l"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("empty value after =", func(t *testing.T) {
		type cfg struct {
			L enumLevel `enum:"a=" flag:"l"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("namespaced field and short alias", func(t *testing.T) {
		type inner struct {
			L enumLevel `enum:"debug,info,warn" flag:"level" short:"l"`
		}
		type cfg struct {
			Log inner `flag:"log"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-log.level=warn"}))
		assert.Equal(t, c.Log.L, enumWarn)
		type short struct {
			L enumLevel `enum:"debug,info,warn" flag:"level" short:"l"`
		}
		var s short
		assert.NotError(t, conflagure(newTestFS(), &s, []string{"-l=info"}))
		assert.Equal(t, s.L, enumInfo)
	})

	t.Run("required enum", func(t *testing.T) {
		type cfg struct {
			L enumLevel `enum:"debug,info" flag:"l" required:"true"`
		}
		assert.ErrorIs(t, conflagure(newTestFS(), &cfg{}, nil), ErrInvalidInput)
		assert.NotError(t, conflagure(newTestFS(), &cfg{}, []string{"-l=info"}))
	})

	t.Run("env slice replaces defaults", func(t *testing.T) {
		t.Setenv("CONFL_ENUM_SLICE", "b,c")
		type cfg struct {
			S []enumFormat `default:"a" enum:"a,b,c" env:"CONFL_ENUM_SLICE" flag:"s" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.S, []enumFormat{"b", "c"})
	})

	t.Run("slice without sep appends once per flag", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `enum:"a,b,c" flag:"s"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=a", "-s=c"}))
		assert.EqualItems(t, c.S, []enumFormat{"a", "c"})
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-s=a,c"}))
	})

	t.Run("slice sep empty keeps whole value", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `enum:"a,b" flag:"s" sep:""`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=a", "-s=b"}))
		assert.EqualItems(t, c.S, []enumFormat{"a", "b"})
	})

	t.Run("empty element in a sep list", func(t *testing.T) {
		type cfg struct {
			S []enumFormat `enum:"a,b" flag:"s" sep:","`
		}
		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		assert.Error(t, conflagure(fs, &cfg{}, []string{"-s=a,"}))
	})

	t.Run("TextUnmarshaler default and slice", func(t *testing.T) {
		type cfg struct {
			F enumText   `default:"BETA" enum:"a,b" flag:"f"`
			S []enumText `enum:"a,b"     flag:"s"   sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-s=alpha,b"}))
		assert.Equal(t, c.F, 1)
		assert.Equal(t, len(c.S), 2)
	})

	t.Run("exact case with env", func(t *testing.T) {
		t.Setenv("CONFL_ENUM_EXACT", "JSON")
		type cfg struct {
			F enumFormat `case-sensitive:"true" enum:"json" env:"CONFL_ENUM_EXACT" flag:"f"`
		}
		assert.ErrorIs(t, conflagure(newTestFS(), &cfg{}, nil), ErrInvalidInput)
	})

	t.Run("duplicate flag across enum fields", func(t *testing.T) {
		type cfg struct {
			A enumLevel `enum:"a" flag:"x"`
			B enumLevel `enum:"a" flag:"x"`
		}
		assert.ErrorIs(t, Validate(&cfg{}), ErrInvalidSpecification)
	})

	t.Run("real subcommand dispatch", func(t *testing.T) {
		type cfg struct {
			Run enumRunCmd `cmd:"run"`
		}
		var c cfg
		cmd, err := conflagureCmd(newTestFS(), &c, []string{"run", "-level=warn"})
		assert.NotError(t, err)
		assert.True(t, cmd != nil)
		assert.Equal(t, c.Run.Level, enumWarn)

		fs := newTestFS()
		fs.SetOutput(&bytes.Buffer{})
		_, err = conflagureCmd(fs, &cfg{}, []string{"run", "-level=loud"})
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "must be one of debug, info, warn")
	})

	t.Run("error lists choices and wraps ErrInvalidInput", func(t *testing.T) {
		type cfg struct {
			L enumLevel `enum:"debug,info" flag:"l"`
		}
		fs := newTestFS()
		var b bytes.Buffer
		fs.SetOutput(&b)
		var c cfg
		assert.NotError(t, bindFlags(fs, reflectVal(&c), "", 0))
		err := fs.Lookup("l").Value.Set("zzz")
		assert.ErrorIs(t, err, ErrInvalidInput)
		assert.Substring(t, err.Error(), "must be one of debug, info")
	})
}
