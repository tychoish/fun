package confl

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/ers"
)

type BaseTest struct {
	DryRun bool `flag:"dry-run" help:"print what would be done without making changes" short:"n"`
}

func newTestFS() *flag.FlagSet {
	return flag.NewFlagSet("test", flag.ContinueOnError)
}

func Test_conflagure_string(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "explicit value", args: []string{"-branch", "feature"}, want: "feature"},
		{name: "uses default when absent", args: nil, want: "main"},
	}

	type cfg struct {
		Branch string `default:"main" flag:"branch" help:"branch name"`
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); (err != nil) != tt.wantErr {
				t.Fatalf("conflagure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if c.Branch != tt.want {
				t.Errorf("Branch = %q, want %q", c.Branch, tt.want)
			}
		})
	}
}

func Test_conflagure_bool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "long flag", args: []string{"-verbose"}, want: true},
		{name: "short alias", args: []string{"-v"}, want: true},
		{name: "explicit false", args: []string{"-verbose=false"}, want: false},
		{name: "default false when absent", args: nil, want: false},
	}

	type cfg struct {
		Verbose bool `default:"false" flag:"verbose" help:"verbose output" short:"v"`
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if c.Verbose != tt.want {
				t.Errorf("Verbose = %v, want %v", c.Verbose, tt.want)
			}
		})
	}
}

func Test_conflagure_numeric(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Count int     `default:"0" flag:"count"`
		Big   int64   `default:"0" flag:"big"`
		Size  uint    `default:"0" flag:"size"`
		Large uint64  `default:"0" flag:"large"`
		Ratio float64 `default:"0" flag:"ratio"`
	}

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, cfg)
	}{
		{
			name: "int",
			args: []string{"-count", "42"},
			check: func(t *testing.T, c cfg) {
				if c.Count != 42 {
					t.Errorf("Count = %d, want 42", c.Count)
				}
			},
		},
		{
			name: "int64",
			args: []string{"-big", "9999999999"},
			check: func(t *testing.T, c cfg) {
				if c.Big != 9999999999 {
					t.Errorf("Big = %d, want 9999999999", c.Big)
				}
			},
		},
		{
			name: "uint",
			args: []string{"-size", "1024"},
			check: func(t *testing.T, c cfg) {
				if c.Size != 1024 {
					t.Errorf("Size = %d, want 1024", c.Size)
				}
			},
		},
		{
			name: "uint64",
			args: []string{"-large", "18446744073709551615"},
			check: func(t *testing.T, c cfg) {
				if c.Large != 18446744073709551615 {
					t.Errorf("Large = %d, want max uint64", c.Large)
				}
			},
		},
		{
			name: "float64",
			args: []string{"-ratio", "3.14"},
			check: func(t *testing.T, c cfg) {
				if c.Ratio != 3.14 {
					t.Errorf("Ratio = %f, want 3.14", c.Ratio)
				}
			},
		},
		{
			name: "defaults when all absent",
			args: nil,
			check: func(t *testing.T, c cfg) {
				if c.Count != 0 || c.Big != 0 || c.Size != 0 || c.Large != 0 || c.Ratio != 0 {
					t.Errorf("expected all zero defaults, got %+v", c)
				}
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			tt.check(t, c)
		})
	}
}

func Test_conflagure_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Target string `flag:"target" help:"target" required:"true"`
	}

	t.Run("missing required field returns error", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for missing required flag, got nil")
		}
	})

	t.Run("required field satisfied", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-target", "prod"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Target != "prod" {
			t.Errorf("Target = %q, want %q", c.Target, "prod")
		}
	})
}

func Test_conflagure_skips_untagged_fields(t *testing.T) {
	t.Parallel()

	type cfg struct {
		internal string
		Name     string `default:"default" flag:"name"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"-name", "test"}); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if c.Name != "test" {
		t.Errorf("Name = %q, want %q", c.Name, "test")
	}
	if c.internal != "" {
		t.Errorf("internal = %q, want empty", c.internal)
	}
}

func Test_conflagure_non_struct_pointer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{name: "string pointer", cfg: new(string)},
		{name: "nil", cfg: nil},
		{name: "struct value not pointer", cfg: struct{ Name string }{Name: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := conflagure(newTestFS(), tt.cfg, nil); err == nil {
				t.Fatal("expected error for non-struct-pointer cfg, got nil")
			}
		})
	}
}

func Test_conflagure_unsupported_type(t *testing.T) {
	t.Parallel()

	// chan is not a supported flag type.
	type cfg struct {
		Ch chan int `flag:"ch"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, nil); err == nil {
		t.Fatal("expected error for unsupported field type, got nil")
	}
}

