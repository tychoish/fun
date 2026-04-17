package confl

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

// ── registerFlag: short validation ───────────────────────────────────────────

func Test_registerFlag_short_validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		short   string
		wantErr bool
	}{
		{name: "single char short is valid", short: "v", wantErr: false},
		{name: "empty short is valid (no alias)", short: "", wantErr: false},
		{name: "two char short returns error", short: "vv", wantErr: true},
		{name: "word short returns error", short: "verbose", wantErr: true},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var s string
			err := registerFlag(newTestFS(), &s, flagSpec{Name: "name", Short: tt.short, Help: "help"})
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidSpecification)
			} else {
				assert.NotError(t, err)
			}
		})
	}
}

// ── registerFlag: invalid defaults ───────────────────────────────────────────

func Test_registerFlag_invalid_defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		defStr string
		ptr    func() any
	}{
		{
			name:   "int with non-numeric default",
			defStr: "notanint",
			ptr:    func() any { var v int; return &v },
		},
		{
			name:   "int64 with non-numeric default",
			defStr: "notanint",
			ptr:    func() any { var v int64; return &v },
		},
		{
			name:   "uint with non-numeric default",
			defStr: "notauint",
			ptr:    func() any { var v uint; return &v },
		},
		{
			name:   "uint64 with non-numeric default",
			defStr: "notauint",
			ptr:    func() any { var v uint64; return &v },
		},
		{
			name:   "float64 with non-numeric default",
			defStr: "notafloat",
			ptr:    func() any { var v float64; return &v },
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			err := registerFlag(newTestFS(), tt.ptr(), flagSpec{Name: "flag-name", Default: tt.defStr, Help: "help"})
			assert.Error(t, err)
		})
	}
}

// ── registerFlag: short alias for numeric types ───────────────────────────────

func Test_registerFlag_short_for_numeric_types(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ptr  func() any
	}{
		{name: "int with short alias", ptr: func() any { var v int; return &v }},
		{name: "int64 with short alias", ptr: func() any { var v int64; return &v }},
		{name: "uint with short alias", ptr: func() any { var v uint; return &v }},
		{name: "uint64 with short alias", ptr: func() any { var v uint64; return &v }},
		{name: "float64 with short alias", ptr: func() any { var v float64; return &v }},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			fs := newTestFS()
			assert.NotError(t, registerFlag(fs, tt.ptr(), flagSpec{Name: "long-flag", Short: "x", Default: "0", Help: "help"}))
			assert.True(t, fs.Lookup("x") != nil)
		})
	}
}

// ── registerFlag: flag.Value interface ───────────────────────────────────────

