package confl

// smoke_test.go — comprehensive table-driven smoke tests for the confl package.
//
// Coverage intent:
//   1.  Scalar types: string, bool, int, int64, uint, uint64, float64, float32,
//       int32, int16, int8, uint32, uint16, uint8
//   2.  Inverted booleans (default:"true" → -no-<name>)
//   3.  Short flag aliases (short:"x")
//   4.  Default values (absent flag uses default)
//   5.  Required fields (required:"true" → ErrInvalidInput when absent)
//   6.  Slice types (accumulate on repeat; comma-separated defaults cleared on first use)
//   7.  time.Time (RFC3339, DateTime, DateOnly; format: tag; default:"now")
//   8.  time.Duration
//   9.  Nested structs (anonymous embedding, flat namespace)
//  10.  Namespaced nested structs (flag:"ns" prefix → -ns.field)
//  11.  flag.Value interface (custom type implementing Set/String)
//  12.  Subcommands via dispatch and conflagureCmd
//  13.  -- terminator (stops flag parsing; remaining args go into narg:"rest" field)
//  14.  narg:"rest" positional collection
//  15.  Error cases: unknown flag, bad value type, duplicate flag, required missing

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/ers"
)

// splitArgs splits a space-separated argument string into a []string.
// An empty string returns nil (no arguments).
func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// ── 1. Scalar types ──────────────────────────────────────────────────────────

// want_string is the result struct for TestSmoke_String.
type want_string struct{ Name string }

func (w want_string) IsEqual(t *testing.T, got want_string) {
	t.Helper()
	if w.Name != got.Name {
		t.Errorf("Name: want %q, got %q", w.Name, got.Name)
	}
}

func TestSmoke_String(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Name string `flag:"name" default:"world"`
	}

	tests := []struct {
		name string
		args string
		want want_string
		err  error
	}{
		{name: "default", args: "", want: want_string{Name: "world"}},
		{name: "explicit", args: "-name alice", want: want_string{Name: "alice"}},
		{name: "override-default", args: "-name bob", want: want_string{Name: "bob"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, want_string{Name: c.Name})
		})
	}
}

// ── Bool ─────────────────────────────────────────────────────────────────────

type wantBool struct{ Flag bool }

func (w wantBool) IsEqual(t *testing.T, got wantBool) {
	t.Helper()
	if w.Flag != got.Flag {
		t.Errorf("Flag: want %v, got %v", w.Flag, got.Flag)
	}
}

func TestSmoke_Bool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Flag bool `flag:"flag" default:"false" short:"f"`
	}

	tests := []struct {
		name string
		args string
		want wantBool
		err  error
	}{
		{name: "default-false", args: "", want: wantBool{Flag: false}},
		{name: "long-flag", args: "-flag", want: wantBool{Flag: true}},
		{name: "short-flag", args: "-f", want: wantBool{Flag: true}},
		{name: "explicit-true", args: "-flag=true", want: wantBool{Flag: true}},
		{name: "explicit-false", args: "-flag=false", want: wantBool{Flag: false}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantBool{Flag: c.Flag})
		})
	}
}

// ── Inverted bool ─────────────────────────────────────────────────────────────

type wantInvertedBool struct{ Enabled bool }

func (w wantInvertedBool) IsEqual(t *testing.T, got wantInvertedBool) {
	t.Helper()
	if w.Enabled != got.Enabled {
		t.Errorf("Enabled: want %v, got %v", w.Enabled, got.Enabled)
	}
}

func TestSmoke_InvertedBool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Enabled bool `flag:"enabled" default:"true" short:"e"`
	}

	tests := []struct {
		name string
		args string
		want wantInvertedBool
		err  error
	}{
		{name: "default-true", args: "", want: wantInvertedBool{Enabled: true}},
		{name: "negate-long", args: "-no-enabled", want: wantInvertedBool{Enabled: false}},
		{name: "negate-short", args: "-no-e", want: wantInvertedBool{Enabled: false}},
		{name: "no-flag=true-disables", args: "-no-enabled=true", want: wantInvertedBool{Enabled: false}},
		{name: "no-flag=false-leaves-enabled", args: "-no-enabled=false", want: wantInvertedBool{Enabled: true}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantInvertedBool{Enabled: c.Enabled})
		})
	}
}

// ── Numeric scalars ───────────────────────────────────────────────────────────

type wantNumerics struct {
	I    int
	I64  int64
	U    uint
	U64  uint64
	F64  float64
	F32  float32
	I32  int32
	I16  int16
	I8   int8
	U32  uint32
	U16  uint16
	U8   uint8
}

func (w wantNumerics) IsEqual(t *testing.T, got wantNumerics) {
	t.Helper()
	if w.I != got.I {
		t.Errorf("I: want %d, got %d", w.I, got.I)
	}
	if w.I64 != got.I64 {
		t.Errorf("I64: want %d, got %d", w.I64, got.I64)
	}
	if w.U != got.U {
		t.Errorf("U: want %d, got %d", w.U, got.U)
	}
	if w.U64 != got.U64 {
		t.Errorf("U64: want %d, got %d", w.U64, got.U64)
	}
	if w.F64 != got.F64 {
		t.Errorf("F64: want %f, got %f", w.F64, got.F64)
	}
	if w.F32 != got.F32 {
		t.Errorf("F32: want %f, got %f", w.F32, got.F32)
	}
	if w.I32 != got.I32 {
		t.Errorf("I32: want %d, got %d", w.I32, got.I32)
	}
	if w.I16 != got.I16 {
		t.Errorf("I16: want %d, got %d", w.I16, got.I16)
	}
	if w.I8 != got.I8 {
		t.Errorf("I8: want %d, got %d", w.I8, got.I8)
	}
	if w.U32 != got.U32 {
		t.Errorf("U32: want %d, got %d", w.U32, got.U32)
	}
	if w.U16 != got.U16 {
		t.Errorf("U16: want %d, got %d", w.U16, got.U16)
	}
	if w.U8 != got.U8 {
		t.Errorf("U8: want %d, got %d", w.U8, got.U8)
	}
}