func Test_conflagure_unsupported_slice_element_type(t *testing.T) {
	t.Parallel()

	// []struct{} has an unsupported element type.
	type cfg struct {
		Items []struct{ X int } `flag:"items"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, nil); err == nil {
		t.Fatal("expected error for unsupported slice element type, got nil")
	}
}

func Test_conflagure_inverted_bool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    bool
		wantErr bool
	}{
		{name: "default true when absent", args: nil, want: true},
		{name: "no-verbose disables field", args: []string{"-no-verbose"}, want: false},
		{name: "no-v short alias disables field", args: []string{"-no-v"}, want: false},
		{name: "explicit no-verbose=true disables field", args: []string{"-no-verbose=true"}, want: false},
		{name: "no-verbose=false leaves field true", args: []string{"-no-verbose=false"}, want: true},
	}

	type cfg struct {
		Verbose bool `default:"true" flag:"verbose" help:"verbose output" short:"v"`
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("conflagure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if c.Verbose != tt.want {
				t.Errorf("Verbose = %v, want %v", c.Verbose, tt.want)
			}
		})
	}
}

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
			// Call registerFlag directly to isolate the short validation
			// without needing real struct tags.
			var s string
			err := registerFlag(newTestFS(), &s, "name", tt.short, "", "", "help")
			if (err != nil) != tt.wantErr {
				t.Errorf("registerFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_conflagure_anonymous_embedded_struct(t *testing.T) {
	t.Parallel()

	type base struct {
		Verbose bool   `flag:"verbose" help:"verbose output" short:"v"`
		Format  string `default:"text" flag:"format"         help:"output format"`
	}
	type cfg struct {
		base

		Name string `flag:"name" help:"name"`
	}

	tests := []struct {
		name    string
		args    []string
		verbose bool
		format  string
		cfgName string
	}{
		{
			name:    "embedded bool via long flag",
			args:    []string{"-verbose"},
			verbose: true, format: "text",
		},
		{
			name:    "embedded bool via short alias",
			args:    []string{"-v"},
			verbose: true, format: "text",
		},
		{
			name:   "embedded string with non-default value",
			args:   []string{"-format", "json"},
			format: "json",
		},
		{
			name:    "outer field alongside embedded fields",
			args:    []string{"-verbose", "-name", "alice"},
			verbose: true, format: "text", cfgName: "alice",
		},
		{
			name:   "defaults when no flags given",
			args:   nil,
			format: "text",
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if c.Verbose != tt.verbose {
				t.Errorf("Verbose = %v, want %v", c.Verbose, tt.verbose)
			}
			if c.Format != tt.format {
				t.Errorf("Format = %q, want %q", c.Format, tt.format)
			}
			if c.Name != tt.cfgName {
				t.Errorf("Name = %q, want %q", c.Name, tt.cfgName)
			}
		})
	}
}

func Test_conflagure_conflaguration_embedding(t *testing.T) {
	t.Parallel()

	type cfg struct {
		BaseTest

		Target string `flag:"target" help:"target"`
	}

	tests := []struct {
		name   string
		args   []string
		dryRun bool
		target string
	}{
		{name: "dry-run long flag sets DryRun", args: []string{"-dry-run"}, dryRun: true},
		{name: "dry-run short flag -n sets DryRun", args: []string{"-n"}, dryRun: true},
		{name: "outer and embedded flags together", args: []string{"-n", "-target", "prod"}, dryRun: true, target: "prod"},
		{name: "defaults when no flags given", args: nil},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if c.DryRun != tt.dryRun {
				t.Errorf("DryRun = %v, want %v", c.DryRun, tt.dryRun)
			}
			if c.Target != tt.target {
				t.Errorf("Target = %q, want %q", c.Target, tt.target)
			}
		})
	}
}

func Test_conflagure_named_struct_field(t *testing.T) {
	t.Parallel()

	// Named (non-anonymous) struct fields are recursed into, so flags can be
	// organised into sub-structs without embedding.
	type db struct {
		Host string `default:"localhost" flag:"db-host" help:"database host"`
		Port int    `default:"5432"      flag:"db-port" help:"database port"`
	}
	type cfg struct {
		DB   db
		Name string `flag:"name" help:"name"`
	}

	tests := []struct {
		name    string
		args    []string
		cfgName string
		host    string
		port    int
	}{
		{
			name: "named sub-struct fields are registered",
			args: []string{"-db-host", "prod.db", "-db-port", "5433"},
			host: "prod.db", port: 5433,
		},
		{
			name:    "outer and sub-struct fields together",
			args:    []string{"-name", "app", "-db-host", "staging.db"},
			cfgName: "app", host: "staging.db", port: 5432,
		},
		{
			name: "defaults when no flags given",
			args: nil,
			host: "localhost", port: 5432,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if c.Name != tt.cfgName {
				t.Errorf("Name = %q, want %q", c.Name, tt.cfgName)
			}
			if c.DB.Host != tt.host {
				t.Errorf("DB.Host = %q, want %q", c.DB.Host, tt.host)
			}
			if c.DB.Port != tt.port {
				t.Errorf("DB.Port = %d, want %d", c.DB.Port, tt.port)
			}
		})
	}
}

func Test_conflagure_named_struct_required(t *testing.T) {
	t.Parallel()

	type creds struct {
		Token string `flag:"token" help:"auth token" required:"true"`
	}
	type cfg struct {
		Auth creds
	}

	t.Run("required field inside named struct enforced", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for missing required flag -token, got nil")
		}
	})

	t.Run("required field inside named struct satisfied", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-token", "secret"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Auth.Token != "secret" {
			t.Errorf("Auth.Token = %q, want %q", c.Auth.Token, "secret")
		}
	})
}

// Test_conflagure_named_struct_no_tag_flat documents that named struct fields
// without a `flag:` tag on the field itself are traversed with a flat
// namespace: leaf flag names are unchanged and the Go field name is ignored.
func Test_conflagure_named_struct_no_tag_flat(t *testing.T) {
	t.Parallel()

	type network struct {
		Host string `default:"localhost" flag:"host" help:"server host"`
		Port int    `default:"8080"      flag:"port" help:"server port"`
	}
	// Server has no `flag:` tag → flat namespace; flags are -host and -port.
	type cfg struct {
		BaseTest

		Server network
	}

	t.Run("no flag tag on struct field means flat namespace", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-host", "example.com", "-port", "9090"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Server.Host != "example.com" {
			t.Errorf("Server.Host = %q, want %q", c.Server.Host, "example.com")
		}
		if c.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want %d", c.Server.Port, 9090)
		}
	})

	t.Run("defaults apply when no flags given", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Server.Host != "localhost" {
			t.Errorf("Server.Host = %q, want %q", c.Server.Host, "localhost")
		}
		if c.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want %d", c.Server.Port, 8080)
		}
	})
}

// Test_conflagure_named_struct_namespace documents that a named struct field
// with a `flag:"ns"` tag causes all leaf flags inside to be registered as
// "-ns.<leaf>", e.g. flag:"srv" + leaf flag:"host" → "-srv.host".
func Test_conflagure_named_struct_namespace(t *testing.T) {
	t.Parallel()

	type network struct {
		Host string `default:"localhost" flag:"host" help:"server host"`
		Port int    `default:"8080"      flag:"port" help:"server port"`
	}
	type cfg struct {
		BaseTest

		Server network `flag:"srv"`
		Label  string  `flag:"label" help:"top-level flag unchanged by namespace"`
	}

	t.Run("leaf flags are prefixed with namespace", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-srv.host", "example.com", "-srv.port", "9090"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Server.Host != "example.com" {
			t.Errorf("Server.Host = %q, want %q", c.Server.Host, "example.com")
		}
		if c.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want %d", c.Server.Port, 9090)
		}
	})

	t.Run("defaults apply without flags", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Server.Host != "localhost" {
			t.Errorf("Server.Host = %q, want %q", c.Server.Host, "localhost")
		}
		if c.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want %d", c.Server.Port, 8080)
		}
	})

	t.Run("namespace and top-level flags coexist", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-dry-run", "-srv.host", "staging.internal", "-label", "staging"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !c.DryRun {
			t.Error("DryRun = false, want true")
		}
		if c.Server.Host != "staging.internal" {
			t.Errorf("Server.Host = %q, want %q", c.Server.Host, "staging.internal")
		}
		if c.Label != "staging" {
			t.Errorf("Label = %q, want %q", c.Label, "staging")
		}
	})

	t.Run("required field inside namespace reports full flag name", func(t *testing.T) {
		type dbCreds struct {
			Token string `flag:"token" help:"auth token" required:"true"`
		}
		type cfg2 struct {
			DB dbCreds `flag:"db"`
		}
		var c cfg2
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for missing required flag, got nil")
		}
		const want = "-db.token"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	})

	t.Run("nested namespaces accumulate with dot separator", func(t *testing.T) {
		type leaf struct {
			Key string `default:"val" flag:"key" help:"leaf key"`
		}
		type middle struct {
			Inner leaf `flag:"inner"`
		}
		type cfg3 struct {
			Outer middle `flag:"outer"`
		}
		var c cfg3
		if err := conflagure(newTestFS(), &c, []string{"-outer.inner.key", "hello"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Outer.Inner.Key != "hello" {
			t.Errorf("Outer.Inner.Key = %q, want %q", c.Outer.Inner.Key, "hello")
		}
	})
}

// Test_Parse exercises the public Parse function. Must not run in parallel
// because it mutates flag.CommandLine and os.Args.
func Test_Parse(t *testing.T) {
	// Not parallel: modifies flag.CommandLine.
	// Pre-parse with nil so Parsed()=true; commandLineArgs() then returns
	// flag.CommandLine.Args() (nil) rather than falling back to os.Args[1:].
	type cfg struct {
		Env string `default:"dev" flag:"env" help:"environment"`
	}

	oldFS := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = oldFS })
	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
	if err := flag.CommandLine.Parse(nil); err != nil {
		t.Fatalf("pre-parse: %v", err)
	}

	var c cfg
	if err := Parse(&c); err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if c.Env != "dev" {
		t.Errorf("Env = %q, want default %q", c.Env, "dev")
	}
}

func Test_ParseCommand(t *testing.T) {
	// Not parallel: modifies flag.CommandLine.
	// Pre-seed CommandLine.Args() by parsing the subcommand name as a
	// positional arg; flag.Parse stops at the first non-'-' token so the
	// entire remaining slice stays in Args().
	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	oldFS := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = oldFS })

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	if err := flag.CommandLine.Parse([]string{"deploy", "-target=staging"}); err != nil {
		t.Fatalf("pre-parse error: %v", err)
	}

	var c cfg
	cmd, err := ParseCommand(&c)
	if err != nil {
		t.Fatalf("ParseCommand() error = %v", err)
	}
	if cmd == nil {
		t.Fatal("ParseCommand() returned nil Commander")
	}
	if _, ok := cmd.(*testDeployCmd); !ok {
		t.Errorf("cmd type = %T, want *testDeployCmd", cmd)
	}
	if c.Deploy.Target != "staging" {
		t.Errorf("Target = %q, want %q", c.Deploy.Target, "staging")
	}
}

func Test_Dispatch(t *testing.T) {
	// Not parallel: modifies flag.CommandLine.
	// Dispatch is full two-phase: pre-seed CommandLine so commandLineArgs()
	// returns the subcommand args, then call Dispatch directly.
	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	oldFS := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = oldFS })

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	if err := flag.CommandLine.Parse([]string{"deploy", "-target=prod"}); err != nil {
		t.Fatalf("pre-parse error: %v", err)
	}

	var c cfg
	result, err := Dispatch(&c)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result == nil {
		t.Fatal("Dispatch() returned nil")
	}
	if _, ok := result.(*testDeployCmd); !ok {
		t.Errorf("result type = %T, want *testDeployCmd", result)
	}
	if c.Deploy.Target != "prod" {
		t.Errorf("Target = %q, want %q", c.Deploy.Target, "prod")
	}
}

func Test_callWhen(t *testing.T) {
	t.Parallel()

	t.Run("true condition calls op", func(t *testing.T) {
		called := false
		callWhen(true, func(_ int) { called = true }, 0)
		if !called {
			t.Error("callWhen(true) did not invoke op")
		}
	})

	t.Run("false condition skips op", func(t *testing.T) {
		called := false
		callWhen(false, func(_ int) { called = true }, 0)
		if called {
			t.Error("callWhen(false) unexpectedly invoked op")
		}
	})
}

func Test_conflagure_inverted_bool_invalid_value(t *testing.T) {
	t.Parallel()

	// Passing a non-bool string to a BoolFunc flag exercises the
	// strconv.ParseBool error return inside the registered closure.
	type cfg struct {
		Verbose bool `default:"true" flag:"verbose" short:"v"`
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid value via long flag", args: []string{"-no-verbose=notabool"}},
		{name: "invalid value via short alias", args: []string{"-no-v=notabool"}},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err == nil {
				t.Fatal("expected error for invalid bool value, got nil")
			}
		})
	}
}

func Test_conflagure_unexported_with_tag(t *testing.T) {
	t.Parallel()

	// An unexported field that carries a flag: tag must return an error from bindFlags.
	type cfg struct {
		//nolint:unused
		internal string `flag:"secret"` //nolint:structcheck
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, nil); err == nil {
		t.Fatal("expected error for unexported field with flag tag, got nil")
	}
}

func Test_conflagure_unexported_nested_propagates_error(t *testing.T) {
	t.Parallel()

	// An unexported field inside a nested struct must propagate the error through
	// the recursive bindFlags call (exercises the `return err` in the struct branch).
	type inner struct {
		//nolint:unused
		secret string `flag:"token"` //nolint:structcheck
	}
	type outer struct {
		Inner inner
	}

	var c outer
	if err := conflagure(newTestFS(), &c, nil); err == nil {
		t.Fatal("expected error propagated from nested struct, got nil")
	}
}

func Test_conflagure_parse_failure(t *testing.T) {
	t.Parallel()

	// Passing an unknown flag causes fs.Parse to return an error (ContinueOnError mode).
	type cfg struct {
		Name string `flag:"name"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"-unknown-flag"}); err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}

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
			err := registerFlag(newTestFS(), tt.ptr(), "flag-name", "", tt.defStr, "", "help")
			if err == nil {
				t.Errorf("registerFlag() expected error for invalid default %q, got nil", tt.defStr)
			}
		})
	}
}

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
			err := registerFlag(fs, tt.ptr(), "long-flag", "x", "0", "", "help")
			if err != nil {
				t.Fatalf("registerFlag() unexpected error: %v", err)
			}
			if f := fs.Lookup("x"); f == nil {
				t.Error("short alias 'x' was not registered")
			}
		})
	}
}

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
			check: func(t *testing.T, c cfg) {
				if c.Count != 99 {
					t.Errorf("Count = %d, want 99", c.Count)
				}
			},
		},
		{
			name: "int64 short alias",
			args: []string{"-b", "88"},
			check: func(t *testing.T, c cfg) {
				if c.Big != 88 {
					t.Errorf("Big = %d, want 88", c.Big)
				}
			},
		},
		{
			name: "uint short alias",
			args: []string{"-s", "77"},
			check: func(t *testing.T, c cfg) {
				if c.Size != 77 {
					t.Errorf("Size = %d, want 77", c.Size)
				}
			},
		},
		{
			name: "uint64 short alias",
			args: []string{"-l", "66"},
			check: func(t *testing.T, c cfg) {
				if c.Large != 66 {
					t.Errorf("Large = %d, want 66", c.Large)
				}
			},
		},
		{
			name: "float64 short alias",
			args: []string{"-r", "2.71"},
			check: func(t *testing.T, c cfg) {
				if c.Ratio != 2.71 {
					t.Errorf("Ratio = %f, want 2.71", c.Ratio)
				}
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			tt.check(t, c)
		})
	}
}

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
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if !slices.Equal(c.Tags, tt.want) {
				t.Errorf("Tags = %v, want %v", c.Tags, tt.want)
			}
		})
	}
}

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
				if !slices.Equal(c.Ints, []int{1, 2, 3}) {
					t.Errorf("Ints = %v, want [1 2 3]", c.Ints)
				}
			},
		},
		{
			name: "[]int64 accumulates",
			args: []string{"-i64", "10", "-i64", "20"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.Int64s, []int64{10, 20}) {
					t.Errorf("Int64s = %v, want [10 20]", c.Int64s)
				}
			},
		},
		{
			name: "[]uint accumulates",
			args: []string{"-u", "5", "-u", "10"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.Uints, []uint{5, 10}) {
					t.Errorf("Uints = %v, want [5 10]", c.Uints)
				}
			},
		},
		{
			name: "[]uint64 accumulates",
			args: []string{"-u64", "100"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.Uint64s, []uint64{100}) {
					t.Errorf("Uint64s = %v, want [100]", c.Uint64s)
				}
			},
		},
		{
			name: "[]float64 accumulates",
			args: []string{"-f", "1.1", "-f", "2.2"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.Floats, []float64{1.1, 2.2}) {
					t.Errorf("Floats = %v, want [1.1 2.2]", c.Floats)
				}
			},
		},
		{
			name: "[]bool accumulates",
			args: []string{"-b", "true", "-b", "false", "-b", "true"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.Bools, []bool{true, false, true}) {
					t.Errorf("Bools = %v, want [true false true]", c.Bools)
				}
			},
		},
		{
			name: "no flags gives nil slices",
			args: nil,
			check: func(t *testing.T, c cfg) {
				if c.Ints != nil || c.Int64s != nil || c.Uints != nil ||
					c.Uint64s != nil || c.Floats != nil || c.Bools != nil {
					t.Errorf("expected all nil slices, got %+v", c)
				}
			},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			tt.check(t, c)
		})
	}
}