func Test_registerFlag_flag_value(t *testing.T) {
	t.Parallel()

	t.Run("basic registration and set via flag", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		assert.NotError(t, registerFlag(fs, v, flagSpec{Name: "custom", Help: "help"}))
		assert.NotError(t, fs.Parse([]string{"-custom=hello"}))
		assert.Equal(t, v.String(), "hello")
	})

	t.Run("default applied via Set", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		assert.NotError(t, registerFlag(fs, v, flagSpec{Name: "custom", Default: "defaultval", Help: "help"}))
		assert.Equal(t, len(v.vals), 1)
		assert.Equal(t, v.vals[0], "defaultval")
	})

	t.Run("short alias registered", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		assert.NotError(t, registerFlag(fs, v, flagSpec{Name: "custom", Short: "c", Help: "help"}))
		assert.True(t, fs.Lookup("c") != nil)
		assert.NotError(t, fs.Parse([]string{"-c=via-short"}))
		assert.Equal(t, v.String(), "via-short")
	})

	t.Run("bad default returns ErrInvalidSpecification", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{setErr: errors.New("bad value")}
		err := registerFlag(fs, v, flagSpec{Name: "custom", Default: "baddefault", Help: "help"})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── registerFlag: flag.Value via conflagure ───────────────────────────────────

func Test_conflagure_flag_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Custom testFlagValue `default:"defval" flag:"custom" help:"custom flag"`
	}

	t.Run("default applied when no args", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, c.Custom.String(), "defval")
	})

	t.Run("explicit value overrides default", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-custom=explicit"}))
		assert.Equal(t, c.Custom.String(), "explicit")
	})

	t.Run("required field not set returns error", func(t *testing.T) {
		type requiredCfg struct {
			Custom testFlagValue `flag:"custom" required:"true"`
		}
		var c requiredCfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
}

// ── registerFlag: duplicate name errors ──────────────────────────────────────

func Test_registerFlag_errors(t *testing.T) {
	t.Parallel()

	t.Run("duplicate long flag name", func(t *testing.T) {
		type cfg struct {
			A string `flag:"dup"`
			B string `flag:"dup"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("duplicate short flag", func(t *testing.T) {
		type cfg struct {
			Alpha string `flag:"alpha" short:"x"`
			Beta  string `flag:"beta"  short:"x"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("bool impossible default", func(t *testing.T) {
		type cfg struct {
			Flag bool `default:"maybe" flag:"flag"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── registerSliceFlag: string slice ──────────────────────────────────────────

func Test_conflagure_slice_string(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags []string `flag:"tag" help:"tags" short:"t"`
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "single occurrence", args: []string{"-tag", "foo"}, want: []string{"foo"}},
		{name: "multiple long", args: []string{"-tag", "a", "-tag", "b", "-tag", "c"}, want: []string{"a", "b", "c"}},
		{name: "short alias", args: []string{"-t", "x"}, want: []string{"x"}},
		{name: "mixed long and short", args: []string{"-tag", "a", "-t", "b"}, want: []string{"a", "b"}},
		{name: "no flags gives nil slice", args: nil, want: nil},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			if tt.want == nil {
				assert.True(t, c.Tags == nil)
			} else {
				assert.EqualItems(t, c.Tags, tt.want)
			}
		})
	}
}

// ── registerSliceFlag: numeric slice types ────────────────────────────────────

func Test_conflagure_slice_numeric(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Ints    []int     `flag:"i"`
		Int64s  []int64   `flag:"i64"`
		Uints   []uint    `flag:"u"`
		Uint64s []uint64  `flag:"u64"`
		Floats  []float64 `flag:"f"`
		Bools   []bool    `flag:"b"`
	}

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, cfg)
	}{
		{
			name: "[]int accumulates",
			args: []string{"-i", "1", "-i", "2", "-i", "3"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Ints, []int{1, 2, 3})
			},
		},
		{
			name: "[]int64 accumulates",
			args: []string{"-i64", "10", "-i64", "20"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Int64s, []int64{10, 20})
			},
		},
		{
			name: "[]uint accumulates",
			args: []string{"-u", "5", "-u", "10"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Uints, []uint{5, 10})
			},
		},
		{
			name: "[]uint64 accumulates",
			args: []string{"-u64", "100"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Uint64s, []uint64{100})
			},
		},
		{
			name: "[]float64 accumulates",
			args: []string{"-f", "1.1", "-f", "2.2"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Floats, []float64{1.1, 2.2})
			},
		},
		{
			name: "[]bool accumulates",
			args: []string{"-b", "true", "-b", "false", "-b", "true"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.Bools, []bool{true, false, true})
			},
		},
		{
			name: "no flags gives nil slices",
			args: nil,
			check: func(t *testing.T, c cfg) {
				assert.True(t, c.Ints == nil)
				assert.True(t, c.Int64s == nil)
				assert.True(t, c.Uints == nil)
				assert.True(t, c.Uint64s == nil)
				assert.True(t, c.Floats == nil)
				assert.True(t, c.Bools == nil)
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			tt.check(t, c)
		})
	}
}

// ── registerSliceFlag: defaults ───────────────────────────────────────────────

func Test_conflagure_slice_defaults(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags []string `default:"debug,info" flag:"tag" sep:","`
	}

	t.Run("default applied when no flags given", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Tags, []string{"debug", "info"})
	})

	t.Run("default discarded when flag is used", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "warn"}))
		assert.EqualItems(t, c.Tags, []string{"warn"})
	})

	t.Run("default discarded when short alias is used first", func(t *testing.T) {
		type cfg2 struct {
			Tags []string `default:"debug,info" flag:"tag" sep:"," short:"t"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-t", "warn", "-tag", "error"}))
		assert.EqualItems(t, c.Tags, []string{"warn", "error"})
	})
}

// ── registerSliceFlag: required ───────────────────────────────────────────────

func Test_conflagure_slice_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags []string `flag:"tag" required:"true"`
	}

	t.Run("missing required slice returns error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("required slice satisfied", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "foo"}))
		assert.EqualItems(t, c.Tags, []string{"foo"})
	})

	t.Run("required slice satisfied by default", func(t *testing.T) {
		type cfg2 struct {
			Tags []string `default:"fallback" flag:"tag" required:"true" sep:""`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── registerSliceFlag: inverted bool slice ────────────────────────────────────

func Test_conflagure_slice_inverted_bool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Flags []bool `default:"true" flag:"flag" sep:"" short:"f"`
	}

	tests := []struct {
		name    string
		args    []string
		want    []bool
		wantErr bool
	}{
		{
			name: "default only — slice holds [true] from pre-population",
			args: nil,
			want: []bool{true},
		},
		{
			name: "bare no-flag appends false, clears default",
			args: []string{"-no-flag"},
			want: []bool{false},
		},
		{
			name: "no-flag=true appends false",
			args: []string{"-no-flag=true"},
			want: []bool{false},
		},
		{
			name: "no-flag=false appends true",
			args: []string{"-no-flag=false"},
			want: []bool{true},
		},
		{
			name: "multiple occurrences accumulate",
			args: []string{"-no-flag=true", "-no-flag=false"},
			want: []bool{false, true},
		},
		{
			name: "bare short alias no-f appends false, clears default",
			args: []string{"-no-f"},
			want: []bool{false},
		},
		{
			name: "mixed long and short accumulate into same slice",
			args: []string{"-no-flag=true", "-no-f=false"},
			want: []bool{false, true},
		},
		{
			name:    "invalid value returns error",
			args:    []string{"-no-flag=notabool"},
			wantErr: true,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NotError(t, err)
			assert.EqualItems(t, c.Flags, tt.want)
		})
	}
}

// ── registerSliceFlag: namespace and embedded ─────────────────────────────────

func Test_conflagure_slice_namespace(t *testing.T) {
	t.Parallel()

	type filters struct {
		Tags []string `flag:"tag" help:"filter tags"`
	}
	type cfg struct {
		Filter filters `flag:"f"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f.tag", "foo", "-f.tag", "bar"}))
	assert.EqualItems(t, c.Filter.Tags, []string{"foo", "bar"})
}

func Test_conflagure_slice_embedded(t *testing.T) {
	t.Parallel()

	type base struct {
		Labels []string `flag:"label" help:"labels"`
	}
	type cfg struct {
		base
		Name string `flag:"name"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-label", "a", "-label", "b", "-name", "test"}))
	assert.EqualItems(t, c.Labels, []string{"a", "b"})
	assert.Equal(t, c.Name, "test")
}

// ── registerSliceFlag: invalid defaults ───────────────────────────────────────

func Test_conflagure_slice_invalid_default(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{name: "[]int bad default", cfg: &struct {
			V []int `default:"1,bad" flag:"v" sep:","`
		}{}},
		{name: "[]int64 bad default", cfg: &struct {
			V []int64 `default:"1,bad" flag:"v" sep:","`
		}{}},
		{name: "[]uint bad default", cfg: &struct {
			V []uint `default:"1,bad" flag:"v" sep:","`
		}{}},
		{name: "[]uint64 bad default", cfg: &struct {
			V []uint64 `default:"1,bad" flag:"v" sep:","`
		}{}},
		{name: "[]float64 bad default", cfg: &struct {
			V []float64 `default:"1.0,notafloat" flag:"v" sep:","`
		}{}},
		{name: "[]bool bad default", cfg: &struct {
			V []bool `default:"true,notabool" flag:"v" sep:","`
		}{}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, conflagure(newTestFS(), tt.cfg, nil))
		})
	}
}

// ── registerSliceFlag: numeric short alias ────────────────────────────────────

func Test_conflagure_slice_numeric_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Ints    []int     `default:"0"     flag:"int"    sep:"" short:"i"`
		Int64s  []int64   `default:"0"     flag:"int64"  sep:"" short:"I"`
		Uints   []uint    `default:"0"     flag:"uint"   sep:"" short:"u"`
		Uint64s []uint64  `default:"0"     flag:"uint64" sep:"" short:"U"`
		Floats  []float64 `default:"0"     flag:"float"  sep:"" short:"f"`
		Bools   []bool    `default:"false" flag:"bool"   sep:"" short:"b"`
	}

	var c cfg
	args := []string{"-i", "1", "-I", "2", "-u", "3", "-U", "4", "-f", "1.5", "-b", "true"}
	assert.NotError(t, conflagure(newTestFS(), &c, args))
	assert.EqualItems(t, c.Ints, []int{1})
	assert.EqualItems(t, c.Int64s, []int64{2})
	assert.EqualItems(t, c.Uints, []uint{3})
	assert.EqualItems(t, c.Uint64s, []uint64{4})
	assert.EqualItems(t, c.Floats, []float64{1.5})
	assert.EqualItems(t, c.Bools, []bool{true})
}