func TestSmoke_NumericScalars(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I   int     `flag:"i"`
		I64 int64   `flag:"i64"`
		U   uint    `flag:"u"`
		U64 uint64  `flag:"u64"`
		F64 float64 `flag:"f64"`
		F32 float32 `flag:"f32"`
		I32 int32   `flag:"i32"`
		I16 int16   `flag:"i16"`
		I8  int8    `flag:"i8"`
		U32 uint32  `flag:"u32"`
		U16 uint16  `flag:"u16"`
		U8  uint8   `flag:"u8"`
	}

	tests := []struct {
		name string
		args string
		want wantNumerics
		err  error
	}{
		{
			name: "all-zero-defaults",
			args: "",
			want: wantNumerics{},
		},
		{
			name: "int",
			args: "-i 42",
			want: wantNumerics{I: 42},
		},
		{
			name: "int64",
			args: "-i64 9999999999",
			want: wantNumerics{I64: 9999999999},
		},
		{
			name: "uint",
			args: "-u 1024",
			want: wantNumerics{U: 1024},
		},
		{
			name: "uint64",
			args: "-u64 18446744073709551615",
			want: wantNumerics{U64: 18446744073709551615},
		},
		{
			name: "float64",
			args: "-f64 3.14",
			want: wantNumerics{F64: 3.14},
		},
		{
			name: "float32",
			args: "-f32 1.5",
			want: wantNumerics{F32: 1.5},
		},
		{
			name: "int32-max",
			args: "-i32 2147483647",
			want: wantNumerics{I32: 2147483647},
		},
		{
			name: "int16-max",
			args: "-i16 32767",
			want: wantNumerics{I16: 32767},
		},
		{
			name: "int8-max",
			args: "-i8 127",
			want: wantNumerics{I8: 127},
		},
		{
			name: "uint32-max",
			args: "-u32 4294967295",
			want: wantNumerics{U32: 4294967295},
		},
		{
			name: "uint16-max",
			args: "-u16 65535",
			want: wantNumerics{U16: 65535},
		},
		{
			name: "uint8-max",
			args: "-u8 255",
			want: wantNumerics{U8: 255},
		},
		{
			name: "negative-int8",
			args: "-i8 -128",
			want: wantNumerics{I8: -128},
		},
		{
			name: "negative-int32",
			args: "-i32 -2147483648",
			want: wantNumerics{I32: -2147483648},
		},
		{
			// int8 overflow — value 200 exceeds int8 range
			name: "int8-overflow",
			args: "-i8 200",
			err:  errors.New("overflow"), // any non-nil causes Is-check only vs ErrInvalidInput not needed
		},
		{
			name: "uint8-overflow",
			args: "-u8 300",
			err:  errors.New("overflow"),
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := wantNumerics{
				I: c.I, I64: c.I64, U: c.U, U64: c.U64,
				F64: c.F64, F32: c.F32,
				I32: c.I32, I16: c.I16, I8: c.I8,
				U32: c.U32, U16: c.U16, U8: c.U8,
			}
			tt.want.IsEqual(t, got)
		})
	}
}

// ── 3. Short flag aliases ──────────────────────────────────────────────────────

type wantShort struct {
	Name    string
	Verbose bool
	Count   int
}

func (w wantShort) IsEqual(t *testing.T, got wantShort) {
	t.Helper()
	if w.Name != got.Name {
		t.Errorf("Name: want %q, got %q", w.Name, got.Name)
	}
	if w.Verbose != got.Verbose {
		t.Errorf("Verbose: want %v, got %v", w.Verbose, got.Verbose)
	}
	if w.Count != got.Count {
		t.Errorf("Count: want %d, got %d", w.Count, got.Count)
	}
}

func TestSmoke_ShortAliases(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Name    string `flag:"name"    short:"n" default:"default"`
		Verbose bool   `flag:"verbose" short:"v"`
		Count   int    `flag:"count"   short:"c" default:"0"`
	}

	tests := []struct {
		name string
		args string
		want wantShort
		err  error
	}{
		{name: "defaults", args: "", want: wantShort{Name: "default"}},
		{name: "long-name", args: "-name foo", want: wantShort{Name: "foo"}},
		{name: "short-name", args: "-n bar", want: wantShort{Name: "bar"}},
		{name: "long-verbose", args: "-verbose", want: wantShort{Name: "default", Verbose: true}},
		{name: "short-verbose", args: "-v", want: wantShort{Name: "default", Verbose: true}},
		{name: "long-count", args: "-count 7", want: wantShort{Name: "default", Count: 7}},
		{name: "short-count", args: "-c 9", want: wantShort{Name: "default", Count: 9}},
		{name: "mixed-short-and-long", args: "-n alice -v -c 3", want: wantShort{Name: "alice", Verbose: true, Count: 3}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantShort{Name: c.Name, Verbose: c.Verbose, Count: c.Count})
		})
	}
}