func Test_conflagure_slice_defaults(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags []string `default:"debug,info" flag:"tag"`
	}

	t.Run("default applied when no flags given", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"debug", "info"}) {
			t.Errorf("Tags = %v, want [debug info]", c.Tags)
		}
	})

	t.Run("default discarded when flag is used", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "warn"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"warn"}) {
			t.Errorf("Tags = %v, want [warn] (default should be cleared)", c.Tags)
		}
	})

	t.Run("default discarded when short alias is used first", func(t *testing.T) {
		type cfg2 struct {
			Tags []string `default:"debug,info" flag:"tag" short:"t"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-t", "warn", "-tag", "error"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"warn", "error"}) {
			t.Errorf("Tags = %v, want [warn error]", c.Tags)
		}
	})
}

func Test_conflagure_slice_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags []string `flag:"tag" required:"true"`
	}

	t.Run("missing required slice returns error", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for missing required slice flag, got nil")
		}
	})

	t.Run("required slice satisfied", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "foo"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"foo"}) {
			t.Errorf("Tags = %v, want [foo]", c.Tags)
		}
	})

	t.Run("required slice satisfied by default", func(t *testing.T) {
		type cfg2 struct {
			Tags []string `default:"fallback" flag:"tag" required:"true"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
	})
}

