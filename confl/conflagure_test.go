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
			err := registerFlag(newTestFS(), &s, flagSpec{Name: "name", Short: tt.short, Help: "help"})
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
			err := registerFlag(newTestFS(), tt.ptr(), flagSpec{Name: "flag-name", Default: tt.defStr, Help: "help"})
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
			err := registerFlag(fs, tt.ptr(), flagSpec{Name: "long-flag", Short: "x", Default: "0", Help: "help"})
			if err != nil {
				t.Fatalf("registerFlag() unexpected error: %v", err)
			}
			if f := fs.Lookup("x"); f == nil {
				t.Error("short alias 'x' was not registered")
			}
		})
	}
}

func Test_registerFlag_flag_value(t *testing.T) {
	t.Parallel()

	t.Run("basic registration and set via flag", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		if err := registerFlag(fs, v, flagSpec{Name: "custom", Help: "help"}); err != nil {
			t.Fatalf("registerFlag() error = %v", err)
		}
		if err := fs.Parse([]string{"-custom=hello"}); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if v.String() != "hello" {
			t.Errorf("got %q, want %q", v.String(), "hello")
		}
	})

	t.Run("default applied via Set", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		if err := registerFlag(fs, v, flagSpec{Name: "custom", Default: "defaultval", Help: "help"}); err != nil {
			t.Fatalf("registerFlag() error = %v", err)
		}
		// default is applied before fs.Parse, so vals should already contain it
		if len(v.vals) != 1 || v.vals[0] != "defaultval" {
			t.Errorf("vals after default = %v, want [defaultval]", v.vals)
		}
	})

	t.Run("short alias registered", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{}
		if err := registerFlag(fs, v, flagSpec{Name: "custom", Short: "c", Help: "help"}); err != nil {
			t.Fatalf("registerFlag() error = %v", err)
		}
		if fs.Lookup("c") == nil {
			t.Error("short alias 'c' was not registered")
		}
		if err := fs.Parse([]string{"-c=via-short"}); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if v.String() != "via-short" {
			t.Errorf("got %q, want %q", v.String(), "via-short")
		}
	})

	t.Run("bad default returns ErrInvalidSpecification", func(t *testing.T) {
		fs := newTestFS()
		v := &testFlagValue{setErr: errors.New("bad value")}
		err := registerFlag(fs, v, flagSpec{Name: "custom", Default: "baddefault", Help: "help"})
		if err == nil {
			t.Fatal("expected error for failing Set on default, got nil")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})
}