// ── 4. Default values ─────────────────────────────────────────────────────────

type wantDefaults struct {
	S   string
	I   int
	B   bool
	F64 float64
}

func (w wantDefaults) IsEqual(t *testing.T, got wantDefaults) {
	t.Helper()
	if w.S != got.S {
		t.Errorf("S: want %q, got %q", w.S, got.S)
	}
	if w.I != got.I {
		t.Errorf("I: want %d, got %d", w.I, got.I)
	}
	if w.B != got.B {
		t.Errorf("B: want %v, got %v", w.B, got.B)
	}
	if w.F64 != got.F64 {
		t.Errorf("F64: want %f, got %f", w.F64, got.F64)
	}
}

func TestSmoke_Defaults(t *testing.T) {
	t.Parallel()

	type cfg struct {
		S   string  `flag:"s"   default:"hello"`
		I   int     `flag:"i"   default:"42"`
		B   bool    `flag:"b"   default:"false"`
		F64 float64 `flag:"f64" default:"2.71"`
	}

	tests := []struct {
		name string
		args string
		want wantDefaults
		err  error
	}{
		{
			name: "all-defaults",
			args: "",
			want: wantDefaults{S: "hello", I: 42, B: false, F64: 2.71},
		},
		{
			name: "override-string",
			args: "-s world",
			want: wantDefaults{S: "world", I: 42, B: false, F64: 2.71},
		},
		{
			name: "override-int",
			args: "-i 100",
			want: wantDefaults{S: "hello", I: 100, B: false, F64: 2.71},
		},
		{
			name: "override-bool",
			args: "-b",
			want: wantDefaults{S: "hello", I: 42, B: true, F64: 2.71},
		},
		{
			name: "override-all",
			args: "-s bye -i 0 -b -f64 0",
			want: wantDefaults{S: "bye", I: 0, B: true, F64: 0},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantDefaults{S: c.S, I: c.I, B: c.B, F64: c.F64})
		})
	}
}

// ── 5. Required fields ────────────────────────────────────────────────────────

type wantRequired struct{ Target string }

func (w wantRequired) IsEqual(t *testing.T, got wantRequired) {
	t.Helper()
	if w.Target != got.Target {
		t.Errorf("Target: want %q, got %q", w.Target, got.Target)
	}
}

func TestSmoke_Required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Target string `flag:"target" required:"true"`
	}

	tests := []struct {
		name string
		args string
		want wantRequired
		err  error
	}{
		{name: "absent-required-errors", args: "", want: wantRequired{}, err: ErrInvalidInput},
		{name: "satisfied", args: "-target prod", want: wantRequired{Target: "prod"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantRequired{Target: c.Target})
		})
	}
}

// ── 6. Slice types ────────────────────────────────────────────────────────────

type wantSlices struct {
	Tags    []string
	Counts  []int
	Floats  []float64
}

func (w wantSlices) IsEqual(t *testing.T, got wantSlices) {
	t.Helper()
	if !slices.Equal(w.Tags, got.Tags) {
		t.Errorf("Tags: want %v, got %v", w.Tags, got.Tags)
	}
	if !slices.Equal(w.Counts, got.Counts) {
		t.Errorf("Counts: want %v, got %v", w.Counts, got.Counts)
	}
	if !slices.Equal(w.Floats, got.Floats) {
		t.Errorf("Floats: want %v, got %v", w.Floats, got.Floats)
	}
}

func TestSmoke_Slices(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags   []string  `flag:"tag"   short:"t" default:"debug,info" sep:","`
		Counts []int     `flag:"count" short:"c"`
		Floats []float64 `flag:"float" short:"f"`
	}

	tests := []struct {
		name string
		args string
		want wantSlices
		err  error
	}{
		{
			name: "string-defaults",
			args: "",
			want: wantSlices{Tags: []string{"debug", "info"}},
		},
		{
			name: "string-defaults-cleared-on-explicit",
			args: "-tag warn",
			want: wantSlices{Tags: []string{"warn"}},
		},
		{
			name: "string-multiple-long",
			args: "-tag a -tag b -tag c",
			want: wantSlices{Tags: []string{"a", "b", "c"}},
		},
		{
			name: "string-short-alias",
			args: "-t x",
			want: wantSlices{Tags: []string{"x"}},
		},
		{
			name: "string-mixed-long-short",
			args: "-tag alpha -t beta",
			want: wantSlices{Tags: []string{"alpha", "beta"}},
		},
		{
			name: "int-accumulates",
			args: "-count 1 -count 2 -count 3",
			want: wantSlices{Tags: []string{"debug", "info"}, Counts: []int{1, 2, 3}},
		},
		{
			name: "int-short-alias",
			args: "-c 10 -c 20",
			want: wantSlices{Tags: []string{"debug", "info"}, Counts: []int{10, 20}},
		},
		{
			name: "float-accumulates",
			args: "-float 1.1 -float 2.2",
			want: wantSlices{Tags: []string{"debug", "info"}, Floats: []float64{1.1, 2.2}},
		},
		{
			name: "no-flags-nil-counts-floats",
			args: "",
			want: wantSlices{Tags: []string{"debug", "info"}, Counts: nil, Floats: nil},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantSlices{Tags: c.Tags, Counts: c.Counts, Floats: c.Floats})
		})
	}
}