// ── registerFlag: small int types ────────────────────────────────────────────

func Test_conflagure_small_int_types(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I8  int8    `flag:"i8"`
		I16 int16   `flag:"i16"`
		I32 int32   `flag:"i32"`
		U8  uint8   `flag:"u8"`
		U16 uint16  `flag:"u16"`
		U32 uint32  `flag:"u32"`
		F32 float32 `flag:"f32"`
	}

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, cfg)
	}{
		{
			name: "int8",
			args: []string{"-i8", "127"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.I8, int8(127)) },
		},
		{
			name: "int16",
			args: []string{"-i16", "32767"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.I16, int16(32767)) },
		},
		{
			name: "int32",
			args: []string{"-i32", "2147483647"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.I32, int32(2147483647)) },
		},
		{
			name: "uint8",
			args: []string{"-u8", "255"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.U8, uint8(255)) },
		},
		{
			name: "uint16",
			args: []string{"-u16", "65535"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.U16, uint16(65535)) },
		},
		{
			name: "uint32",
			args: []string{"-u32", "4294967295"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.U32, uint32(4294967295)) },
		},
		{
			name: "float32",
			args: []string{"-f32", "1.5"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.F32, float32(1.5)) },
		},
		{
			name: "negative int8",
			args: []string{"-i8", "-128"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.I8, int8(-128)) },
		},
		{
			name: "negative int32",
			args: []string{"-i32", "-2147483648"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.I32, int32(-2147483648)) },
		},
		{
			name: "defaults when all absent",
			args: nil,
			check: func(t *testing.T, c cfg) {
				assert.Zero(t, c.I8)
				assert.Zero(t, c.I16)
				assert.Zero(t, c.I32)
				assert.Zero(t, c.U8)
				assert.Zero(t, c.U16)
				assert.Zero(t, c.U32)
				assert.Zero(t, c.F32)
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			tt.check(t, c)
		})
	}
}