func Test_conflagure_flag_value(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Custom testFlagValue `default:"defval" flag:"custom" help:"custom flag"`
	}

	t.Run("default applied when no args", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Custom.String() != "defval" {
			t.Errorf("got %q, want %q", c.Custom.String(), "defval")
		}
	})

	t.Run("explicit value overrides default", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-custom=explicit"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Custom.String() != "explicit" {
			t.Errorf("got %q, want %q", c.Custom.String(), "explicit")
		}
	})

	t.Run("required field not set returns error", func(t *testing.T) {
		type requiredCfg struct {
			Custom testFlagValue `flag:"custom" required:"true"`
		}
		var c requiredCfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for required flag.Value field not set")
		}
		if !isErr(err, ErrInvalidInput) {
			t.Errorf("error = %v, want ErrInvalidInput", err)
		}
	})
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
		Tags []string `default:"debug,info" flag:"tag" sep:","`
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
			Tags []string `default:"debug,info" flag:"tag" sep:"," short:"t"`
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
			Tags []string `default:"fallback" flag:"tag" required:"true" sep:""`
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
				V []int `default:"1,bad" flag:"v" sep:","`
			}{},
		},
		{
			name: "[]int64 bad default",
			cfg: &struct {
				V []int64 `default:"1,bad" flag:"v" sep:","`
			}{},
		},
		{
			name: "[]uint bad default",
			cfg: &struct {
				V []uint `default:"1,bad" flag:"v" sep:","`
			}{},
		},
		{
			name: "[]uint64 bad default",
			cfg: &struct {
				V []uint64 `default:"1,bad" flag:"v" sep:","`
			}{},
		},
		{
			name: "[]float64 bad default",
			cfg: &struct {
				V []float64 `default:"1.0,notafloat" flag:"v" sep:","`
			}{},
		},
		{
			name: "[]bool bad default",
			cfg: &struct {
				V []bool `default:"true,notabool" flag:"v" sep:","`
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
		Ints    []int     `default:"0"     flag:"int"    sep:"" short:"i"`
		Int64s  []int64   `default:"0"     flag:"int64"  sep:"" short:"I"`
		Uints   []uint    `default:"0"     flag:"uint"   sep:"" short:"u"`
		Uint64s []uint64  `default:"0"     flag:"uint64" sep:"" short:"U"`
		Floats  []float64 `default:"0"     flag:"float"  sep:"" short:"f"`
		Bools   []bool    `default:"false" flag:"bool"   sep:"" short:"b"`
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
			check: func(t *testing.T, c cfg) {
				if c.I8 != 127 {
					t.Errorf("I8 = %d, want 127", c.I8)
				}
			},
		},
		{
			name: "int16",
			args: []string{"-i16", "32767"},
			check: func(t *testing.T, c cfg) {
				if c.I16 != 32767 {
					t.Errorf("I16 = %d, want 32767", c.I16)
				}
			},
		},
		{
			name: "int32",
			args: []string{"-i32", "2147483647"},
			check: func(t *testing.T, c cfg) {
				if c.I32 != 2147483647 {
					t.Errorf("I32 = %d, want 2147483647", c.I32)
				}
			},
		},
		{
			name: "uint8",
			args: []string{"-u8", "255"},
			check: func(t *testing.T, c cfg) {
				if c.U8 != 255 {
					t.Errorf("U8 = %d, want 255", c.U8)
				}
			},
		},
		{
			name: "uint16",
			args: []string{"-u16", "65535"},
			check: func(t *testing.T, c cfg) {
				if c.U16 != 65535 {
					t.Errorf("U16 = %d, want 65535", c.U16)
				}
			},
		},
		{
			name: "uint32",
			args: []string{"-u32", "4294967295"},
			check: func(t *testing.T, c cfg) {
				if c.U32 != 4294967295 {
					t.Errorf("U32 = %d, want 4294967295", c.U32)
				}
			},
		},
		{
			name: "float32",
			args: []string{"-f32", "1.5"},
			check: func(t *testing.T, c cfg) {
				if c.F32 != 1.5 {
					t.Errorf("F32 = %f, want 1.5", c.F32)
				}
			},
		},
		{
			name: "negative int8",
			args: []string{"-i8", "-128"},
			check: func(t *testing.T, c cfg) {
				if c.I8 != -128 {
					t.Errorf("I8 = %d, want -128", c.I8)
				}
			},
		},
		{
			name: "negative int32",
			args: []string{"-i32", "-2147483648"},
			check: func(t *testing.T, c cfg) {
				if c.I32 != -2147483648 {
					t.Errorf("I32 = %d, want -2147483648", c.I32)
				}
			},
		},
		{
			name: "defaults when all absent",
			args: nil,
			check: func(t *testing.T, c cfg) {
				if c.I8 != 0 || c.I16 != 0 || c.I32 != 0 ||
					c.U8 != 0 || c.U16 != 0 || c.U32 != 0 || c.F32 != 0 {
					t.Errorf("expected all zero, got %+v", c)
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
	if err := conflagure(newTestFS(), &c, nil); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if c.I8 != 10 {
		t.Errorf("I8 = %d, want 10", c.I8)
	}
	if c.I16 != 200 {
		t.Errorf("I16 = %d, want 200", c.I16)
	}
	if c.I32 != 300 {
		t.Errorf("I32 = %d, want 300", c.I32)
	}
	if c.U8 != 20 {
		t.Errorf("U8 = %d, want 20", c.U8)
	}
	if c.U16 != 400 {
		t.Errorf("U16 = %d, want 400", c.U16)
	}
	if c.U32 != 500 {
		t.Errorf("U32 = %d, want 500", c.U32)
	}
	if c.F32 != 2.5 {
		t.Errorf("F32 = %f, want 2.5", c.F32)
	}
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
			if err := conflagure(newTestFS(), tt.cfg, nil); err == nil {
				t.Fatal("expected error for invalid default, got nil")
			}
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
			if err := conflagure(newTestFS(), tt.cfg, tt.args); err == nil {
				t.Fatal("expected error for out-of-range value, got nil")
			}
		})
	}
}

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
				if !slices.Equal(c.I8s, []int8{1, 2, -3}) {
					t.Errorf("I8s = %v, want [1 2 -3]", c.I8s)
				}
			},
		},
		{
			name: "[]int16 accumulates",
			args: []string{"-i16", "100", "-i16", "-200"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.I16s, []int16{100, -200}) {
					t.Errorf("I16s = %v, want [100 -200]", c.I16s)
				}
			},
		},
		{
			name: "[]int32 accumulates",
			args: []string{"-i32", "1000", "-i32", "2000"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.I32s, []int32{1000, 2000}) {
					t.Errorf("I32s = %v, want [1000 2000]", c.I32s)
				}
			},
		},
		{
			name: "[]uint8 accumulates",
			args: []string{"-u8", "10", "-u8", "20"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.U8s, []uint8{10, 20}) {
					t.Errorf("U8s = %v, want [10 20]", c.U8s)
				}
			},
		},
		{
			name: "[]uint16 accumulates",
			args: []string{"-u16", "1000", "-u16", "2000"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.U16s, []uint16{1000, 2000}) {
					t.Errorf("U16s = %v, want [1000 2000]", c.U16s)
				}
			},
		},
		{
			name: "[]uint32 accumulates",
			args: []string{"-u32", "100000", "-u32", "200000"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.U32s, []uint32{100000, 200000}) {
					t.Errorf("U32s = %v, want [100000 200000]", c.U32s)
				}
			},
		},
		{
			name: "[]float32 accumulates",
			args: []string{"-f32", "1.5", "-f32", "2.5"},
			check: func(t *testing.T, c cfg) {
				if !slices.Equal(c.F32s, []float32{1.5, 2.5}) {
					t.Errorf("F32s = %v, want [1.5 2.5]", c.F32s)
				}
			},
		},
		{
			name: "no flags gives nil slices",
			args: nil,
			check: func(t *testing.T, c cfg) {
				if c.I8s != nil || c.I16s != nil || c.I32s != nil ||
					c.U8s != nil || c.U16s != nil || c.U32s != nil || c.F32s != nil {
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
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.I8s, []int8{1, 2}) {
			t.Errorf("I8s = %v, want [1 2]", c.I8s)
		}
		if !slices.Equal(c.F32s, []float32{1.5, 2.5}) {
			t.Errorf("F32s = %v, want [1.5 2.5]", c.F32s)
		}
	})

	t.Run("flag use discards defaults", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-i8", "99"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.I8s, []int8{99}) {
			t.Errorf("I8s = %v, want [99] (defaults cleared)", c.I8s)
		}
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
			if err := conflagure(newTestFS(), tt.cfg, nil); err == nil {
				t.Fatal("expected error for invalid slice default, got nil")
			}
		})
	}
}