// ── 6b. Required slice ────────────────────────────────────────────────────────

type wantRequiredSlice struct{ Items []string }

func (w wantRequiredSlice) IsEqual(t *testing.T, got wantRequiredSlice) {
	t.Helper()
	if !slices.Equal(w.Items, got.Items) {
		t.Errorf("Items: want %v, got %v", w.Items, got.Items)
	}
}

func TestSmoke_RequiredSlice(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Items []string `flag:"item" required:"true"`
	}

	tests := []struct {
		name string
		args string
		want wantRequiredSlice
		err  error
	}{
		{name: "absent-errors", args: "", want: wantRequiredSlice{}, err: ErrInvalidInput},
		{name: "satisfied", args: "-item foo", want: wantRequiredSlice{Items: []string{"foo"}}},
		{name: "multiple", args: "-item a -item b", want: wantRequiredSlice{Items: []string{"a", "b"}}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantRequiredSlice{Items: c.Items})
		})
	}
}

// ── 7. time.Time ──────────────────────────────────────────────────────────────

type wantTime struct{ At time.Time }

func (w wantTime) IsEqual(t *testing.T, got wantTime) {
	t.Helper()
	if !w.At.Equal(got.At) {
		t.Errorf("At: want %v, got %v", w.At, got.At)
	}
}

func TestSmoke_TimeTime(t *testing.T) {
	t.Parallel()

	type cfg struct {
		At time.Time `flag:"at"`
	}

	rfc3339val := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	datetimeval := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	dateonlyval := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		args string
		want wantTime
		err  error
	}{
		{
			name: "zero-when-absent",
			args: "",
			want: wantTime{At: time.Time{}},
		},
		{
			name: "RFC3339",
			args: "-at 2024-06-01T12:00:00Z",
			want: wantTime{At: rfc3339val},
		},
		// NOTE: DateTime format "2024-03-15 10:30:00" cannot be tested via
		// splitArgs because it splits on whitespace. Use TestSmoke_TimeTime_DateTime
		// (which pre-splits the args slice) for that format. The case is
		// intentionally omitted here to avoid false failures.
		{
			name: "DateOnly",
			args: "-at 2024-03-15",
			want: wantTime{At: dateonlyval},
		},
		{
			name: "invalid-errors",
			args: "-at not-a-time",
			err:  errors.New("parse error"),
		},
	}
	_ = datetimeval // avoid unused warning

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantTime{At: c.At})
		})
	}
}

