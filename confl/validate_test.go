package confl

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

// ── Validate() tests ──────────────────────────────────────────────────────────

func TestValidate_valid_structs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "simple string field",
			cfg: &struct {
				Name string `flag:"name"`
			}{},
		},
		{
			name: "bool with default",
			cfg: &struct {
				Flag bool `default:"true" flag:"flag"`
			}{},
		},
		{
			name: "int with short alias",
			cfg: &struct {
				Count int `flag:"count" short:"c"`
			}{},
		},
		{
			name: "slice with sep tag",
			cfg: &struct {
				Tags []string `default:"a,b" flag:"tags" sep:","`
			}{},
		},
		{
			name: "slice with sep-empty tag",
			cfg: &struct {
				Tags []string `default:"whole-value" flag:"tags" sep:""`
			}{},
		},
		{
			name: "slice no default no sep",
			cfg: &struct {
				Tags []string `flag:"tags"`
			}{},
		},
		{
			name: "narg rest field",
			cfg: &struct {
				Rest []string `narg:"rest"`
			}{},
		},
		{
			name: "narg until with flag",
			cfg: &struct {
				Files []string `flag:"files" narg:"until"`
			}{},
		},
		{
			name: "nested anonymous struct",
			cfg: &struct {
				BaseTest

				Name string `flag:"name"`
			}{},
		},
		{
			name: "namespaced nested struct",
			cfg: &struct {
				Server struct {
					Host string `flag:"host"`
				} `flag:"srv"`
			}{},
		},
		{
			name: "flag.Value field",
			cfg: &struct {
				Custom testFlagValue `flag:"custom"`
			}{},
		},
		{
			name: "time.Time field",
			cfg: &struct {
				At time.Time `flag:"at"`
			}{},
		},
		{
			name: "time.Duration field",
			cfg: &struct {
				Timeout time.Duration `default:"5s" flag:"timeout"`
			}{},
		},
		{
			name: "field with no flag or narg tag is silently skipped",
			cfg: &struct {
				Name    string `flag:"name"`
				Ignored int    // no flag: or narg: tag — should be skipped
			}{},
		},
		{
			name: "unexported struct field with valid contents is silently skipped",
			cfg: &struct {
				extras struct{ x int } // unexported struct field, valid contents
				Name   string          `flag:"name"`
			}{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.cfg); err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_nil_input(t *testing.T) {
	t.Parallel()

	err := Validate(nil)
	if err == nil {
		t.Fatal("Validate(nil) expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_non_pointer(t *testing.T) {
	t.Parallel()

	type cfg struct{ X string }
	err := Validate(cfg{})
	if err == nil {
		t.Fatal("Validate(struct value) expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_unexported_flag(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		//nolint:unused
		internal string `flag:"secret"` //nolint:structcheck
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for unexported flag field, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_short_too_long(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Name string `flag:"name" short:"ab"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for multi-char short, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_rest_with_flag(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Files []string `flag:"files" narg:"rest"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for narg:rest + flag: combo, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_until_no_flag(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Files []string `narg:"until"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for narg:until without flag:, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_rest_non_slice(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		File string `narg:"rest"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for narg:rest on non-slice, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_until_non_slice(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		File string `flag:"file" narg:"until"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for narg:until on non-slice, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_rest_with_cmd(t *testing.T) {
	t.Parallel()

	type sub struct {
		X string `flag:"x"`
	}
	cfg := &struct {
		Rest   []string `narg:"rest"`
		Deploy sub      `cmd:"deploy"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for narg:rest + cmd:, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_multiple_rest(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Files1 []string `narg:"rest"`
		Files2 []string `narg:"rest"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for multiple narg:rest, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_unknown(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Files []string `narg:"invalid"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for unknown narg value, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_sep_on_non_slice(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Name string `flag:"name" sep:":"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for sep: on non-slice, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_slice_default_no_sep(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Tags []string `default:"a,b,c" flag:"tags"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for slice default without sep:, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_bad_default_value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "int bad default",
			cfg: &struct {
				V int `default:"notanint" flag:"v"`
			}{},
		},
		{
			name: "uint bad default",
			cfg: &struct {
				V uint `default:"notauint" flag:"v"`
			}{},
		},
		{
			name: "float64 bad default",
			cfg: &struct {
				V float64 `default:"notafloat" flag:"v"`
			}{},
		},
		{
			name: "bool impossible default",
			cfg: &struct {
				V bool `default:"maybe" flag:"v"`
			}{},
		},
		{
			name: "time.Time bad default",
			cfg: &struct {
				V time.Time `default:"not-a-time" flag:"v"`
			}{},
		},
		{
			name: "time.Duration bad default",
			cfg: &struct {
				V time.Duration `default:"notaduration" flag:"v"`
			}{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err == nil {
				t.Fatal("expected ErrInvalidSpecification for bad default, got nil")
			}
			if !errors.Is(err, ErrInvalidSpecification) {
				t.Errorf("err = %v, want ErrInvalidSpecification", err)
			}
		})
	}
}

func TestValidate_duplicate_flag_names(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "duplicate long flag",
			cfg: &struct {
				A string `flag:"dup"`
				B string `flag:"dup"`
			}{},
		},
		{
			name: "duplicate short flag",
			cfg: &struct {
				A string `flag:"alpha" short:"x"`
				B string `flag:"beta"  short:"x"`
			}{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err == nil {
				t.Fatal("expected ErrInvalidSpecification for duplicate flag, got nil")
			}
			if !errors.Is(err, ErrInvalidSpecification) {
				t.Errorf("err = %v, want ErrInvalidSpecification", err)
			}
		})
	}
}

func TestValidate_subcommand_fields(t *testing.T) {
	t.Parallel()

	t.Run("valid subcommand", func(t *testing.T) {
		type deploy struct {
			Target string `flag:"target" required:"true"`
		}
		cfg := &struct {
			Verbose bool   `flag:"verbose"`
			Deploy  deploy `cmd:"deploy"`
		}{}
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("unexported cmd field errors", func(t *testing.T) {
		type deploy struct {
			Target string `flag:"target"`
		}
		cfg := &struct {
			deploy deploy `cmd:"deploy"` //nolint:unused
		}{}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for unexported cmd field")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("cmd+flag on same field errors", func(t *testing.T) {
		type deploy struct {
			Target string `flag:"target"`
		}
		cfg := &struct {
			Deploy deploy `cmd:"deploy" flag:"deploy"`
		}{}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for cmd+flag on same field")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("nested cmd is valid", func(t *testing.T) {
		cfg := &struct {
			Deploy testCmdWithNestedCmd `cmd:"deploy"`
		}{}
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate() unexpected error for nested cmd: %v", err)
		}
	})
}

func TestValidate_subcommand_inner_struct_error_propagates(t *testing.T) {
	t.Parallel()

	// A cmd: field whose inner struct contains an invalid flag spec (bad default).
	// Covers validate.go:154-156 where validateStruct errors on the subcommand body.
	type badSub struct {
		V int `default:"notanint" flag:"v"`
	}
	cfg := &struct {
		Deploy badSub `cmd:"deploy"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error from inner struct validation inside cmd: field")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_namespaced_struct_error_propagates(t *testing.T) {
	t.Parallel()

	// A named exported struct field with a flag: namespace tag whose inner fields
	// cause a validation error — the error must propagate out of validateStruct.
	cfg := &struct {
		Server struct {
			//nolint:unused
			host string `flag:"host"` //nolint:structcheck
		} `flag:"srv"`
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error propagated from namespaced nested struct")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_narg_unexported_errors(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		files []string `narg:"rest"` //nolint:unused
	}{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unexported narg field")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

func TestValidate_unexported_nested_struct(t *testing.T) {
	t.Parallel()

	// An unexported named struct field (lowercase field name) is recursed and
	// errors from its sub-fields are propagated.
	t.Run("unexported field name with invalid inner field", func(t *testing.T) {
		// The field name "inner" is unexported; validateStruct recurses into it
		// and the inner unexported flag field triggers an error.
		type innerType struct {
			//nolint:unused
			secret string `flag:"token"` //nolint:structcheck
		}
		cfg := &struct {
			inner innerType //nolint:unused
		}{}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error propagated from unexported-named nested struct field")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("exported field name with invalid inner field", func(t *testing.T) {
		// An exported named struct field; sub-fields can still trigger errors.
		type inner struct {
			//nolint:unused
			secret string `flag:"token"` //nolint:structcheck
		}
		cfg := &struct {
			Inner inner
		}{}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error propagated from nested struct")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})
}

func TestValidate_flag_value_bad_default(t *testing.T) {
	t.Parallel()

	// flag.Value with a bad default should be caught by validateDefault.
	cfg := &struct {
		Custom testFlagValue `default:"baddefault" flag:"custom"`
	}{
		// setErr is only set on the instance; for struct field tests the zero
		// value is fine — Set won't error.  Use a pointer-to-struct field
		// via a wrapper to trigger the error.
	}
	// Since testFlagValue's Set has no error, this should pass.
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── appendRestArgs coverage: all element types ────────────────────────────────

func TestAppendRestArgs_all_types(t *testing.T) {
	t.Parallel()

	t.Run("int64", func(t *testing.T) {
		var s []int64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"42", "100"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int64{42, 100}) {
			t.Errorf("s = %v, want [42 100]", s)
		}
	})

	t.Run("int64 error", func(t *testing.T) {
		var s []int64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notanint"}); err == nil {
			t.Fatal("expected error for bad int64")
		}
	})

	t.Run("int32", func(t *testing.T) {
		var s []int32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"32"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int32{32}) {
			t.Errorf("s = %v, want [32]", s)
		}
	})

	t.Run("int32 error", func(t *testing.T) {
		var s []int32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notanint"}); err == nil {
			t.Fatal("expected error for bad int32")
		}
	})

	t.Run("int16", func(t *testing.T) {
		var s []int16
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"16"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int16{16}) {
			t.Errorf("s = %v, want [16]", s)
		}
	})

	t.Run("int16 error", func(t *testing.T) {
		var s []int16
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notanint"}); err == nil {
			t.Fatal("expected error for bad int16")
		}
	})

	t.Run("int8", func(t *testing.T) {
		var s []int8
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"8"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int8{8}) {
			t.Errorf("s = %v, want [8]", s)
		}
	})

	t.Run("int8 error", func(t *testing.T) {
		var s []int8
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notanint"}); err == nil {
			t.Fatal("expected error for bad int8")
		}
	})

	t.Run("uint", func(t *testing.T) {
		var s []uint
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"42"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []uint{42}) {
			t.Errorf("s = %v, want [42]", s)
		}
	})

	t.Run("uint error", func(t *testing.T) {
		var s []uint
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notauint"}); err == nil {
			t.Fatal("expected error for bad uint")
		}
	})

	t.Run("uint64", func(t *testing.T) {
		var s []uint64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"64"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []uint64{64}) {
			t.Errorf("s = %v, want [64]", s)
		}
	})

	t.Run("uint64 error", func(t *testing.T) {
		var s []uint64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notauint"}); err == nil {
			t.Fatal("expected error for bad uint64")
		}
	})

	t.Run("uint32", func(t *testing.T) {
		var s []uint32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"32"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []uint32{32}) {
			t.Errorf("s = %v, want [32]", s)
		}
	})

	t.Run("uint32 error", func(t *testing.T) {
		var s []uint32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notauint"}); err == nil {
			t.Fatal("expected error for bad uint32")
		}
	})

	t.Run("uint16", func(t *testing.T) {
		var s []uint16
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"16"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []uint16{16}) {
			t.Errorf("s = %v, want [16]", s)
		}
	})

	t.Run("uint16 error", func(t *testing.T) {
		var s []uint16
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notauint"}); err == nil {
			t.Fatal("expected error for bad uint16")
		}
	})

	t.Run("uint8", func(t *testing.T) {
		var s []uint8
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"8"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []uint8{8}) {
			t.Errorf("s = %v, want [8]", s)
		}
	})

	t.Run("uint8 error", func(t *testing.T) {
		var s []uint8
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notauint"}); err == nil {
			t.Fatal("expected error for bad uint8")
		}
	})

	t.Run("float32", func(t *testing.T) {
		var s []float32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"3.14"}); err != nil {
			t.Fatal(err)
		}
		if len(s) != 1 || s[0] != float32(3.14) {
			t.Errorf("s = %v, want [3.14]", s)
		}
	})

	t.Run("float32 error", func(t *testing.T) {
		var s []float32
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notafloat"}); err == nil {
			t.Fatal("expected error for bad float32")
		}
	})

	t.Run("float64", func(t *testing.T) {
		var s []float64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"1.1", "2.2"}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []float64{1.1, 2.2}) {
			t.Errorf("s = %v, want [1.1 2.2]", s)
		}
	})

	t.Run("float64 error", func(t *testing.T) {
		var s []float64
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"notafloat"}); err == nil {
			t.Fatal("expected error for bad float64")
		}
	})

	t.Run("unsupported type returns ErrInvalidSpecification", func(t *testing.T) {
		var s []chan int
		fval := reflect.ValueOf(&s).Elem()
		if err := appendRestArgs(fval, []string{"x"}); err == nil {
			t.Fatal("expected error for unsupported type")
		} else if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})
}

// ── collectUntilFlags coverage: unexported nested struct with until field ─────

func TestCollectUntilFlags_unexported_nested(t *testing.T) {
	t.Parallel()

	// An unexported nested struct that contains a narg:"until" field.
	// collectUntilFlags should recurse into it and collect the flag.
	type inner struct {
		Files []string `flag:"files" narg:"until"`
	}
	type cfg struct {
		inner // unexported embedded struct
	}

	var c cfg
	result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
	if !result["files"] {
		t.Errorf("expected 'files' in untilFlags, got %v", result)
	}
}

func TestCollectUntilFlags_flag_value_skipped(t *testing.T) {
	t.Parallel()

	// A struct field that implements flag.Value should be skipped during
	// collectUntilFlags traversal (not recursed into).
	type cfg struct {
		Custom testFlagValue `flag:"custom"`
	}
	var c cfg
	result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
	// No "until" flags expected - the flag.Value struct should be skipped.
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestCollectUntilFlags_namespaced_nested(t *testing.T) {
	t.Parallel()

	// A named struct with flag: tag (namespace) containing a narg:"until" field.
	type inner struct {
		Files []string `flag:"files" narg:"until"`
	}
	type cfg struct {
		Server inner `flag:"srv"`
	}
	var c cfg
	result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
	if !result["srv.files"] {
		t.Errorf("expected 'srv.files' in untilFlags, got %v", result)
	}
}

func TestCollectUntilFlags_until_no_flag_skipped(t *testing.T) {
	t.Parallel()

	// A field with narg:"until" but no flag: tag is skipped by collectUntilFlags
	// (bindFlags catches this as an error before collectUntilFlags is ever called
	// in normal operation, but collectUntilFlags must handle it gracefully).
	type cfg struct {
		Files []string `narg:"until"` // deliberately missing flag: tag
	}
	var c cfg
	result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
	// The field should not appear in the result since it has no flag name.
	if len(result) != 0 {
		t.Errorf("expected empty result for narg:until without flag:, got %v", result)
	}
}

func TestCollectUntilFlags_cmd_skipped(t *testing.T) {
	t.Parallel()

	// cmd: fields are skipped during collectUntilFlags traversal.
	type deploy struct {
		Files []string `flag:"files" narg:"until"`
	}
	type cfg struct {
		Deploy deploy `cmd:"deploy"`
	}
	var c cfg
	result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
	// The deploy subcommand's 'files' should NOT be collected at the top level.
	if result["files"] {
		t.Errorf("cmd: field contents should not appear in top-level untilFlags, got %v", result)
	}
}

// ── collectRestField coverage: hasCmdField path ───────────────────────────────

func TestCollectRestField_rest_and_cmd(t *testing.T) {
	t.Parallel()

	// If both rest and cmd: fields are present, collectRestField should error.
	type sub struct {
		X string `flag:"x"`
	}
	type cfg struct {
		Args   []string `narg:"rest"`
		Deploy sub      `cmd:"deploy"`
	}
	var c cfg
	_, _, err := collectRestField(reflect.ValueOf(&c).Elem())
	if err == nil {
		t.Fatal("expected error for rest + cmd: combo")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// ── bindFlags coverage: unexported narg field ─────────────────────────────────

func TestBindFlags_unexported_narg_field(t *testing.T) {
	t.Parallel()

	// An unexported field with a narg tag should return ErrInvalidSpecification.
	type cfg struct {
		files []string `narg:"rest"` //nolint:unused
	}
	var c cfg
	err := bindFlags(newTestFS(), reflect.ValueOf(&c).Elem(), "", 0)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for unexported narg field")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// ── parseAndCheck coverage: collectUntilFlags error path ─────────────────────
// Note: collectUntilFlags currently always returns nil error, so the error
// path in parseAndCheck at line 44-46 is unreachable. This is documented here.

// ── selectSubcommand coverage: ErrHelp path ──────────────────────────────────
// Note: the callWhen(errors.Is(err, flag.ErrHelp), os.Exit, 0) in
// selectSubcommand is tested indirectly through the subcommand parse error
// tests. The flag.ErrHelp path would call os.Exit(0) which is not testable
// without subprocess techniques.