func Test_parseTimeFuncAuto(t *testing.T) {
	t.Parallel()

	parse := parseTimeFuncAuto()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, time.Time)
	}{
		{
			name:  "RFC3339",
			input: "2024-03-15T10:30:00Z",
			check: func(t *testing.T, got time.Time) {
				if got.Year() != 2024 || got.Month() != 3 || got.Day() != 15 {
					t.Errorf("got %v, want 2024-03-15", got)
				}
			},
		},
		{
			name:  "DateTime",
			input: "2024-03-15 10:30:00",
			check: func(t *testing.T, got time.Time) {
				if got.Year() != 2024 || got.Month() != 3 || got.Day() != 15 {
					t.Errorf("got %v, want 2024-03-15", got)
				}
				if got.Hour() != 10 || got.Minute() != 30 {
					t.Errorf("got %v, want 10:30", got)
				}
			},
		},
		{
			name:  "DateOnly",
			input: "2024-03-15",
			check: func(t *testing.T, got time.Time) {
				if got.Year() != 2024 || got.Month() != 3 || got.Day() != 15 {
					t.Errorf("got %v, want 2024-03-15", got)
				}
				if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
					t.Errorf("expected midnight, got %v", got)
				}
			},
		},
		{
			name:  "now resolves",
			input: "now",
			check: func(t *testing.T, got time.Time) {
				if got.IsZero() {
					t.Error("now produced zero time")
				}
			},
		},
		{
			name:    "invalid returns error",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "partial date rejected",
			input:   "2024-03",
			wantErr: true,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTimeFuncAuto()(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func Test_appendOnce(t *testing.T) {
	t.Parallel()

	t.Run("appends without clearing when no defaults", func(t *testing.T) {
		s := []int{9}
		fn := appendOnce(&s, strconv.Atoi, false, "", false)
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
		fn := appendOnce(&s, strconv.Atoi, true, "", false)
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
		fn := appendOnce(&s, strconv.Atoi, false, "", false)
		if err := fn("notanint"); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func Test_parseAndSetSliceDefault(t *testing.T) {
	t.Parallel()

	t.Run("empty defStr is a no-op", func(t *testing.T) {
		var s []string
		if err := parseAndSetSliceDefault(&s, "", "", false, func(v string) (string, error) { return v, nil }); err != nil {
			t.Fatal(err)
		}
		if s != nil {
			t.Errorf("s = %v, want nil", s)
		}
	})

	t.Run("comma-separated values are parsed and appended", func(t *testing.T) {
		var s []int
		if err := parseAndSetSliceDefault(&s, "1,2,3", ",", true, strconv.Atoi); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []int{1, 2, 3}) {
			t.Errorf("s = %v, want [1 2 3]", s)
		}
	})

	t.Run("whitespace around values is trimmed", func(t *testing.T) {
		var s []string
		if err := parseAndSetSliceDefault(&s, " a , b , c ", ",", true, func(v string) (string, error) { return v, nil }); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a", "b", "c"}) {
			t.Errorf("s = %v, want [a b c]", s)
		}
	})

	t.Run("parse error is returned", func(t *testing.T) {
		var s []int
		if err := parseAndSetSliceDefault(&s, "1,bad,3", ",", true, strconv.Atoi); err == nil {
			t.Fatal("expected error for unparseable element, got nil")
		}
	})

	t.Run("no sep tag with non-empty default returns ErrInvalidSpecification", func(t *testing.T) {
		var s []string
		err := parseAndSetSliceDefault(&s, "a,b,c", "", false, func(v string) (string, error) { return v, nil })
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification for missing sep: tag, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})
}

// ── sep: tag tests ────────────────────────────────────────────────────────────

func Test_sep_tag_default_splitting(t *testing.T) {
	t.Parallel()

	t.Run("sep colon splits default into three elements", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b:c" flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"a", "b", "c"}) {
			t.Errorf("Tags = %v, want [a b c]", c.Tags)
		}
	})

	t.Run("sep pipe splits default", func(t *testing.T) {
		type cfg struct {
			Items []string `default:"x|y|z" flag:"item" sep:"|"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Items, []string{"x", "y", "z"}) {
			t.Errorf("Items = %v, want [x y z]", c.Items)
		}
	})

	t.Run("sep colon on int slice splits default", func(t *testing.T) {
		type cfg struct {
			Nums []int `default:"1:2:3" flag:"num" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Nums, []int{1, 2, 3}) {
			t.Errorf("Nums = %v, want [1 2 3]", c.Nums)
		}
	})

	t.Run("explicit sep comma behaves same as absent sep for defaults", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a,b,c" flag:"tag" sep:","`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"a", "b", "c"}) {
			t.Errorf("Tags = %v, want [a b c]", c.Tags)
		}
	})

	t.Run("sep empty string disables default splitting", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a,b,c" flag:"tag" sep:""`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		// sep:"" means no splitting: "a,b,c" is one element
		if !slices.Equal(c.Tags, []string{"a,b,c"}) {
			t.Errorf("Tags = %v, want [a,b,c]", c.Tags)
		}
	})

	t.Run("absent sep tag with non-empty default returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"debug,info,warn" flag:"tag"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification for missing sep: tag, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})
}

func Test_sep_tag_live_flag_splitting(t *testing.T) {
	t.Parallel()

	t.Run("sep colon splits single flag invocation into multiple elements", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "a:b:c"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"a", "b", "c"}) {
			t.Errorf("Tags = %v, want [a b c]", c.Tags)
		}
	})

	t.Run("multiple flag invocations each split and accumulate", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "a:b", "-tag", "c"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"a", "b", "c"}) {
			t.Errorf("Tags = %v, want [a b c]", c.Tags)
		}
	})

	t.Run("absent sep tag does not split live flag values", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "a:b:c"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		// Without sep: tag, "a:b:c" is a single element
		if !slices.Equal(c.Tags, []string{"a:b:c"}) {
			t.Errorf("Tags = %v, want [a:b:c]", c.Tags)
		}
	})

	t.Run("sep empty string does not split live flag values", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:""`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "a,b,c"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		// sep:"" → no splitting on live flags
		if !slices.Equal(c.Tags, []string{"a,b,c"}) {
			t.Errorf("Tags = %v, want [a,b,c]", c.Tags)
		}
	})

	t.Run("sep colon on int slice splits live flag value", func(t *testing.T) {
		type cfg struct {
			Nums []int `flag:"num" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-num", "10:20:30"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Nums, []int{10, 20, 30}) {
			t.Errorf("Nums = %v, want [10 20 30]", c.Nums)
		}
	})

	t.Run("sep colon with short alias also splits", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":" short:"t"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-t", "x:y"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"x", "y"}) {
			t.Errorf("Tags = %v, want [x y]", c.Tags)
		}
	})

	t.Run("sep colon parse error on bad element returns error", func(t *testing.T) {
		type cfg struct {
			Nums []int `flag:"num" sep:":"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, []string{"-num", "1:notanint:3"})
		if err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func Test_sep_tag_default_cleared_on_first_invocation(t *testing.T) {
	t.Parallel()

	t.Run("defaults are cleared when first flag invocation occurs", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-tag", "c:d"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		// defaults ["a","b"] replaced by ["c","d"]
		if !slices.Equal(c.Tags, []string{"c", "d"}) {
			t.Errorf("Tags = %v, want [c d]", c.Tags)
		}
	})

	t.Run("multiple invocations after clearing accumulate", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":" short:"t"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-t", "c:d", "-tag", "e"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		// First invocation clears defaults, splits "c:d" → c,d; second appends e
		if !slices.Equal(c.Tags, []string{"c", "d", "e"}) {
			t.Errorf("Tags = %v, want [c d e]", c.Tags)
		}
	})

	t.Run("no invocation leaves defaults intact", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !slices.Equal(c.Tags, []string{"a", "b"}) {
			t.Errorf("Tags = %v, want [a b]", c.Tags)
		}
	})
}

func Test_sep_tag_validation(t *testing.T) {
	t.Parallel()

	t.Run("sep on string scalar returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Name string `flag:"name" sep:":"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("sep on int scalar returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Count int `flag:"count" sep:":"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("sep on bool scalar returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Flag bool `flag:"flag" sep:":"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("sep on float64 scalar returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Ratio float64 `flag:"ratio" sep:":"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("sep on slice is valid", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Errorf("unexpected error for sep on slice: %v", err)
		}
	})
}

func Test_parseAndSetSliceDefault_sep(t *testing.T) {
	t.Parallel()

	t.Run("sepSet false returns ErrInvalidSpecification when defStr non-empty", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		err := parseAndSetSliceDefault(&s, "a,b,c", "", false, id)
		if err == nil {
			t.Fatal("expected ErrInvalidSpecification, got nil")
		}
		if !errors.Is(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("sepSet true sep colon uses colon", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		if err := parseAndSetSliceDefault(&s, "a:b:c", ":", true, id); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a", "b", "c"}) {
			t.Errorf("s = %v, want [a b c]", s)
		}
	})

	t.Run("sepSet true sep empty treats whole value as single element", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		if err := parseAndSetSliceDefault(&s, "a,b,c", "", true, id); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a,b,c"}) {
			t.Errorf("s = %v, want [a,b,c]", s)
		}
	})

	t.Run("whitespace trimmed with custom sep", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		if err := parseAndSetSliceDefault(&s, " x : y : z ", ":", true, id); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"x", "y", "z"}) {
			t.Errorf("s = %v, want [x y z]", s)
		}
	})
}