func TestSmoke_TimeTime_formats(t *testing.T) {
	t.Parallel()

	t.Run("DateTime format with pre-split args", func(t *testing.T) {
		// DateTime format contains a space; pass args as a pre-split slice.
		type cfg struct {
			At time.Time `flag:"at"`
		}
		var c cfg
		args := []string{"-at", "2024-03-15 10:30:00"}
		if err := conflagure(newTestFS(), &c, args); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
		if !c.At.Equal(want) {
			t.Errorf("At = %v, want %v", c.At, want)
		}
	})

	t.Run("now default", func(t *testing.T) {
		type cfg struct {
			At time.Time `flag:"at" default:"now"`
		}
		before := time.Now().UTC().Add(-time.Second)
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !c.At.After(before) {
			t.Errorf("At = %v, expected after %v", c.At, before)
		}
	})

	t.Run("custom format tag", func(t *testing.T) {
		type cfg struct {
			Date time.Time `flag:"date" format:"2006-01-02"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-date", "2024-03-15"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		if !c.Date.Equal(want) {
			t.Errorf("Date = %v, want %v", c.Date, want)
		}
	})

	t.Run("short alias", func(t *testing.T) {
		type cfg struct {
			At time.Time `flag:"at" short:"a"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-a", "2024-06-01T00:00:00Z"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.At.IsZero() {
			t.Error("At is zero after setting via short alias")
		}
	})

	t.Run("required absent errors", func(t *testing.T) {
		type cfg struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("required satisfied", func(t *testing.T) {
		type cfg struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-at", "2024-01-01T00:00:00Z"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ── 8. time.Duration ──────────────────────────────────────────────────────────

type wantDuration struct{ Timeout time.Duration }

func (w wantDuration) IsEqual(t *testing.T, got wantDuration) {
	t.Helper()
	if w.Timeout != got.Timeout {
		t.Errorf("Timeout: want %v, got %v", w.Timeout, got.Timeout)
	}
}

func TestSmoke_Duration(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Timeout time.Duration `flag:"timeout" short:"t"`
	}

	tests := []struct {
		name string
		args string
		want wantDuration
		err  error
	}{
		{name: "zero-when-absent", args: "", want: wantDuration{}},
		{name: "parse-hours-minutes", args: "-timeout 1h30m", want: wantDuration{Timeout: 90 * time.Minute}},
		{name: "parse-seconds", args: "-timeout 30s", want: wantDuration{Timeout: 30 * time.Second}},
		{name: "parse-ms", args: "-timeout 500ms", want: wantDuration{Timeout: 500 * time.Millisecond}},
		{name: "short-alias", args: "-t 2m", want: wantDuration{Timeout: 2 * time.Minute}},
		{name: "invalid-errors", args: "-timeout notaduration", err: errors.New("parse error")},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantDuration{Timeout: c.Timeout})
		})
	}
}

func TestSmoke_Duration_Default(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Timeout time.Duration `flag:"timeout" default:"5s"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.Timeout)
	}
}

// ── 9. Nested structs (anonymous embedding, flat namespace) ───────────────────

type wantNested struct {
	Verbose bool
	Format  string
	Name    string
}

func (w wantNested) IsEqual(t *testing.T, got wantNested) {
	t.Helper()
	if w.Verbose != got.Verbose {
		t.Errorf("Verbose: want %v, got %v", w.Verbose, got.Verbose)
	}
	if w.Format != got.Format {
		t.Errorf("Format: want %q, got %q", w.Format, got.Format)
	}
	if w.Name != got.Name {
		t.Errorf("Name: want %q, got %q", w.Name, got.Name)
	}
}

func TestSmoke_AnonymousEmbedding(t *testing.T) {
	t.Parallel()

	type base struct {
		Verbose bool   `flag:"verbose" short:"v"`
		Format  string `flag:"format"  default:"text"`
	}
	type cfg struct {
		base
		Name string `flag:"name"`
	}

	tests := []struct {
		name string
		args string
		want wantNested
		err  error
	}{
		{name: "defaults", args: "", want: wantNested{Format: "text"}},
		{name: "embedded-bool-long", args: "-verbose", want: wantNested{Verbose: true, Format: "text"}},
		{name: "embedded-bool-short", args: "-v", want: wantNested{Verbose: true, Format: "text"}},
		{name: "embedded-string", args: "-format json", want: wantNested{Format: "json"}},
		{name: "outer-field", args: "-name alice", want: wantNested{Format: "text", Name: "alice"}},
		{name: "all-together", args: "-v -format json -name bob", want: wantNested{Verbose: true, Format: "json", Name: "bob"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantNested{Verbose: c.Verbose, Format: c.Format, Name: c.Name})
		})
	}
}

func TestSmoke_NamedStructFlatNamespace(t *testing.T) {
	t.Parallel()

	// Named struct field without a flag: tag → flat namespace.
	type network struct {
		Host string `flag:"host" default:"localhost"`
		Port int    `flag:"port" default:"8080"`
	}
	type cfg struct {
		Server network // no flag: tag → flat
	}

	type want struct {
		Host string
		Port int
	}
	eq := func(t *testing.T, w, got want) {
		t.Helper()
		if w.Host != got.Host {
			t.Errorf("Host: want %q, got %q", w.Host, got.Host)
		}
		if w.Port != got.Port {
			t.Errorf("Port: want %d, got %d", w.Port, got.Port)
		}
	}

	tests := []struct {
		name string
		args string
		want want
		err  error
	}{
		{name: "defaults", args: "", want: want{Host: "localhost", Port: 8080}},
		{name: "explicit-host", args: "-host prod.db", want: want{Host: "prod.db", Port: 8080}},
		{name: "explicit-port", args: "-port 9090", want: want{Host: "localhost", Port: 9090}},
		{name: "both", args: "-host h -port 1", want: want{Host: "h", Port: 1}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			eq(t, tt.want, want{Host: c.Server.Host, Port: c.Server.Port})
		})
	}
}

// ── 10. Namespaced nested structs ─────────────────────────────────────────────

type wantNamespaced struct {
	Host  string
	Port  int
	Label string
}

func (w wantNamespaced) IsEqual(t *testing.T, got wantNamespaced) {
	t.Helper()
	if w.Host != got.Host {
		t.Errorf("Host: want %q, got %q", w.Host, got.Host)
	}
	if w.Port != got.Port {
		t.Errorf("Port: want %d, got %d", w.Port, got.Port)
	}
	if w.Label != got.Label {
		t.Errorf("Label: want %q, got %q", w.Label, got.Label)
	}
}

func TestSmoke_NamespacedStruct(t *testing.T) {
	t.Parallel()

	type network struct {
		Host string `flag:"host" default:"localhost"`
		Port int    `flag:"port" default:"8080"`
	}
	type cfg struct {
		Server network `flag:"srv"`
		Label  string  `flag:"label"`
	}

	tests := []struct {
		name string
		args string
		want wantNamespaced
		err  error
	}{
		{name: "defaults", args: "", want: wantNamespaced{Host: "localhost", Port: 8080}},
		{name: "namespaced-host", args: "-srv.host example.com", want: wantNamespaced{Host: "example.com", Port: 8080}},
		{name: "namespaced-port", args: "-srv.port 9090", want: wantNamespaced{Host: "localhost", Port: 9090}},
		{name: "top-level-label", args: "-label staging", want: wantNamespaced{Host: "localhost", Port: 8080, Label: "staging"}},
		{
			name: "all-together",
			args: "-srv.host prod -srv.port 443 -label prod",
			want: wantNamespaced{Host: "prod", Port: 443, Label: "prod"},
		},
		{
			// Using the old flat-namespace name should fail (no -host flag registered)
			name: "old-flat-name-unknown",
			args: "-host xxx",
			err:  errors.New("unknown flag"),
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantNamespaced{Host: c.Server.Host, Port: c.Server.Port, Label: c.Label})
		})
	}
}

func TestSmoke_NestedNamespaceAccumulates(t *testing.T) {
	t.Parallel()

	// Nested namespaces: outer.inner.key
	type leaf struct {
		Key string `flag:"key" default:"val"`
	}
	type middle struct {
		Inner leaf `flag:"inner"`
	}
	type cfg struct {
		Outer middle `flag:"outer"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"-outer.inner.key", "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Outer.Inner.Key != "hello" {
		t.Errorf("Outer.Inner.Key = %q, want %q", c.Outer.Inner.Key, "hello")
	}
}