func Test_conflagure_small_int_defaults(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I8  int8    `default:"10"  flag:"i8"`
		I16 int16   `default:"200" flag:"i16"`
		I32 int32   `default:"300" flag:"i32"`
		U8  uint8   `default:"20"  flag:"u8"`
		U16 uint16  `default:"400" flag:"u16"`
		U32 uint32  `default:"500" flag:"u32"`
		F32 float32 `default:"2.5" flag:"f32"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, nil))
	assert.Equal(t, c.I8, int8(10))
	assert.Equal(t, c.I16, int16(200))
	assert.Equal(t, c.I32, int32(300))
	assert.Equal(t, c.U8, uint8(20))
	assert.Equal(t, c.U16, uint16(400))
	assert.Equal(t, c.U32, uint32(500))
	assert.Equal(t, c.F32, float32(2.5))
}

func Test_conflagure_small_int_invalid_defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{name: "int8 overflow", cfg: &struct {
			V int8 `default:"200" flag:"v"`
		}{}},
		{name: "int16 overflow", cfg: &struct {
			V int16 `default:"40000" flag:"v"`
		}{}},
		{name: "int32 overflow", cfg: &struct {
			V int32 `default:"3000000000" flag:"v"`
		}{}},
		{name: "uint8 overflow", cfg: &struct {
			V uint8 `default:"300" flag:"v"`
		}{}},
		{name: "uint16 overflow", cfg: &struct {
			V uint16 `default:"70000" flag:"v"`
		}{}},
		{name: "uint32 overflow", cfg: &struct {
			V uint32 `default:"5000000000" flag:"v"`
		}{}},
		{name: "float32 non-numeric", cfg: &struct {
			V float32 `default:"notafloat" flag:"v"`
		}{}},
		{name: "int8 non-numeric", cfg: &struct {
			V int8 `default:"notanint" flag:"v"`
		}{}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, conflagure(newTestFS(), tt.cfg, nil))
		})
	}
}

func Test_conflagure_small_int_invalid_values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
		args []string
	}{
		{name: "int8 overflow at parse", cfg: &struct {
			V int8 `flag:"v"`
		}{}, args: []string{"-v", "200"}},
		{name: "uint8 overflow at parse", cfg: &struct {
			V uint8 `flag:"v"`
		}{}, args: []string{"-v", "300"}},
		{name: "float32 bad value", cfg: &struct {
			V float32 `flag:"v"`
		}{}, args: []string{"-v", "notafloat"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, conflagure(newTestFS(), tt.cfg, tt.args))
		})
	}
}

// ── registerSliceFlag: small int slice types ───────────────────────────────────

func Test_conflagure_slice_small_int_types(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I8s  []int8    `flag:"i8"`
		I16s []int16   `flag:"i16"`
		I32s []int32   `flag:"i32"`
		U8s  []uint8   `flag:"u8"`
		U16s []uint16  `flag:"u16"`
		U32s []uint32  `flag:"u32"`
		F32s []float32 `flag:"f32"`
	}

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, cfg)
	}{
		{
			name: "[]int8 accumulates",
			args: []string{"-i8", "1", "-i8", "2", "-i8", "-3"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.I8s, []int8{1, 2, -3})
			},
		},
		{
			name: "[]int16 accumulates",
			args: []string{"-i16", "100", "-i16", "-200"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.I16s, []int16{100, -200})
			},
		},
		{
			name: "[]int32 accumulates",
			args: []string{"-i32", "1000", "-i32", "2000"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.I32s, []int32{1000, 2000})
			},
		},
		{
			name: "[]uint8 accumulates",
			args: []string{"-u8", "10", "-u8", "20"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.U8s, []uint8{10, 20})
			},
		},
		{
			name: "[]uint16 accumulates",
			args: []string{"-u16", "1000", "-u16", "2000"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.U16s, []uint16{1000, 2000})
			},
		},
		{
			name: "[]uint32 accumulates",
			args: []string{"-u32", "100000", "-u32", "200000"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.U32s, []uint32{100000, 200000})
			},
		},
		{
			name: "[]float32 accumulates",
			args: []string{"-f32", "1.5", "-f32", "2.5"},
			check: func(t *testing.T, c cfg) {
				assert.EqualItems(t, c.F32s, []float32{1.5, 2.5})
			},
		},
		{
			name: "no flags gives nil slices",
			args: nil,
			check: func(t *testing.T, c cfg) {
				assert.True(t, c.I8s == nil && c.I16s == nil && c.I32s == nil)
				assert.True(t, c.U8s == nil && c.U16s == nil && c.U32s == nil)
				assert.True(t, c.F32s == nil)
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			tt.check(t, c)
		})
	}
}

func Test_conflagure_slice_small_int_defaults(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I8s  []int8    `default:"1,2"     flag:"i8"  sep:","`
		I16s []int16   `default:"10,20"   flag:"i16" sep:","`
		I32s []int32   `default:"100,200" flag:"i32" sep:","`
		U8s  []uint8   `default:"3,4"     flag:"u8"  sep:","`
		U16s []uint16  `default:"30,40"   flag:"u16" sep:","`
		U32s []uint32  `default:"300,400" flag:"u32" sep:","`
		F32s []float32 `default:"1.5,2.5" flag:"f32" sep:","`
	}

	t.Run("defaults applied when no flags given", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.I8s, []int8{1, 2})
		assert.EqualItems(t, c.F32s, []float32{1.5, 2.5})
	})

	t.Run("flag use discards defaults", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-i8", "99"}))
		assert.EqualItems(t, c.I8s, []int8{99})
	})
}

func Test_conflagure_slice_small_int_invalid_defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{name: "[]int8 overflow in default", cfg: &struct {
			V []int8 `default:"1,200" flag:"v" sep:","`
		}{}},
		{name: "[]int16 overflow in default", cfg: &struct {
			V []int16 `default:"1,40000" flag:"v" sep:","`
		}{}},
		{name: "[]int32 overflow in default", cfg: &struct {
			V []int32 `default:"1,3000000000" flag:"v" sep:","`
		}{}},
		{name: "[]uint8 overflow in default", cfg: &struct {
			V []uint8 `default:"1,300" flag:"v" sep:","`
		}{}},
		{name: "[]uint16 overflow in default", cfg: &struct {
			V []uint16 `default:"1,70000" flag:"v" sep:","`
		}{}},
		{name: "[]uint32 overflow in default", cfg: &struct {
			V []uint32 `default:"1,5000000000" flag:"v" sep:","`
		}{}},
		{name: "[]float32 non-numeric in default", cfg: &struct {
			V []float32 `default:"1.0,notafloat" flag:"v" sep:","`
		}{}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, conflagure(newTestFS(), tt.cfg, nil))
		})
	}
}

// ── registerSliceFlag: time types ─────────────────────────────────────────────

func Test_conflagure_slice_time_Duration(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Timeouts []time.Duration `flag:"timeout" help:"timeouts" short:"t"`
	}

	tests := []struct {
		name string
		args []string
		want []time.Duration
	}{
		{name: "single value", args: []string{"-timeout", "1s"}, want: []time.Duration{time.Second}},
		{name: "multiple values", args: []string{"-timeout", "1s", "-timeout", "2m"}, want: []time.Duration{time.Second, 2 * time.Minute}},
		{name: "short alias", args: []string{"-t", "500ms"}, want: []time.Duration{500 * time.Millisecond}},
		{name: "mixed long and short", args: []string{"-timeout", "1s", "-t", "2s"}, want: []time.Duration{time.Second, 2 * time.Second}},
		{name: "no flags gives nil", args: nil, want: nil},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			if tt.want == nil {
				assert.True(t, c.Timeouts == nil)
			} else {
				assert.EqualItems(t, c.Timeouts, tt.want)
			}
		})
	}
}

func Test_conflagure_slice_time_Duration_defaults(t *testing.T) {
	t.Parallel()

	t.Run("comma-separated default applied", func(t *testing.T) {
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Timeouts, []time.Duration{time.Second, 2 * time.Minute})
	})

	t.Run("default discarded when flag used", func(t *testing.T) {
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout" sep:","`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-timeout", "5s"}))
		assert.EqualItems(t, c.Timeouts, []time.Duration{5 * time.Second})
	})

	t.Run("invalid default returns error", func(t *testing.T) {
		type cfg struct {
			Timeouts []time.Duration `default:"1s,notaduration" flag:"timeout" sep:","`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		type cfg struct {
			Timeouts []time.Duration `flag:"timeout"`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"-timeout", "notaduration"}))
	})
}