func Test_conflagure_slice_inverted_bool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Flags []bool `default:"true" flag:"flag" short:"f"`
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
			// bare -no-flag passes "true" to the handler → appends !true = false
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
			// bare -no-f is equivalent to -no-f=true
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
			if (err != nil) != tt.wantErr {
				t.Fatalf("conflagure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !slices.Equal(c.Flags, tt.want) {
				t.Errorf("Flags = %v, want %v", c.Flags, tt.want)
			}
		})
	}
}

func Test_conflagure_slice_namespace(t *testing.T) {
	t.Parallel()

	type filters struct {
		Tags []string `flag:"tag" help:"filter tags"`
	}
	type cfg struct {
		Filter filters `flag:"f"`
	}

	t.Run("slice in namespaced struct gets prefixed flag", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-f.tag", "foo", "-f.tag", "bar"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Filter.Tags, []string{"foo", "bar"}) {
			t.Errorf("Filter.Tags = %v, want [foo bar]", c.Filter.Tags)
		}
	})
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

	t.Run("slice in anonymous embedded struct works", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-label", "a", "-label", "b", "-name", "test"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Labels, []string{"a", "b"}) {
			t.Errorf("Labels = %v, want [a b]", c.Labels)
		}
		if c.Name != "test" {
			t.Errorf("Name = %q, want %q", c.Name, "test")
		}
	})
}