func Test_appendOnce_sep(t *testing.T) {
	t.Parallel()

	t.Run("sepSet false does not split on any character", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		fn := appendOnce(&s, id, false, "", false)
		if err := fn("a:b:c"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a:b:c"}) {
			t.Errorf("s = %v, want [a:b:c]", s)
		}
	})

	t.Run("sepSet true sep colon splits value", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		fn := appendOnce(&s, id, false, ":", true)
		if err := fn("a:b:c"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a", "b", "c"}) {
			t.Errorf("s = %v, want [a b c]", s)
		}
	})

	t.Run("sepSet true sep empty does not split", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		fn := appendOnce(&s, id, false, "", true)
		if err := fn("a:b:c"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a:b:c"}) {
			t.Errorf("s = %v, want [a:b:c]", s)
		}
	})

	t.Run("clearOnFirst still fires exactly once with sep", func(t *testing.T) {
		s := []string{"old"}
		id := func(v string) (string, error) { return v, nil }
		fn := appendOnce(&s, id, true, ":", true)
		if err := fn("x:y"); err != nil {
			t.Fatal(err)
		}
		// old cleared, x and y appended
		if !slices.Equal(s, []string{"x", "y"}) {
			t.Errorf("s = %v, want [x y]", s)
		}
		// second call appends without clearing again
		if err := fn("z"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"x", "y", "z"}) {
			t.Errorf("s = %v, want [x y z]", s)
		}
	})

	t.Run("parse error inside split propagates", func(t *testing.T) {
		var s []int
		fn := appendOnce(&s, strconv.Atoi, false, ":", true)
		if err := fn("1:notanint:3"); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})

	t.Run("whitespace trimmed per-element with sep", func(t *testing.T) {
		var s []string
		id := func(v string) (string, error) { return v, nil }
		fn := appendOnce(&s, id, false, ":", true)
		if err := fn(" a : b : c "); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s, []string{"a", "b", "c"}) {
			t.Errorf("s = %v, want [a b c]", s)
		}
	})
}