// ── 11. flag.Value interface ──────────────────────────────────────────────────

// smokeMultiValue is a custom flag.Value that accumulates Set calls.
type smokeMultiValue struct {
	vals   []string
	setErr error
}

func (v *smokeMultiValue) String() string {
	if len(v.vals) == 0 {
		return ""
	}
	return v.vals[len(v.vals)-1]
}

func (v *smokeMultiValue) Set(s string) error {
	if v.setErr != nil {
		return v.setErr
	}
	v.vals = append(v.vals, s)
	return nil
}

type wantFlagValue struct{ Val string }

func (w wantFlagValue) IsEqual(t *testing.T, got wantFlagValue) {
	t.Helper()
	if w.Val != got.Val {
		t.Errorf("Val: want %q, got %q", w.Val, got.Val)
	}
}

func TestSmoke_FlagValueInterface(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Custom smokeMultiValue `flag:"custom" default:"defval"`
	}

	tests := []struct {
		name string
		args string
		want wantFlagValue
		err  error
	}{
		{name: "default-applied", args: "", want: wantFlagValue{Val: "defval"}},
		{name: "explicit-overrides", args: "-custom explicit", want: wantFlagValue{Val: "explicit"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want.IsEqual(t, wantFlagValue{Val: c.Custom.String()})
		})
	}
}

func TestSmoke_FlagValueInterface_extra(t *testing.T) {
	t.Parallel()

	t.Run("short alias", func(t *testing.T) {
		type cfg struct {
			Custom smokeMultiValue `flag:"custom" short:"c"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-c", "via-short"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Custom.String() != "via-short" {
			t.Errorf("Custom = %q, want %q", c.Custom.String(), "via-short")
		}
	})

	t.Run("bad default surfaces ErrInvalidSpecification", func(t *testing.T) {
		fs := newTestFS()
		v := &smokeMultiValue{setErr: errors.New("rejected")}
		err := registerFlag(fs, v, flagSpec{Name: "custom", Default: "badval"})
		if err == nil {
			t.Fatal("expected error for failing Set on default, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("required absent errors", func(t *testing.T) {
		type cfg struct {
			Custom smokeMultiValue `flag:"custom" required:"true"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})
}

// ── 12. Subcommands via dispatch and conflagureCmd ────────────────────────────

// smokeDeployCmd and smokeRollbackCmd are separate from the existing testDeployCmd
// to keep these smoke tests self-contained.

type smokeDeployCmd struct {
	Target string `flag:"target" required:"true"`
	DryRun bool   `flag:"dry-run" short:"n"`
}

func (d *smokeDeployCmd) Run(_ context.Context) error { return nil }

type smokeRollbackCmd struct {
	Version string `flag:"version" required:"true"`
	Force   bool   `flag:"force"`
}

func (r *smokeRollbackCmd) Run(_ context.Context) error { return nil }

// wantDispatch captures which subcommand was chosen plus a couple of its fields.
type wantDispatch struct {
	CmdType string // "deploy" or "rollback"
	Target  string
	Version string
	Force   bool
	DryRun  bool
}

func (w wantDispatch) IsEqual(t *testing.T, got wantDispatch) {
	t.Helper()
	if w.CmdType != got.CmdType {
		t.Errorf("CmdType: want %q, got %q", w.CmdType, got.CmdType)
	}
	if w.Target != got.Target {
		t.Errorf("Target: want %q, got %q", w.Target, got.Target)
	}
	if w.Version != got.Version {
		t.Errorf("Version: want %q, got %q", w.Version, got.Version)
	}
	if w.Force != got.Force {
		t.Errorf("Force: want %v, got %v", w.Force, got.Force)
	}
	if w.DryRun != got.DryRun {
		t.Errorf("DryRun: want %v, got %v", w.DryRun, got.DryRun)
	}
}

func TestSmoke_Subcommands_conflagureCmd(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose  bool             `flag:"verbose" short:"v"`
		Deploy   smokeDeployCmd   `cmd:"deploy"`
		Rollback smokeRollbackCmd `cmd:"rollback"`
	}

	tests := []struct {
		name string
		args []string // pre-split because subcommand args can't use splitArgs safely
		want wantDispatch
		err  error
	}{
		{
			name: "deploy",
			args: []string{"deploy", "-target=prod"},
			want: wantDispatch{CmdType: "deploy", Target: "prod"},
		},
		{
			name: "deploy-with-dry-run-short",
			args: []string{"deploy", "-target=staging", "-n"},
			want: wantDispatch{CmdType: "deploy", Target: "staging", DryRun: true},
		},
		{
			name: "rollback",
			args: []string{"rollback", "-version=v1.2", "-force"},
			want: wantDispatch{CmdType: "rollback", Version: "v1.2", Force: true},
		},
		{
			name: "global-flag-then-subcommand",
			args: []string{"-verbose", "deploy", "-target=x"},
			want: wantDispatch{CmdType: "deploy", Target: "x"},
		},
		{
			name: "no-subcommand-errors",
			args: nil,
			err:  ers.ErrNotFound,
		},
		{
			name: "unknown-subcommand-errors",
			args: []string{"bogus"},
			err:  ErrInvalidInput,
		},
		{
			name: "required-flag-missing-in-subcommand",
			args: []string{"deploy"}, // -target required
			err:  ErrInvalidInput,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			cmd, err := conflagureCmd(newTestFS(), &c, tt.args)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got wantDispatch
			switch v := cmd.(type) {
			case *smokeDeployCmd:
				got = wantDispatch{CmdType: "deploy", Target: v.Target, DryRun: v.DryRun}
			case *smokeRollbackCmd:
				got = wantDispatch{CmdType: "rollback", Version: v.Version, Force: v.Force}
			default:
				t.Fatalf("unexpected cmd type %T", cmd)
			}
			tt.want.IsEqual(t, got)
		})
	}
}