func Test_conflagure_slice_invalid_default(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  any
	}{
		{
			name: "[]int bad default",
			cfg: &struct {
				V []int `default:"1,bad" flag:"v"`
			}{},
		},
		{
			name: "[]int64 bad default",
			cfg: &struct {
				V []int64 `default:"1,bad" flag:"v"`
			}{},
		},
		{
			name: "[]uint bad default",
			cfg: &struct {
				V []uint `default:"1,bad" flag:"v"`
			}{},
		},
		{
			name: "[]uint64 bad default",
			cfg: &struct {
				V []uint64 `default:"1,bad" flag:"v"`
			}{},
		},
		{
			name: "[]float64 bad default",
			cfg: &struct {
				V []float64 `default:"1.0,notafloat" flag:"v"`
			}{},
		},
		{
			name: "[]bool bad default",
			cfg: &struct {
				V []bool `default:"true,notabool" flag:"v"`
			}{},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			if err := conflagure(newTestFS(), tt.cfg, nil); err == nil {
				t.Fatal("expected error for invalid slice default, got nil")
			}
		})
	}
}

func Test_conflagure_slice_numeric_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Ints    []int     `default:"0"     flag:"int"    short:"i"`
		Int64s  []int64   `default:"0"     flag:"int64"  short:"I"`
		Uints   []uint    `default:"0"     flag:"uint"   short:"u"`
		Uint64s []uint64  `default:"0"     flag:"uint64" short:"U"`
		Floats  []float64 `default:"0"     flag:"float"  short:"f"`
		Bools   []bool    `default:"false" flag:"bool"   short:"b"`
	}

	var c cfg
	args := []string{"-i", "1", "-I", "2", "-u", "3", "-U", "4", "-f", "1.5", "-b", "true"}
	if err := conflagure(newTestFS(), &c, args); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if !slices.Equal(c.Ints, []int{1}) {
		t.Errorf("Ints = %v, want [1]", c.Ints)
	}
	if !slices.Equal(c.Int64s, []int64{2}) {
		t.Errorf("Int64s = %v, want [2]", c.Int64s)
	}
	if !slices.Equal(c.Uints, []uint{3}) {
		t.Errorf("Uints = %v, want [3]", c.Uints)
	}
	if !slices.Equal(c.Uint64s, []uint64{4}) {
		t.Errorf("Uint64s = %v, want [4]", c.Uint64s)
	}
	if !slices.Equal(c.Floats, []float64{1.5}) {
		t.Errorf("Floats = %v, want [1.5]", c.Floats)
	}
	if !slices.Equal(c.Bools, []bool{true}) {
		t.Errorf("Bools = %v, want [true]", c.Bools)
	}
}

func Test_appendOnce(t *testing.T) {
	t.Parallel()

	t.Run("appends without clearing when no defaults", func(t *testing.T) {
		s := []int{9}
		fn := appendOnce(&s, strconv.Atoi, false)
		if err := fn("1"); err != nil {
			t.Fatal(err)
		}
		if err := fn("2"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int{9, 1, 2}) {
			t.Errorf("s = %v, want [9 1 2]", s)
		}
	})

	t.Run("clears pre-populated defaults on first call", func(t *testing.T) {
		s := []int{10, 20}
		fn := appendOnce(&s, strconv.Atoi, true)
		if err := fn("1"); err != nil {
			t.Fatal(err)
		}
		if err := fn("2"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int{1, 2}) {
			t.Errorf("s = %v, want [1 2] (defaults should be cleared)", s)
		}
	})

	t.Run("parse error propagates", func(t *testing.T) {
		var s []int
		fn := appendOnce(&s, strconv.Atoi, false)
		if err := fn("notanint"); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func Test_parseAndSetSliceDefault(t *testing.T) {
	t.Parallel()

	t.Run("empty defStr is a no-op", func(t *testing.T) {
		var s []string
		if err := parseAndSetSliceDefault(&s, "", func(v string) (string, error) { return v, nil }); err != nil {
			t.Fatal(err)
		}
		if s != nil {
			t.Errorf("s = %v, want nil", s)
		}
	})

	t.Run("comma-separated values are parsed and appended", func(t *testing.T) {
		var s []int
		if err := parseAndSetSliceDefault(&s, "1,2,3", strconv.Atoi); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int{1, 2, 3}) {
			t.Errorf("s = %v, want [1 2 3]", s)
		}
	})

	t.Run("whitespace around values is trimmed", func(t *testing.T) {
		var s []string
		if err := parseAndSetSliceDefault(&s, " a , b , c ", func(v string) (string, error) { return v, nil }); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a", "b", "c"}) {
			t.Errorf("s = %v, want [a b c]", s)
		}
	})

	t.Run("parse error is returned", func(t *testing.T) {
		var s []int
		if err := parseAndSetSliceDefault(&s, "1,bad,3", strconv.Atoi); err == nil {
			t.Fatal("expected error for unparseable element, got nil")
		}
	})
}

func Test_conflagure_time_Time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		At time.Time `flag:"at" help:"timestamp"`
	}

	t.Run("RFC3339 value", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-at", "2024-06-01T12:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		if !c.At.Equal(want) {
			t.Errorf("At = %v, want %v", c.At, want)
		}
	})

	t.Run("zero value when flag absent", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !c.At.IsZero() {
			t.Errorf("At = %v, want zero", c.At)
		}
	})

	t.Run("RFC3339 default applied", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `default:"2023-01-15T00:00:00Z" flag:"at"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		if !c.At.Equal(want) {
			t.Errorf("At = %v, want %v", c.At, want)
		}
	})

	t.Run("now default sets non-zero time", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `default:"now" flag:"at"`
		}
		before := time.Now().UTC().Add(-time.Second)
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !c.At.After(before) {
			t.Errorf("At = %v, expected after %v", c.At, before)
		}
	})

	t.Run("custom format tag", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Date time.Time `flag:"date" format:"2006-01-02"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-date", "2024-03-15"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		if !c.Date.Equal(want) {
			t.Errorf("Date = %v, want %v", c.Date, want)
		}
	})

	t.Run("short alias", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `flag:"at" short:"a"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-a", "2024-06-01T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.At.IsZero() {
			t.Error("At is zero, expected non-zero via short alias")
		}
	})

	t.Run("required field missing returns error", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for missing required time.Time, got nil")
		}
	})

	t.Run("required field satisfied", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-at", "2024-01-01T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-at", "not-a-time"}); err == nil {
			t.Fatal("expected error for invalid time value, got nil")
		}
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `default:"not-a-time" flag:"at"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid time default, got nil")
		}
	})

	t.Run("format and default mismatch returns error at bind time", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			At time.Time `default:"2024-01-01T00:00:00Z" flag:"at" format:"2006-01-02"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for layout/default mismatch, got nil")
		}
	})
}

func Test_conflagure_time_Duration(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Timeout time.Duration `flag:"timeout"`
	}

	t.Run("parses Go duration string", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "1h30m"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 90*time.Minute {
			t.Errorf("Timeout = %v, want 1h30m", c.Timeout)
		}
	})

	t.Run("zero when flag absent", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 0 {
			t.Errorf("Timeout = %v, want 0", c.Timeout)
		}
	})

	t.Run("default applied", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Timeout time.Duration `default:"5s" flag:"timeout"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want 5s", c.Timeout)
		}
	})

	t.Run("short alias", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Timeout time.Duration `flag:"timeout" short:"t"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-t", "500ms"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 500*time.Millisecond {
			t.Errorf("Timeout = %v, want 500ms", c.Timeout)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "notaduration"}); err == nil {
			t.Fatal("expected error for invalid duration, got nil")
		}
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Timeout time.Duration `default:"notaduration" flag:"timeout"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid duration default, got nil")
		}
	})
}

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
			t.Parallel()
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if !slices.Equal(c.Timeouts, tt.want) {
				t.Errorf("Timeouts = %v, want %v", c.Timeouts, tt.want)
			}
		})
	}
}