func Test_conflagure_time_Time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		At time.Time `flag:"at" help:"timestamp"`
	}

	t.Run("RFC3339 value", func(t *testing.T) {
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
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if !c.At.IsZero() {
			t.Errorf("At = %v, want zero", c.At)
		}
	})

	t.Run("RFC3339 default applied", func(t *testing.T) {
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
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for missing required time.Time, got nil")
		}
	})

	t.Run("required field satisfied", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, []string{"-at", "2024-01-01T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-at", "not-a-time"}); err == nil {
			t.Fatal("expected error for invalid time value, got nil")
		}
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `default:"not-a-time" flag:"at"`
		}
		var c cfg2
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid time default, got nil")
		}
	})

	t.Run("format and default mismatch returns error at bind time", func(t *testing.T) {
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
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "1h30m"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 90*time.Minute {
			t.Errorf("Timeout = %v, want 1h30m", c.Timeout)
		}
	})

	t.Run("zero when flag absent", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Timeout != 0 {
			t.Errorf("Timeout = %v, want 0", c.Timeout)
		}
	})

	t.Run("default applied", func(t *testing.T) {
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
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-timeout", "notaduration"}); err == nil {
			t.Fatal("expected error for invalid duration, got nil")
		}
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
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
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout" sep:","`
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
		type cfg struct {
			Timeouts []time.Duration `default:"1s,2m" flag:"timeout" sep:","`
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
		type cfg struct {
			Timeouts []time.Duration `default:"1s,notaduration" flag:"timeout" sep:","`
		}
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err == nil {
			t.Fatal("expected error for invalid duration default, got nil")
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
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
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-d", "2024-03-15T00:00:00Z"}); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if len(c.Dates) != 1 {
			t.Errorf("len(Dates) = %d, want 1", len(c.Dates))
		}
	})

	t.Run("no flags gives nil", func(t *testing.T) {
		var c cfg
		if err := conflagure(newTestFS(), &c, nil); err != nil {
			t.Fatalf("conflagure() error = %v", err)
		}
		if c.Dates != nil {
			t.Errorf("Dates = %v, want nil", c.Dates)
		}
	})

	t.Run("now in default resolves to non-zero time", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `default:"now" flag:"date" sep:""`
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
		type cfg2 struct {
			Dates []time.Time `default:"2023-01-01T00:00:00Z" flag:"date" sep:""`
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
		var c cfg
		if err := conflagure(newTestFS(), &c, []string{"-date", "not-a-time"}); err == nil {
			t.Fatal("expected error for invalid time value, got nil")
		}
	})

	t.Run("invalid default returns error", func(t *testing.T) {
		type cfg2 struct {
			Dates []time.Time `default:"not-a-time" flag:"date" sep:""`
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
		parse := parseTimeFunc(time.RFC3339)
		if _, err := parse("not-a-time"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("custom layout", func(t *testing.T) {
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
			if err := registerFlag(fs, &s, flagSpec{Name: tt.flagName, Short: tt.shortName, Help: "long help text"}); err != nil {
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
	if !isErr(err, ErrDispatchNoSelection) {
		t.Errorf("expected ErrDispatchNoSelection when no subcommand selected, got %v", err)
	}
	if !c.Verbose {
		t.Error("Verbose not set")
	}
}

// testRestAndCmdSub implements Commander but has both narg:"rest" and a cmd:
// field, which is an invalid combination. Used to exercise the error path in
// validateCommanders where collectSubcommands is called on the sub-struct after
// the outer collect already succeeded.
type testRestAndCmdSub struct {
	Rest []string      `narg:"rest"`
	Sub  testDeployCmd `cmd:"sub"`
}

func (t *testRestAndCmdSub) Run(_ context.Context) error { return nil }

// testFlagAndCmdField implements Commander and has an inner field that carries
// both flag: and cmd: tags. The outer collectSubcommands succeeds (bindFlags
// skips cmd: fields silently), but when selectSubcommand calls collectSubcommands
// on this struct after dispatch, the flag+cmd conflict is detected.
// This is the correct type to exercise subcommand.go:163-165.
type testFlagAndCmdField struct {
	Sub testDeployCmd `cmd:"sub" flag:"sub"`
}

func (t *testFlagAndCmdField) Run(_ context.Context) error { return nil }

// testL3NoCommander is a plain struct with no Run method used to test that
// validateCommanders catches a missing Commander implementation three levels deep.
type testL3NoCommander struct {
	V string `flag:"v"`
}

// testL2WithDeepBadSub has a nested cmd: field whose type does not implement
// Commander. Used to test the recursive-error path in validateCommanders.
type testL2WithDeepBadSub struct {
	Nested testL3NoCommander `cmd:"nested"`
}

func (t *testL2WithDeepBadSub) Run(_ context.Context) error { return nil }

// testCmdWithNestedCmd is a Commander whose struct body contains a nested cmd:
// field, allowing multi-level subcommand dispatch.
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

func Test_conflagureCmd_no_subcommand_named(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testDeployCmd `cmd:"deploy"`
	}

	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if !isErr(err, ErrDispatchNoSelection) {
		t.Errorf("error = %v, want ErrDispatchNoSelection", err)
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

func Test_conflagureCmd_conf_errors(t *testing.T) {
	t.Parallel()

	t.Run("nil conf", func(t *testing.T) {
		_, err := conflagureCmd(newTestFS(), nil, nil)
		if err == nil {
			t.Fatal("expected error for nil conf")
		}
	})

	t.Run("non-pointer conf", func(t *testing.T) {
		type cfg struct{ X string }
		_, err := conflagureCmd(newTestFS(), cfg{}, nil)
		if err == nil {
			t.Fatal("expected error for non-pointer conf")
		}
	})
}

// ── collectSubcommands error paths ────────────────────────────────────────────

func Test_collectSubcommands_errors(t *testing.T) {
	t.Parallel()

	t.Run("field with both flag: and cmd: tags", func(t *testing.T) {
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
	})

	t.Run("unexported cmd: field", func(t *testing.T) {
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
	})

	t.Run("non-struct cmd: field", func(t *testing.T) {
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
	})

	t.Run("non-Commander struct is accepted by collectSubcommands", func(t *testing.T) {
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
	})
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

// ── nested cmd: tag ───────────────────────────────────────────────────────────

func Test_bindFlags_nested_cmd_tag_skipped(t *testing.T) {
	t.Parallel()

	// bindFlags silently skips cmd: fields at any depth; subcommand dispatch
	// handles them separately.
	fs := newTestFS()
	type inner struct {
		Target string        `flag:"target"`
		Sub    testDeployCmd `cmd:"sub"`
	}
	var b inner
	if err := bindFlags(fs, reflectVal(&b), "", 1); err != nil {
		t.Errorf("bindFlags() unexpected error for nested cmd: field: %v", err)
	}
	// Only -target should be registered; cmd: field is skipped.
	if fs.Lookup("target") == nil {
		t.Error("expected -target flag to be registered")
	}
}

// ── dispatch ──────────────────────────────────────────────────────────────────

func Test_dispatch_conf_errors(t *testing.T) {
	t.Parallel()

	t.Run("nil conf", func(t *testing.T) {
		_, err := dispatch(newTestFS(), nil, nil)
		if err == nil {
			t.Fatal("expected error for nil conf")
		}
	})

	t.Run("non-pointer conf", func(t *testing.T) {
		type cfg struct{ X string }
		_, err := dispatch(newTestFS(), cfg{}, nil)
		if err == nil {
			t.Fatal("expected error for non-pointer conf")
		}
	})

	t.Run("no subcommands returns ErrDispatchNoSelection", func(t *testing.T) {
		type cfg struct {
			X string `flag:"x"`
		}
		var c cfg
		_, err := dispatch(newTestFS(), &c, nil)
		if !isErr(err, ErrDispatchNoSelection) {
			t.Errorf("expected ErrDispatchNoSelection when no cmd: fields, got %v", err)
		}
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ── additional coverage tests ─────────────────────────────────────────────────

// Test_conflagureCmd_usage_closure exercises the custom Usage closure installed
// by conflagureCmd: triggered by an unknown flag, covering the with-help,
// nil-origUsage, and preset-origUsage branches.
func Test_conflagureCmd_usage_closure(t *testing.T) {
	t.Parallel()

	t.Run("with help and no-help subcommands", func(t *testing.T) {
		// Deploy has a help tag; Rollback does not — both loop branches are hit.
		type cfg struct {
			Deploy   testDeployCmd   `cmd:"deploy"   help:"deploy to target"`
			Rollback testRollbackCmd `cmd:"rollback"`
		}
		fs := newTestFS()
		fs.SetOutput(io.Discard)
		var c cfg
		_, err := conflagureCmd(fs, &c, []string{"-unknown-flag"})
		if err == nil {
			t.Fatal("expected parse error for unknown flag")
		}
	})

	t.Run("nil origUsage", func(t *testing.T) {
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
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !customUsageCalled {
			t.Error("expected original Usage func to be called")
		}
	})
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

// Test_conflagureCmd_errors consolidates error-path coverage for conflagureCmd:
// collectSubcommands failure, bindFlags failure, global required flag missing,
// and subcommand parse failure.
func Test_conflagureCmd_errors(t *testing.T) {
	t.Parallel()

	t.Run("collectSubcommands error", func(t *testing.T) {
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
	})

	t.Run("bindFlags error — invalid short flag", func(t *testing.T) {
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
	})

	t.Run("global required flag missing", func(t *testing.T) {
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
	})

	t.Run("subcommand parse error — unknown subcmd flag", func(t *testing.T) {
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
	})
}

// Test_dispatch_errors consolidates error-path coverage for dispatch:
// collectSubcommands failure, global parse failure, and unknown subcommand.
func Test_dispatch_errors(t *testing.T) {
	t.Parallel()

	t.Run("collectSubcommands error", func(t *testing.T) {
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
		if err == nil {
			t.Fatal("expected parse error for unknown flag")
		}
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
		if err == nil {
			t.Fatal("expected error for unknown subcommand")
		}
		if !isErr(err, ErrInvalidInput) {
			t.Errorf("error = %v, want ErrInvalidInput", err)
		}
	})
}

// Test_collectSubcommands_nested_cmd collects a subcommand whose struct
// itself contains a nested cmd: field. Nesting is now supported.
func Test_collectSubcommands_nested_cmd(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	entries, err := collectSubcommands(reflectVal(&c), "test")
	if err != nil {
		t.Fatalf("collectSubcommands() unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].name != "deploy" {
		t.Errorf("entries = %v, want [{deploy ...}]", entries)
	}
}

// Test_conflagureCmd_nested_subcommand exercises two-level dispatch via
// conflagureCmd: the outer subcommand itself has a cmd: field.
func Test_conflagureCmd_nested_subcommand(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool                 `flag:"verbose"`
		Deploy  testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	// "deploy sub -target=prod" → global parse, then deploy, then sub.
	cmd, err := conflagureCmd(newTestFS(), &c, []string{"deploy", "sub", "-target=prod"})
	if err != nil {
		t.Fatalf("conflagureCmd() error = %v", err)
	}
	if _, ok := cmd.(*testDeployCmd); !ok {
		t.Errorf("cmd type = %T, want *testDeployCmd", cmd)
	}
	if c.Deploy.Sub.Target != "prod" {
		t.Errorf("Sub.Target = %q, want %q", c.Deploy.Sub.Target, "prod")
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

// testFlagValue is a minimal flag.Value implementation used to test the
// flag.Value case in registerFlag. Set appends each call to vals so tests can
// verify how many times it was called and with what arguments.
type testFlagValue struct {
	vals   []string
	setErr error // if non-nil, Set returns this error
}

func (v *testFlagValue) String() string {
	if len(v.vals) == 0 {
		return ""
	}
	return v.vals[len(v.vals)-1]
}

func (v *testFlagValue) Set(s string) error {
	if v.setErr != nil {
		return v.setErr
	}
	v.vals = append(v.vals, s)
	return nil
}

// Test_registerFlag_errors covers the duplicate-name, duplicate-short, and
// impossible-bool-default error branches in registerFlag / bindFlags.
func Test_registerFlag_errors(t *testing.T) {
	t.Parallel()

	t.Run("duplicate long flag name", func(t *testing.T) {
		type cfg struct {
			A string `flag:"dup"`
			B string `flag:"dup"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for duplicate flag name")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("duplicate short flag", func(t *testing.T) {
		type cfg struct {
			Alpha string `flag:"alpha" short:"x"`
			Beta  string `flag:"beta"  short:"x"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for duplicate short flag")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("bool impossible default", func(t *testing.T) {
		// The default: value is neither a recognised bool string nor empty.
		type cfg struct {
			Flag bool `default:"maybe" flag:"flag"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for impossible bool default")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})
}

// Test_dispatch_subcommand_rest_parse_error checks that populateRestField errors
// are propagated from selectSubcommand when a subcommand's rest field cannot
// parse the remaining args (e.g. []int rest field given a non-integer arg).
func Test_dispatch_subcommand_rest_parse_error(t *testing.T) {
	t.Parallel()

	type intCmd struct {
		Nums []int `narg:"rest"`
	}
	type cfg struct {
		Verbose bool   `flag:"verbose"`
		Calc    intCmd `cmd:"calc"`
	}
	var c cfg
	// "notanint" is passed as a remaining arg after the subcommand name.
	_, err := dispatch(newTestFS(), &c, []string{"calc", "notanint"})
	if err == nil {
		t.Fatal("expected error from populateRestField with non-integer rest arg")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

// ── narg:"rest" tests ─────────────────────────────────────────────────────────

func Test_narg_rest_basic(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `narg:"rest"`
	}

	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr bool
	}{
		{name: "single arg", args: []string{"foo.txt"}, want: []string{"foo.txt"}},
		{name: "multiple args", args: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
		{name: "no args", args: nil, want: nil},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); (err != nil) != tt.wantErr {
				t.Fatalf("conflagure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(c.Files, tt.want) {
				t.Errorf("Files = %v, want %v", c.Files, tt.want)
			}
		})
	}
}

func Test_narg_rest_with_flags(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Output string   `flag:"output"`
		Files  []string `narg:"rest"`
	}

	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantFiles  []string
	}{
		{
			name:       "flags then rest",
			args:       []string{"-output", "out.txt", "foo.txt", "bar.txt"},
			wantOutput: "out.txt",
			wantFiles:  []string{"foo.txt", "bar.txt"},
		},
		{
			name:       "rest only",
			args:       []string{"foo.txt", "bar.txt"},
			wantOutput: "",
			wantFiles:  []string{"foo.txt", "bar.txt"},
		},
		{
			name:       "flag only, no rest",
			args:       []string{"-output", "out.txt"},
			wantOutput: "out.txt",
			wantFiles:  nil,
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if c.Output != tt.wantOutput {
				t.Errorf("Output = %q, want %q", c.Output, tt.wantOutput)
			}
			if !reflect.DeepEqual(c.Files, tt.wantFiles) {
				t.Errorf("Files = %v, want %v", c.Files, tt.wantFiles)
			}
		})
	}
}

func Test_narg_rest_typed_int(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Numbers []int `narg:"rest"`
	}

	tests := []struct {
		name    string
		args    []string
		want    []int
		wantErr bool
	}{
		{name: "multiple ints", args: []string{"1", "2", "3"}, want: []int{1, 2, 3}},
		{name: "single int", args: []string{"42"}, want: []int{42}},
		{name: "invalid int", args: []string{"notanint"}, wantErr: true},
		{name: "no args", args: nil, want: nil},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); (err != nil) != tt.wantErr {
				t.Fatalf("conflagure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(c.Numbers, tt.want) {
				t.Errorf("Numbers = %v, want %v", c.Numbers, tt.want)
			}
		})
	}
}

func Test_narg_rest_typed_float64(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Values []float64 `narg:"rest"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"1.1", "2.2", "3.3"}); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	want := []float64{1.1, 2.2, 3.3}
	if !reflect.DeepEqual(c.Values, want) {
		t.Errorf("Values = %v, want %v", c.Values, want)
	}
}

func Test_narg_rest_required_empty(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `narg:"rest" required:"true"`
	}

	var c cfg
	err := conflagure(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected error for required rest field with no args")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

func Test_narg_rest_required_provided(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `narg:"rest" required:"true"`
	}

	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"a.txt"}); err != nil {
		t.Fatalf("conflagure() unexpected error = %v", err)
	}
	if len(c.Files) != 1 || c.Files[0] != "a.txt" {
		t.Errorf("Files = %v, want [a.txt]", c.Files)
	}
}

// ── narg:"until" tests ────────────────────────────────────────────────────────

func Test_narg_until_basic(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files  []string `flag:"files"  narg:"until"`
		Output string   `flag:"output"`
	}

	tests := []struct {
		name       string
		args       []string
		wantFiles  []string
		wantOutput string
	}{
		{
			name:       "basic until",
			args:       []string{"-files", "a.txt", "b.txt", "c.txt", "-output", "out.txt"},
			wantFiles:  []string{"a.txt", "b.txt", "c.txt"},
			wantOutput: "out.txt",
		},
		{
			name:      "until at end",
			args:      []string{"-files", "a.txt", "b.txt"},
			wantFiles: []string{"a.txt", "b.txt"},
		},
		{
			name:      "single value",
			args:      []string{"-files", "a.txt"},
			wantFiles: []string{"a.txt"},
		},
		{
			name:       "no until values, just output",
			args:       []string{"-output", "out.txt"},
			wantOutput: "out.txt",
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			if err := conflagure(newTestFS(), &c, tt.args); err != nil {
				t.Fatalf("conflagure() error = %v", err)
			}
			if !reflect.DeepEqual(c.Files, tt.wantFiles) {
				t.Errorf("Files = %v, want %v", c.Files, tt.wantFiles)
			}
			if c.Output != tt.wantOutput {
				t.Errorf("Output = %q, want %q", c.Output, tt.wantOutput)
			}
		})
	}
}

func Test_narg_until_eq_form(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files  []string `flag:"files"  narg:"until"`
		Output string   `flag:"output"`
	}

	// -files=a.txt b.txt c.txt -output out.txt -> Files = [a.txt, b.txt, c.txt], Output = out.txt
	var c cfg
	if err := conflagure(newTestFS(), &c, []string{"-files=a.txt", "b.txt", "c.txt", "-output", "out.txt"}); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	wantFiles := []string{"a.txt", "b.txt", "c.txt"}
	if !reflect.DeepEqual(c.Files, wantFiles) {
		t.Errorf("Files = %v, want %v", c.Files, wantFiles)
	}
	if c.Output != "out.txt" {
		t.Errorf("Output = %q, want out.txt", c.Output)
	}
}

func Test_narg_until_multiple_fields(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Inputs  []string `flag:"input"  narg:"until"`
		Outputs []string `flag:"output" narg:"until"`
	}

	var c cfg
	args := []string{"-input", "a.txt", "b.txt", "-output", "x.txt", "y.txt"}
	if err := conflagure(newTestFS(), &c, args); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if !reflect.DeepEqual(c.Inputs, []string{"a.txt", "b.txt"}) {
		t.Errorf("Inputs = %v, want [a.txt b.txt]", c.Inputs)
	}
	if !reflect.DeepEqual(c.Outputs, []string{"x.txt", "y.txt"}) {
		t.Errorf("Outputs = %v, want [x.txt y.txt]", c.Outputs)
	}
}

func Test_narg_until_coexists_with_regular_slice(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags  []string `flag:"tag"`
		Files []string `flag:"files" narg:"until"`
	}

	var c cfg
	args := []string{"-tag", "go", "-tag", "cli", "-files", "a.txt", "b.txt"}
	if err := conflagure(newTestFS(), &c, args); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if !reflect.DeepEqual(c.Tags, []string{"go", "cli"}) {
		t.Errorf("Tags = %v, want [go cli]", c.Tags)
	}
	if !reflect.DeepEqual(c.Files, []string{"a.txt", "b.txt"}) {
		t.Errorf("Files = %v, want [a.txt b.txt]", c.Files)
	}
}

// ── narg validation error tests ───────────────────────────────────────────────

// Test_narg_invalid_spec_errors consolidates all ErrInvalidSpecification cases
// for narg: tag misuse.
func Test_narg_invalid_spec_errors(t *testing.T) {
	t.Parallel()

	t.Run("rest with flag: tag", func(t *testing.T) {
		type cfg struct {
			Files []string `flag:"files" narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for narg:rest with flag: tag")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("until without flag: tag", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"until"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for narg:until without flag: tag")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("rest on non-slice type", func(t *testing.T) {
		type cfg struct {
			File string `narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for narg:rest on non-slice type")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("until on non-slice type", func(t *testing.T) {
		type cfg struct {
			File string `flag:"file" narg:"until"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for narg:until on non-slice type")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("unknown narg value", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"unknown"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for unknown narg value")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("rest and cmd: are incompatible", func(t *testing.T) {
		type sub struct {
			X string `flag:"x"`
		}
		type cfg struct {
			Args   []string `narg:"rest"`
			Deploy sub      `cmd:"deploy"`
		}
		var c cfg
		_, err := dispatch(newTestFS(), &c, nil)
		if err == nil {
			t.Fatal("expected error for narg:rest with cmd: fields")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})

	t.Run("multiple rest fields", func(t *testing.T) {
		type cfg struct {
			Files1 []string `narg:"rest"`
			Files2 []string `narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, []string{"a"})
		if err == nil {
			t.Fatal("expected error for multiple narg:rest fields")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("error = %v, want ErrInvalidSpecification", err)
		}
	})
}

// ── narg:"rest" in subcommand context ─────────────────────────────────────────

func Test_narg_rest_in_subcommand(t *testing.T) {
	t.Parallel()

	type sub struct {
		Output string   `flag:"output"`
		Files  []string `narg:"rest"`
	}
	type cfg struct {
		Deploy sub `cmd:"deploy"`
	}

	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantFiles  []string
	}{
		{
			name:       "rest after flag",
			args:       []string{"deploy", "-output", "out.txt", "a.txt", "b.txt"},
			wantOutput: "out.txt",
			wantFiles:  []string{"a.txt", "b.txt"},
		},
		{
			name:      "rest only",
			args:      []string{"deploy", "a.txt", "b.txt"},
			wantFiles: []string{"a.txt", "b.txt"},
		},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			result, err := dispatch(newTestFS(), &c, tt.args)
			if err != nil {
				t.Fatalf("dispatch() error = %v", err)
			}
			got, ok := result.(*sub)
			if !ok {
				t.Fatalf("result is not *sub, got %T", result)
			}
			if got.Output != tt.wantOutput {
				t.Errorf("Output = %q, want %q", got.Output, tt.wantOutput)
			}
			if !reflect.DeepEqual(got.Files, tt.wantFiles) {
				t.Errorf("Files = %v, want %v", got.Files, tt.wantFiles)
			}
		})
	}
}

func Test_narg_until_in_subcommand(t *testing.T) {
	t.Parallel()

	type sub struct {
		Files  []string `flag:"files"  narg:"until"`
		Output string   `flag:"output"`
	}
	type cfg struct {
		Deploy sub `cmd:"deploy"`
	}

	var c cfg
	args := []string{"deploy", "-files", "a.txt", "b.txt", "-output", "out.txt"}
	result, err := dispatch(newTestFS(), &c, args)
	if err != nil {
		t.Fatalf("dispatch() error = %v", err)
	}
	got, ok := result.(*sub)
	if !ok {
		t.Fatalf("result is not *sub, got %T", result)
	}
	if !reflect.DeepEqual(got.Files, []string{"a.txt", "b.txt"}) {
		t.Errorf("Files = %v, want [a.txt b.txt]", got.Files)
	}
	if got.Output != "out.txt" {
		t.Errorf("Output = %q, want out.txt", got.Output)
	}
}

func Test_narg_rest_required_in_subcommand_empty(t *testing.T) {
	t.Parallel()

	type sub struct {
		Files []string `narg:"rest" required:"true"`
	}
	type cfg struct {
		Deploy sub `cmd:"deploy"`
	}

	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"deploy"})
	if err == nil {
		t.Fatal("expected error for required rest field in subcommand with no args")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("error = %v, want ErrInvalidInput", err)
	}
}

func Test_tryCastCommander(t *testing.T) {
	t.Parallel()

	t.Run("implements Commander returns it", func(t *testing.T) {
		var d testDeployCmd
		cmd, err := tryCastCommander(&d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd == nil {
			t.Fatal("expected non-nil Commander")
		}
		if _, ok := cmd.(*testDeployCmd); !ok {
			t.Errorf("cmd type = %T, want *testDeployCmd", cmd)
		}
	})

	t.Run("does not implement Commander returns ErrInvalidSpecification", func(t *testing.T) {
		type plain struct{ X string }
		_, err := tryCastCommander(&plain{})
		if err == nil {
			t.Fatal("expected error for non-Commander type")
		}
		if !isErr(err, ErrInvalidSpecification) {
			t.Errorf("err = %v, want ErrInvalidSpecification", err)
		}
	})
}

func Test_narg_until_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `flag:"files" narg:"until" short:"f"`
	}

	var c cfg
	args := []string{"-f", "a.txt", "b.txt"}
	if err := conflagure(newTestFS(), &c, args); err != nil {
		t.Fatalf("conflagure() error = %v", err)
	}
	if !reflect.DeepEqual(c.Files, []string{"a.txt", "b.txt"}) {
		t.Errorf("Files = %v, want [a.txt b.txt]", c.Files)
	}
}

// ── coverage: subcommand.go uncovered paths ───────────────────────────────────

// Test_collectSubcommands_bindflags_err covers the error path at
// subcommand.go:79-81 where bindFlags returns an error while building a
// subcommand entry (e.g. an unexported flag field inside the subcommand struct).
func Test_collectSubcommands_bindflags_err(t *testing.T) {
	t.Parallel()

	// badSub has an unexported field with a flag: tag; bindFlags rejects this.
	type badSub struct {
		//nolint:unused
		secret string `flag:"secret"` //nolint:structcheck
	}
	type cfg struct {
		Deploy badSub `cmd:"deploy"`
	}
	var c cfg
	_, err := collectSubcommands(reflectVal(&c), "test")
	if err == nil {
		t.Fatal("expected error from bindFlags for unexported flag field in subcommand")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// Test_validateCommanders_inner_collectSubcommands_err covers subcommand.go:104-106
// where collectSubcommands errors on the sub-struct inside validateCommanders.
// The outer collectSubcommands succeeds because it does not recurse into cmd:
// fields; only validateCommanders calls collectSubcommands on each entry value.
func Test_validateCommanders_inner_collectSubcommands_err(t *testing.T) {
	t.Parallel()

	// testRestAndCmdSub has narg:"rest" and cmd: at the same level — invalid, but
	// the outer collectSubcommands does not catch it; validateCommanders does.
	type cfg struct {
		Deploy testRestAndCmdSub `cmd:"deploy"`
	}
	var c cfg
	entries, err := collectSubcommands(reflectVal(&c), "test")
	if err != nil {
		t.Fatalf("outer collectSubcommands() unexpected error: %v", err)
	}
	err = validateCommanders(entries)
	if err == nil {
		t.Fatal("expected error from collectSubcommands inside validateCommanders")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// Test_validateCommanders_recursive_err covers subcommand.go:107-109 where
// the recursive validateCommanders call returns an error because a nested
// subcommand does not implement Commander.
func Test_validateCommanders_recursive_err(t *testing.T) {
	t.Parallel()

	// testL2WithDeepBadSub implements Commander but contains testL3NoCommander
	// (which does not implement Commander) as a nested subcommand.
	type cfg struct {
		Mid testL2WithDeepBadSub `cmd:"mid"`
	}
	var c cfg
	_, err := conflagureCmd(newTestFS(), &c, nil)
	if err == nil {
		t.Fatal("expected ErrInvalidSpecification for non-Commander nested subcommand")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// Test_selectSubcommand_inner_collectSubcommands_err covers subcommand.go:163-165
// where collectSubcommands errors on the matched subcommand's struct during
// recursive dispatch. Reached via dispatch (which skips validateCommanders).
//
// testFlagAndCmdField is used because its inner flag+cmd conflict is not caught
// by the outer collectSubcommands (bindFlags skips cmd: fields) or by
// populateRestField (no rest field), so it reaches line 162 before failing.
func Test_selectSubcommand_inner_collectSubcommands_err(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Deploy testFlagAndCmdField `cmd:"deploy"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"deploy"})
	if err == nil {
		t.Fatal("expected error from inner collectSubcommands during selectSubcommand")
	}
	if !isErr(err, ErrInvalidSpecification) {
		t.Errorf("err = %v, want ErrInvalidSpecification", err)
	}
}

// Test_selectSubcommand_recursive_unknown_subcommand covers subcommand.go:168-170
// where the recursive selectSubcommand call returns a non-ErrNotFound error
// (an unknown subcommand name at the nested level).
func Test_selectSubcommand_recursive_unknown_subcommand(t *testing.T) {
	t.Parallel()

	// testCmdWithNestedCmd has cmd:"sub" internally. Dispatching to "deploy bogus"
	// selects deploy, then tries to select "bogus" from deploy's subcommands, which
	// fails with ErrInvalidInput (unknown subcommand).
	type cfg struct {
		Deploy testCmdWithNestedCmd `cmd:"deploy"`
	}
	var c cfg
	_, err := dispatch(newTestFS(), &c, []string{"deploy", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown nested subcommand")
	}
	if !isErr(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}