func TestSmoke_Subcommands_dispatch(t *testing.T) {
	t.Parallel()

	// dispatch does not require Commander; use plain structs.
	type deployCmd struct {
		Target string `flag:"target" required:"true"`
	}
	type cfg struct {
		Deploy deployCmd `cmd:"deploy"`
	}

	tests := []struct {
		name   string
		args   []string
		target string
		err    error
	}{
		{
			name:   "deploy-selected",
			args:   []string{"deploy", "-target=qa"},
			target: "qa",
		},
		{
			name: "no-subcommand-returns-ErrNotFound",
			args: nil,
			err:  ers.ErrNotFound,
		},
		{
			name: "unknown-subcommand-errors",
			args: []string{"nosuch"},
			err:  ErrInvalidInput,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			result, err := dispatch(newTestFS(), &c, tt.args)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cmd, ok := result.(*deployCmd)
			if !ok {
				t.Fatalf("result type = %T, want *deployCmd", result)
			}
			if cmd.Target != tt.target {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.target)
			}
		})
	}
}

func TestSmoke_Subcommands_RequiredSubcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy smokeDeployCmd `cmd:"deploy" required:"true"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// ── 13 & 14. -- terminator and narg:"rest" ────────────────────────────────────

func TestSmoke_DashDash_Terminator(t *testing.T) {
	t.Parallel()

	// The flag package stops at "--"; remaining args become positional (fs.Args()).
	// With narg:"rest" the positional args are collected into the slice.
	type cfg struct {
		Verbose bool     `flag:"verbose" short:"v"`
		Rest    []string `narg:"rest"`
	}

	tests := []struct {
		name    string
		args    []string
		verbose bool
		rest    []string
		err     error
	}{
		{
			name:    "no-double-dash",
			args:    []string{"-v"},
			verbose: true,
			rest:    nil,
		},
		{
			name:    "double-dash-captures-rest",
			args:    []string{"-v", "--", "a", "b", "c"},
			verbose: true,
			rest:    []string{"a", "b", "c"},
		},
		{
			name: "double-dash-no-flags-before",
			args: []string{"--", "x", "y"},
			rest: []string{"x", "y"},
		},
		{
			name: "rest-after-known-positional",
			// No "--" needed; unknown positional args stop flag parsing too.
			args: []string{"--", "hello"},
			rest: []string{"hello"},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.Verbose != tt.verbose {
				t.Errorf("Verbose: want %v, got %v", tt.verbose, c.Verbose)
			}
			if !slices.Equal(c.Rest, tt.rest) {
				t.Errorf("Rest: want %v, got %v", tt.rest, c.Rest)
			}
		})
	}
}

func TestSmoke_NargRest_Int(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Output string `flag:"out" default:"-"`
		Rest   []int  `narg:"rest"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"-out", "file.txt", "--", "1", "2", "3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Output != "file.txt" {
		t.Errorf("Output = %q, want %q", c.Output, "file.txt")
	}
	if !slices.Equal(c.Rest, []int{1, 2, 3}) {
		t.Errorf("Rest = %v, want [1 2 3]", c.Rest)
	}
}

func TestSmoke_NargRest_Required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Rest []string `narg:"rest" required:"true"`
	}

	t.Run("absent-errors", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("satisfied", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"--", "arg1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(c.Rest, []string{"arg1"}) {
			t.Errorf("Rest = %v, want [arg1]", c.Rest)
		}
	})
}

// ── 15. Error cases ───────────────────────────────────────────────────────────

func TestSmoke_Errors_UnknownFlag(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Name string `flag:"name"`
	}

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{name: "unknown-flag", args: "-unknown-flag", wantErr: true},
		{name: "known-flag-ok", args: "-name foo", wantErr: false},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, splitArgs(tt.args))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestSmoke_Errors_BadValueType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
		args []string
	}{
		{
			name: "int-non-numeric",
			cfg:  &struct{ V int `flag:"v"` }{},
			args: []string{"-v", "notanint"},
		},
		{
			name: "float64-non-numeric",
			cfg:  &struct{ V float64 `flag:"v"` }{},
			args: []string{"-v", "notafloat"},
		},
		{
			name: "uint-negative",
			cfg:  &struct{ V uint `flag:"v"` }{},
			args: []string{"-v", "-1"},
		},
		{
			name: "duration-invalid",
			cfg:  &struct{ V time.Duration `flag:"v"` }{},
			args: []string{"-v", "notaduration"},
		},
		{
			name: "time-invalid",
			cfg:  &struct{ V time.Time `flag:"v"` }{},
			args: []string{"-v", "not-a-time"},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if err := conflagure(newTestFS(), tt.cfg, tt.args); err == nil {
				t.Fatalf("expected error for bad value type, got nil")
			}
		})
	}
}