func Test_conflagure_slice_time_Duration_defaults(t *testing.T) {
	t.Parallel()

	t.Run("comma-separated default applied", func(t *testing.T) {
		t.Parallel()
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := []time.Duration{time.Second, 2 * time.Minute}
		if !slices.Equal(c.Timeouts, want) {
			t.Errorf("Timeouts = %v, want %v", c.Timeouts, want)
		}
	})

	t.Run("default discarded when flag used", func(t *testing.T) {
		t.Parallel()
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "5s"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Timeouts, []time.Duration{5 * time.Second}) {
			t.Errorf("Timeouts = %v, want [5s]", c.Timeouts)
		}
	})

	t.Run("invalid default returns error", func(t *testing.T) {
		t.Parallel()
		type cfg struct {
			Timeouts []time.Duration `default:"1s,notaduration" flag:"timeout"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid duration default, got nil")
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Parallel()
		type cfg struct {
			Timeouts []time.Duration `flag:"timeout"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "notaduration"}); err == nil {
			t.Fatal("expected error for invalid duration value, got nil")
		}
	})
}

func Test_conflagure_slice_time_Time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Dates []time.Time `flag:"date" help:"dates" short:"d"`
	}

	t.Run("single RFC3339 value", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-date", "2024-06-01T12:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		if len(c.Dates) != 1 || !c.Dates[0].Equal(want) {
			t.Errorf("Dates = %v, want [%v]", c.Dates, want)
		}
	})

	t.Run("multiple values accumulate", func(t *testing.T) {
		t.Parallel()
		var c cfg
		args := []string{"-date", "2024-01-01T00:00:00Z", "-date", "2024-06-01T00:00:00Z"}
		if err := conflagure(newTestFS(), &c, args); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if len(c.Dates) != 2 {
			t.Fatalf("len(Dates) = %d, want 2", len(c.Dates))
		}
	})

	t.Run("short alias", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-d", "2024-03-15T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if len(c.Dates) != 1 {
			t.Errorf("len(Dates) = %d, want 1", len(c.Dates))
		}
	})

	t.Run("no flags gives nil", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Dates != nil {
			t.Errorf("Dates = %v, want nil", c.Dates)
		}
	})

	t.Run("now in default resolves to non-zero time", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Dates []time.Time `default:"now" flag:"date"`
		}
		before := time.Now().UTC().Add(-time.Second)
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if len(c.Dates) != 1 || !c.Dates[0].After(before) {
			t.Errorf("Dates[0] = %v, expected after %v", c.Dates[0], before)
		}
	})

	t.Run("custom format tag", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Dates []time.Time `flag:"date" format:"2006-01-02"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-date", "2024-03-15"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		if len(c.Dates) != 1 || !c.Dates[0].Equal(want) {
			t.Errorf("Dates[0] = %v, want %v", c.Dates[0], want)
		}
	})

	t.Run("default discarded when flag used", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Dates []time.Time `default:"2023-01-01T00:00:00Z" flag:"date"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-date", "2024-06-01T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		if len(c.Dates) != 1 || !c.Dates[0].Equal(want) {
			t.Errorf("Dates = %v, want [%v] (default should be cleared)", c.Dates, want)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Parallel()
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-date", "not-a-time"}); err == nil {
			t.Fatal("expected error for invalid time value, got nil")
		}
	})

	t.Run("invalid default returns error", func(t *testing.T) {
		t.Parallel()
		type cfg2 struct {
			Dates []time.Time `default:"not-a-time" flag:"date"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid time default, got nil")
		}
	})
}

func Test_parseTimeFunc(t *testing.T) {
	t.Parallel()

	t.Run("parses RFC3339", func(t *testing.T) {
		t.Parallel()
		parse := parseTimeFunc(time.RFC3339)
		got, err := parse("2024-06-01T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("now returns current time", func(t *testing.T) {
		t.Parallel()
		before := time.Now().UTC().Add(-time.Second)
		parse := parseTimeFunc(time.RFC3339)
		got, err := parse("now")
		if err != nil {
			t.Fatal(err)
		}
		if !got.After(before) {
			t.Errorf("got %v, expected after %v", got, before)
		}
	})

	t.Run("invalid string returns error", func(t *testing.T) {
		t.Parallel()
		parse := parseTimeFunc(time.RFC3339)
		if _, err := parse("not-a-time"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("custom layout", func(t *testing.T) {
		t.Parallel()
		parse := parseTimeFunc("2006-01-02")
		got, err := parse("2024-03-15")
		if err != nil {
			t.Fatal(err)
		}
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func Test_conflagure_short_help_text(t *testing.T) {
	t.Parallel()

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
			if err := registerFlag(fs, &s, tt.flagName, tt.shortName, "", "", "long help text"); err != nil {
				t.Fatalf("registerFlag() error = %v", err)
			}
			f := fs.Lookup(tt.shortName)
			if f == nil {
				t.Fatalf("short flag %q not registered", tt.shortName)
			}
			if f.Usage != tt.wantUsage {
				t.Errorf("short flag usage = %q, want %q", f.Usage, tt.wantUsage)
			}
		})
	}
}

// ── subcommand test helpers ───────────────────────────────────────────────────

type testDeployCmd struct {
	Target    string `flag:"target"  help:"deployment target" required:"true"`
	DryRun    bool   `flag:"dry-run" short:"n"`
	RunCalled bool
}

func (d *testDeployCmd) Run(_ context.Context) error { d.RunCalled = true; return nil }

type testRollbackCmd struct {
	Version   string `flag:"version" required:"true"`
	Force     bool   `flag:"force"`
	RunCalled bool
}

func (r *testRollbackCmd) Run(_ context.Context) error { r.RunCalled = true; return nil }

// ── conflagureCmd (ParseCommand internals) ────────────────────────────────────

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
		wantCmd    string // "deploy" or "rollback"
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
			t.Parallel()
			var c cfg
			cmd, err := conflagureCmd(newTestFS(), &c, tt.args)
			if err != nil {
				t.Fatalf("conflagureCmd() error = %v", err)
			}
			if cmd == nil {
				t.Fatal("expected non-nil Commander")
			}
			switch tt.wantCmd {
			case "deploy":
				if _, ok := cmd.(*testDeployCmd); !ok {
					t.Errorf("cmd type = %T, want *testDeployCmd", cmd)
				}
				if c.Deploy.Target != tt.wantTarget {
					t.Errorf("Target = %q, want %q", c.Deploy.Target, tt.wantTarget)
				}
			case "rollback":
				if _, ok := cmd.(*testRollbackCmd); !ok {
					t.Errorf("cmd type = %T, want *testRollbackCmd", cmd)
				}
				if c.Rollback.Force != tt.wantForce {
					t.Errorf("Force = %v, want %v", c.Rollback.Force, tt.wantForce)
				}
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
	if !isErr(err, ers.ErrNotFound) {
		t.Errorf("expected ErrNotFound when no subcommand selected, got %v", err)
	}
	if !c.Verbose {
		t.Error("Verbose not set")
	}
}

// testCmdWithNestedCmd is a Commander whose struct body contains a cmd: field,
// which triggers the nested-cmd error when bindFlags processes it at depth=1.
type testCmdWithNestedCmd struct {
	Target string        `flag:"target"`
	Sub    testDeployCmd `cmd:"sub"`
}

func (t *testCmdWithNestedCmd) Run(_ context.Context) error { return nil }

// testRootCfg is a package-level type that itself implements Commander,
// used to verify that the root struct is returned when no subcommand is selected.
type testRootCfg struct {
	Verbose   bool          `flag:"verbose"`
	Deploy    testDeployCmd `cmd:"deploy"`
	RunCalled bool
}

func (r *testRootCfg) Run(_ context.Context) error { r.RunCalled = true; return nil }

func Test_conflagureCmd_root_commander(t *testing.T) {
	t.Parallel()

	var c testRootCfg
	cmd, err := conflagureCmd(newTestFS(), &c, []string{"-verbose"})
	if err != nil {
		t.Fatalf("conflagureCmd() error = %v", err)
	}
	// Root implements Commander; should be returned when no subcommand selected.
	if cmd == nil {
		t.Fatal("expected non-nil Commander (root)")
	}
	if _, ok := cmd.(*testRootCfg); !ok {
		t.Errorf("cmd type = %T, want *testRootCfg", cmd)
	}
	if !c.Verbose {
		t.Error("Verbose not set")
	}
}

func Test_conflagureCmd_required_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" required:"true"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for missing required subcommand")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

func Test_conflagureCmd_unknown_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

func Test_conflagureCmd_subcommand_required_flag_missing(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	// deploy requires -target; omitting it should error.
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"deploy"})
	if err == nil {
		t.Fatal("expected error for missing required subcommand flag")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

func Test_conflagureCmd_subcommand_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, []string{"deploy", "-target=prod", "-n"})
	if err != nil {
		t.Fatalf("conflagureCmd() error = %v", err)
	}
	if !c.Deploy.DryRun {
		t.Error("DryRun not set via short alias -n")
	}
}

func Test_conflagureCmd_nil_conf(t *testing.T) {
	t.Parallel()
	_, err := conflagureCmd(newTestFS(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil conf")
	}
}

func Test_conflagureCmd_non_pointer_conf(t *testing.T) {
	t.Parallel()
	type cfg struct{ X string }
	_, err := conflagureCmd(newTestFS(), cfg{}, nil)
	if err == nil {
		t.Fatal("expected error for non-pointer conf")
	}
}

// ── collectSubcommands error paths ────────────────────────────────────────────

func Test_collectSubcommands_both_flag_and_cmd(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	if err == nil {
		t.Fatal("expected error for field with both flag: and cmd: tags")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

func Test_collectSubcommands_unexported_field(t *testing.T) {
	t.Parallel()

	type cfg struct {
		deploy testDeployCmd `cmd:"deploy"` //nolint:unused
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	if err == nil {
		t.Fatal("expected error for unexported cmd: field")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

func Test_collectSubcommands_not_struct(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy string `cmd:"deploy"`
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	if err == nil {
		t.Fatal("expected error for non-struct cmd: field")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

func Test_collectSubcommands_no_commander(t *testing.T) {
	t.Parallel()

	// collectSubcommands no longer requires Commander; non-Commander structs
	// are accepted. validateCommanders / conflagureCmd enforce the requirement.
	type notCmd struct {
		Target string `flag:"target"`
	}
	type cfg struct {
		Deploy notCmd `cmd:"deploy"`
	}
	var c cfg
	entries, err := collectSubcommands(reflectVal(&c), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func Test_conflagureCmd_no_commander(t *testing.T) {
	t.Parallel()

	// conflagureCmd must error when a cmd: field does not implement Commander.
	type notCmd struct {
		Target string `flag:"target"`
	}
	type cfg struct {
		Deploy notCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for type not implementing Commander")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// ── nested cmd: tag detection ─────────────────────────────────────────────────

func Test_bindFlags_nested_cmd_tag_errors(t *testing.T) {
	t.Parallel()

	// A subcommand struct that itself contains a cmd: field.
	type innerSub struct {
		testDeployCmd

		Nested testRollbackCmd `cmd:"nested"`
	}
	// innerSub must implement Commander for collectSubcommands to accept it.
	// We can't add methods to local types, so define it at package level.
	// Instead, confirm the error is triggered from bindFlags when depth=1.
	fs := newTestFS()
	type badInner struct {
		Target string        `flag:"target"`
		Sub    testDeployCmd `cmd:"sub"`
	}
	var b badInner
	err := bindFlags(fs, reflectVal(&b), "", 1)
	if err == nil {
		t.Fatal("expected error for nested cmd: tag at depth > 0")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// ── dispatch ──────────────────────────────────────────────────────────────────

func Test_dispatch_nil_conf(t *testing.T) {
	t.Parallel()
	_, err := dispatch(newTestFS(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func Test_dispatch_non_pointer(t *testing.T) {
	t.Parallel()
	type cfg struct{ X string }
	_, err := dispatch(newTestFS(), cfg{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func Test_dispatch_no_subcommands(t *testing.T) {
	t.Parallel()
	type cfg struct {
		X string `flag:"x"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, nil)
	if !isErr(err, ers.ErrNotFound) {
		t.Errorf("expected ErrNotFound when no cmd: fields, got %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ── additional coverage tests ─────────────────────────────────────────────────

// Test_conflagureCmd_usage_closure_triggered passes an unknown global flag so
// that fs.Parse calls our custom Usage closure (without os.Exit since it is not
// a -help flag). Exercises both the "has help" and "no help" branches.
func Test_conflagureCmd_usage_closure_triggered(t *testing.T) {
	t.Parallel()

	type cfg struct {
		// Deploy has a help tag; Rollback does not — both loop branches are hit.
		Deploy   testDeployCmd   `cmd:"deploy"   help:"deploy to target"`
		Rollback testRollbackCmd `cmd:"rollback"`
	}

	fs := newTestFS()
	fs.SetOutput(io.Discard) // suppress output in test logs
	var c cfg
	_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
	if err == nil {
		t.Fatal("expected parse error for unknown flag")
	}
}

// Test_conflagureCmd_usage_nil_origUsage covers the else branch in the closure
// where origUsage is nil (explicitly zeroed before conflagureCmd runs).
func Test_conflagureCmd_usage_nil_origUsage(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" help:"deploy"`
	}

	fs := newTestFS()
	fs.Usage = nil // explicitly nil so origUsage is nil inside the closure
	fs.SetOutput(io.Discard)
	var c cfg
	_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
	if err == nil {
		t.Fatal("expected parse error for unknown flag")
	}
}

// Test_conflagureCmd_usage_with_preset_origUsage verifies that a pre-existing
// Usage function is called inside our closure (origUsage != nil branch).
func Test_conflagureCmd_usage_with_preset_origUsage(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" help:"deploy"`
	}

	fs := newTestFS()
	fs.SetOutput(io.Discard)
	customUsageCalled := false
	fs.Usage = func() { customUsageCalled = true }

	var c cfg
	_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !customUsageCalled {
		t.Error("expected original Usage func to be called")
	}
}

// Test_conflagureCmd_global_required_error checks that a required global flag
// (not a subcommand flag) triggers ErrInvalidInput.
func Test_conflagureCmd_global_required_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Username string        `flag:"username" required:"true"`
		Deploy   testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for missing required global flag")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

// Test_conflagureCmd_collectSubcommands_error triggers a collectSubcommands
// error (flag: and cmd: on the same field) from conflagureCmd.
func Test_conflagureCmd_collectSubcommands_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for field with both flag: and cmd: tags")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// Test_conflagureCmd_bindFlags_error triggers bindFlags failure for a global
// field (invalid short flag) from conflagureCmd.
func Test_conflagureCmd_bindFlags_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool          `flag:"verbose" short:"ab"` // short must be 1 char
		Deploy  testDeployCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for invalid short flag")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// Test_conflagureCmd_subcommand_parse_error passes an unknown flag to the
// subcommand FlagSet, covering the matched.fs.Parse error path in dispatchEntries.
func Test_conflagureCmd_subcommand_parse_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	fs := newTestFS()
	fs.SetOutput(io.Discard)
	var c cfg
	_, err := conflagureCmd(fs, &c, []string{"deploy", "-unknown-subcmd-flag"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand flag")
	}
}

// Test_dispatch_collectSubcommands_error exercises the collectSubcommands error
// path inside dispatch directly.
func Test_dispatch_collectSubcommands_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy" flag:"deploy"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// Test_collectSubcommands_bindflags_error exercises the path where bindFlags
// errors while processing a subcommand struct (nested cmd: at depth=1).
func Test_collectSubcommands_bindflags_error(t *testing.T) {
	t.Parallel()

	type cfg struct {
		// testCmdWithNestedCmd implements Commander but has a cmd: field inside,
		// causing bindFlags to error when called at depth=1.
		Deploy testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	if err == nil {
		t.Fatal("expected error for nested cmd: inside subcommand struct")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("error = %v, want ErrInvalidSpecification", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ── commandLineArgs ───────────────────────────────────────────────────────────

func Test_commandLineArgs(t *testing.T) {
	// Not parallel: modifies flag.CommandLine.
	oldFS := flag.CommandLine
	t.Cleanup(func() { flag.CommandLine = oldFS })

	t.Run("not yet parsed falls back to os.Args", func(t *testing.T) {
		flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
		if got, want := commandLineArgs(), os.Args[1:]; !slices.Equal(got, want) {
			t.Errorf("commandLineArgs() = %v, want %v", got, want)
		}
	})

	t.Run("already parsed returns CommandLine.Args()", func(t *testing.T) {
		flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
		flag.CommandLine.Parse([]string{"remaining"})
		if got, want := commandLineArgs(), []string{"remaining"}; !slices.Equal(got, want) {
			t.Errorf("commandLineArgs() = %v, want %v", got, want)
		}
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// reflectVal returns the reflect.Value of the struct pointed to by ptr.
func reflectVal(ptr any) reflect.Value {
	return reflect.ValueOf(ptr).Elem()
}

// isErr reports whether err wraps target using errors.Is.
func isErr(err, target error) bool {
	return errors.Is(err, target)
}