func Test_conflagure_slice_time_Time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Dates []time.Time `flag:"date" help:"dates" short:"d"`
	}

	t.Run("single RFC3339 value", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-date", "2024-06-01T12:00:00Z"}))
		want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		assert.Equal(t, len(c.Dates), 1)
		assert.True(t, c.Dates[0].Equal(want))
	})

	t.Run("multiple values accumulate", func(t *testing.T) {
		var c cfg
		args := []string{"-date", "2024-01-01T00:00:00Z", "-date", "2024-06-01T00:00:00Z"}
		assert.NotError(t, conflagure(newTestFS(), &c, args))
		assert.Equal(t, len(c.Dates), 2)
	})

	t.Run("short alias", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-d", "2024-03-15T00:00:00Z"}))
		assert.Equal(t, len(c.Dates), 1)
	})

	t.Run("no flags gives nil", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.True(t, c.Dates == nil)
	})

	t.Run("now in default resolves to non-zero time", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `default:"now" flag:"date" sep:""`
		}
		before := time.Now().UTC().Add(-time.Second)
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, len(c.Dates), 1)
		assert.True(t, c.Dates[0].After(before))
	})

	t.Run("custom format tag", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `flag:"date" format:"2006-01-02"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-date", "2024-03-15"}))
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, len(c.Dates), 1)
		assert.True(t, c.Dates[0].Equal(want))
	})

	t.Run("default discarded when flag used", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `default:"2023-01-01T00:00:00Z" flag:"date" sep:""`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-date", "2024-06-01T00:00:00Z"}))
		want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, len(c.Dates), 1)
		assert.True(t, c.Dates[0].Equal(want))
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"-date", "not-a-time"}))
	})

	t.Run("invalid default returns error", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `default:"not-a-time" flag:"date" sep:""`
		}
		var c cfg2
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── registerFlag: numeric via struct with short alias ─────────────────────────

func Test_conflagure_numeric_with_short_via_struct(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Count int     `default:"1"   flag:"count" short:"c"`
		Big   int64   `default:"2"   flag:"big"   short:"b"`
		Size  uint    `default:"3"   flag:"size"  short:"s"`
		Large uint64  `default:"4"   flag:"large" short:"l"`
		Ratio float64 `default:"1.5" flag:"ratio" short:"r"`
	}

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, cfg)
	}{
		{
			name: "int short alias",
			args: []string{"-c", "99"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.Count, 99) },
		},
		{
			name: "int64 short alias",
			args: []string{"-b", "88"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.Big, int64(88)) },
		},
		{
			name: "uint short alias",
			args: []string{"-s", "77"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.Size, uint(77)) },
		},
		{
			name: "uint64 short alias",
			args: []string{"-l", "66"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.Large, uint64(66)) },
		},
		{
			name: "float64 short alias",
			args: []string{"-r", "2.71"},
			check: func(t *testing.T, c cfg) { assert.Equal(t, c.Ratio, 2.71) },
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			tt.check(t, c)
		})
	}
}

// ── registerFlag: short help text ────────────────────────────────────────────

func Test_conflagure_short_help_text(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		shortName string
		wantUsage string
	}{
		{
			name:      "string short alias has correct help",
			flagName:  "branch",
			shortName: "b",
			wantUsage: "short for -branch",
		},
		{
			name:      "int short alias has correct help",
			flagName:  "count",
			shortName: "c",
			wantUsage: "short for -count",
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			fs := newTestFS()
			var s string
			assert.NotError(t, registerFlag(fs, &s, flagSpec{Name: tt.flagName, Short: tt.shortName, Help: "long help text"}))
			f := fs.Lookup(tt.shortName)
			assert.True(t, f != nil)
			assert.Equal(t, f.Usage, tt.wantUsage)
		})
	}
}