func TestSmoke_Errors_DuplicateFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "duplicate-long-flag",
			cfg: &struct {
				A string `flag:"dup"`
				B string `flag:"dup"`
			}{},
		},
		{
			name: "duplicate-short-flag",
			cfg: &struct {
				A string `flag:"alpha" short:"x"`
				B string `flag:"beta"  short:"x"`
			}{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			err := conflagure(newTestFS(), tt.cfg, nil)
			if err == nil {
				t.Fatal("expected error for duplicate flag, got nil")
			}
			if !errors.Is(err, ErrInvalidSpecification) {
				t.Errorf("err = %v, want ErrInvalidSpecification", err)
			}
		})
	}
}

func TestSmoke_Errors_InvalidDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "int-bad-default",
			cfg:  &struct{ V int `flag:"v" default:"notanint"` }{},
		},
		{
			name: "uint-bad-default",
			cfg:  &struct{ V uint `flag:"v" default:"notauint"` }{},
		},
		{
			name: "float64-bad-default",
			cfg:  &struct{ V float64 `flag:"v" default:"notafloat"` }{},
		},
		{
			name: "bool-impossible-default",
			cfg:  &struct{ V bool `flag:"v" default:"maybe"` }{},
		},
		{
			name: "time-bad-default",
			cfg:  &struct{ V time.Time `flag:"v" default:"not-a-time"` }{},
		},
		{
			name: "duration-bad-default",
			cfg:  &struct{ V time.Duration `flag:"v" default:"notaduration"` }{},
		},
		{
			name: "int8-overflow-default",
			cfg:  &struct{ V int8 `flag:"v" default:"200"` }{},
		},
		{
			name: "uint8-overflow-default",
			cfg:  &struct{ V uint8 `flag:"v" default:"300"` }{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			err := conflagure(newTestFS(), tt.cfg, nil)
			if err == nil {
				t.Fatal("expected error for invalid default, got nil")
			}
			if !errors.Is(err, ErrInvalidSpecification) {
				t.Errorf("err = %v, want ErrInvalidSpecification", err)
			}
		})
	}
}

// TestSmoke_Errors_UnsupportedType verifies that unsupported field types
// produce ErrInvalidSpecification.
func TestSmoke_Errors_UnsupportedType(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Ch chan int `flag:"ch"`
	}

	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !errors.Is(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// ── Additional smoke: slice time types ────────────────────────────────────────

func TestSmoke_SliceTime(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Dates    []time.Time     `flag:"date"    short:"d"`
		Timeouts []time.Duration `flag:"timeout" short:"t"`
	}

	t.Run("time-accumulates", func(t *testing.T) {
		var c cfg
		args := []string{"-date", "2024-01-01T00:00:00Z", "-date", "2024-06-01T00:00:00Z"}
		if err := conflagure(newTestFS(), &c, args); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(c.Dates) != 2 {
			t.Errorf("len(Dates) = %d, want 2", len(c.Dates))
		}
	})

	t.Run("duration-accumulates", func(t *testing.T) {
		var c cfg
		args := []string{"-timeout", "1s", "-timeout", "2m"}
		if err := conflagure(newTestFS(), &c, args); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []time.Duration{time.Second, 2 * time.Minute}
		if !slices.Equal(c.Timeouts, want) {
			t.Errorf("Timeouts = %v, want %v", c.Timeouts, want)
		}
	})

	t.Run("short-alias-time", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-d", "2024-03-15T00:00:00Z"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(c.Dates) != 1 {
			t.Errorf("len(Dates) = %d, want 1", len(c.Dates))
		}
	})

	t.Run("nil-when-absent", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Dates != nil || c.Timeouts != nil {
			t.Error("expected nil slices when no flags given")
		}
	})
}

// ── Additional smoke: small-int slice types ───────────────────────────────────

func TestSmoke_SliceSmallInts(t *testing.T) {
	t.Parallel()

	type cfg struct {
		I8s  []int8  `flag:"i8"`
		I16s []int16 `flag:"i16"`
		I32s []int32 `flag:"i32"`
		U8s  []uint8 `flag:"u8"`
	}

	t.Run("accumulate", func(t *testing.T) {
		var c cfg
		args := []string{"-i8", "1", "-i8", "2", "-i16", "100", "-i32", "1000", "-u8", "10", "-u8", "20"}
		if err := conflagure(newTestFS(), &c, args); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(c.I8s, []int8{1, 2}) {
			t.Errorf("I8s = %v, want [1 2]", c.I8s)
		}
		if !slices.Equal(c.I16s, []int16{100}) {
			t.Errorf("I16s = %v, want [100]", c.I16s)
		}
		if !slices.Equal(c.I32s, []int32{1000}) {
			t.Errorf("I32s = %v, want [1000]", c.I32s)
		}
		if !slices.Equal(c.U8s, []uint8{10, 20}) {
			t.Errorf("U8s = %v, want [10 20]", c.U8s)
		}
	})
}

// TestSmoke_Scalars_String was kept above as the first test, but it has a dead
// first version. Remove duplicate by ensuring only one TestSmoke_Scalars_String
// variant compiles. The version using want_string's IsEqual method is the live
// one; the stub above compiles but does nothing harmful.
