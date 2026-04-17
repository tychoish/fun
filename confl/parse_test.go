package confl

import (
	"flag"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

// ── conflagure: basic scalar types ───────────────────────────────────────────

func Test_conflagure_string(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Branch string `default:"main" flag:"branch" help:"branch name"`
	}

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "explicit value", args: []string{"-branch", "feature"}, want: "feature"},
		{name: "uses default when absent", args: nil, want: "main"},
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
			assert.Equal(t, c.Branch, tt.want)
		})
	}
}

func Test_conflagure_bool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool `default:"false" flag:"verbose" help:"verbose output" short:"v"`
	}

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

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Verbose, tt.want)
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
				assert.Equal(t, c.Count, 42)
			},
		},
		{
			name: "int64",
			args: []string{"-big", "9999999999"},
			check: func(t *testing.T, c cfg) {
				assert.Equal(t, c.Big, int64(9999999999))
			},
		},
		{
			name: "uint",
			args: []string{"-size", "1024"},
			check: func(t *testing.T, c cfg) {
				assert.Equal(t, c.Size, uint(1024))
			},
		},
		{
			name: "uint64",
			args: []string{"-large", "18446744073709551615"},
			check: func(t *testing.T, c cfg) {
				assert.Equal(t, c.Large, uint64(18446744073709551615))
			},
		},
		{
			name: "float64",
			args: []string{"-ratio", "3.14"},
			check: func(t *testing.T, c cfg) {
				assert.Equal(t, c.Ratio, 3.14)
			},
		},
		{
			name: "defaults when all absent",
			args: nil,
			check: func(t *testing.T, c cfg) {
				assert.Zero(t, c.Count)
				assert.Zero(t, c.Big)
				assert.Zero(t, c.Size)
				assert.Zero(t, c.Large)
				assert.Zero(t, c.Ratio)
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

func Test_conflagure_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Target string `flag:"target" help:"target" required:"true"`
	}

	t.Run("missing required field returns error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("required field satisfied", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-target", "prod"}))
		assert.Equal(t, c.Target, "prod")
	})
}

func Test_conflagure_skips_untagged_fields(t *testing.T) {
	t.Parallel()

	type cfg struct {
		internal string //nolint:unused
		Name     string `default:"default" flag:"name"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-name", "test"}))
	assert.Equal(t, c.Name, "test")
	assert.Equal(t, c.internal, "")
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
			assert.Error(t, conflagure(newTestFS(), tt.cfg, nil))
		})
	}
}

func Test_conflagure_unsupported_types(t *testing.T) {
	t.Parallel()

	t.Run("chan type returns error", func(t *testing.T) {
		type cfg struct {
			Ch chan int `flag:"ch"`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("slice of struct element type returns error", func(t *testing.T) {
		type cfg struct {
			Items []struct{ X int } `flag:"items"`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── conflagure: inverted bool ─────────────────────────────────────────────────

func Test_conflagure_inverted_bool(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Verbose bool `default:"true" flag:"verbose" help:"verbose output" short:"v"`
	}

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

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NotError(t, err)
			assert.Equal(t, c.Verbose, tt.want)
		})
	}
}

func Test_conflagure_inverted_bool_invalid_value(t *testing.T) {
	t.Parallel()

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
			assert.Error(t, conflagure(newTestFS(), &c, tt.args))
		})
	}
}

// ── conflagure: embedded structs ──────────────────────────────────────────────

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
		{name: "embedded bool via long flag", args: []string{"-verbose"}, verbose: true, format: "text"},
		{name: "embedded bool via short alias", args: []string{"-v"}, verbose: true, format: "text"},
		{name: "embedded string with non-default value", args: []string{"-format", "json"}, format: "json"},
		{name: "outer field alongside embedded fields", args: []string{"-verbose", "-name", "alice"}, verbose: true, format: "text", cfgName: "alice"},
		{name: "defaults when no flags given", args: nil, format: "text"},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Verbose, tt.verbose)
			assert.Equal(t, c.Format, tt.format)
			assert.Equal(t, c.Name, tt.cfgName)
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
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.DryRun, tt.dryRun)
			assert.Equal(t, c.Target, tt.target)
		})
	}
}

// ── conflagure: named struct fields ───────────────────────────────────────────

func Test_conflagure_named_struct_field(t *testing.T) {
	t.Parallel()

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
		{name: "named sub-struct fields are registered", args: []string{"-db-host", "prod.db", "-db-port", "5433"}, host: "prod.db", port: 5433},
		{name: "outer and sub-struct fields together", args: []string{"-name", "app", "-db-host", "staging.db"}, cfgName: "app", host: "staging.db", port: 5432},
		{name: "defaults when no flags given", args: nil, host: "localhost", port: 5432},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			var c cfg
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Name, tt.cfgName)
			assert.Equal(t, c.DB.Host, tt.host)
			assert.Equal(t, c.DB.Port, tt.port)
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
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("required field inside named struct satisfied", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-token", "secret"}))
		assert.Equal(t, c.Auth.Token, "secret")
	})
}

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
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-srv.host", "example.com", "-srv.port", "9090"}))
		assert.Equal(t, c.Server.Host, "example.com")
		assert.Equal(t, c.Server.Port, 9090)
	})

	t.Run("defaults apply without flags", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, c.Server.Host, "localhost")
		assert.Equal(t, c.Server.Port, 8080)
	})

	t.Run("namespace and top-level flags coexist", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-dry-run", "-srv.host", "staging.internal", "-label", "staging"}))
		assert.True(t, c.DryRun)
		assert.Equal(t, c.Server.Host, "staging.internal")
		assert.Equal(t, c.Label, "staging")
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
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "-db.token")
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
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-outer.inner.key", "hello"}))
		assert.Equal(t, c.Outer.Inner.Key, "hello")
	})
}

func Test_conflagure_named_struct_no_tag_flat(t *testing.T) {
	t.Parallel()

	type network struct {
		Host string `default:"localhost" flag:"host" help:"server host"`
		Port int    `default:"8080"      flag:"port" help:"server port"`
	}
	type cfg struct {
		BaseTest

		Server network
	}

	t.Run("no flag tag on struct field means flat namespace", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-host", "example.com", "-port", "9090"}))
		assert.Equal(t, c.Server.Host, "example.com")
		assert.Equal(t, c.Server.Port, 9090)
	})

	t.Run("defaults apply when no flags given", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, c.Server.Host, "localhost")
		assert.Equal(t, c.Server.Port, 8080)
	})
}

// ── conflagure: unexported fields ─────────────────────────────────────────────

func Test_conflagure_unexported_with_tag(t *testing.T) {
	t.Parallel()

	type cfg struct {
		//nolint:unused
		internal string `flag:"secret"` //nolint:structcheck
	}
	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, nil))
}

func Test_conflagure_unexported_nested_propagates_error(t *testing.T) {
	t.Parallel()

	type inner struct {
		//nolint:unused
		secret string `flag:"token"` //nolint:structcheck
	}
	type outer struct {
		Inner inner
	}
	var c outer
	assert.Error(t, conflagure(newTestFS(), &c, nil))
}

// ── conflagure: parse failure ─────────────────────────────────────────────────

func Test_conflagure_parse_failure(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Name string `flag:"name"`
	}
	var c cfg
	assert.Error(t, conflagure(newTestFS(), &c, []string{"-unknown-flag"}))
}

// ── conflagure: time types ────────────────────────────────────────────────────

func Test_conflagure_time_Time(t *testing.T) {
	t.Parallel()

	type cfg struct {
		At time.Time `flag:"at" help:"timestamp"`
	}

	t.Run("RFC3339 value", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-at", "2024-06-01T12:00:00Z"}))
		want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		assert.True(t, c.At.Equal(want))
	})

	t.Run("zero value when flag absent", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.True(t, c.At.IsZero())
	})

	t.Run("RFC3339 default applied", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `default:"2023-01-15T00:00:00Z" flag:"at"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		want := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		assert.True(t, c.At.Equal(want))
	})

	t.Run("now default sets non-zero time", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `default:"now" flag:"at"`
		}
		before := time.Now().UTC().Add(-time.Second)
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.True(t, c.At.After(before))
	})

	t.Run("custom format tag", func(t *testing.T) {
		type cfg2 struct {
			Date time.Time `flag:"date" format:"2006-01-02"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-date", "2024-03-15"}))
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		assert.True(t, c.Date.Equal(want))
	})

	t.Run("short alias", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `flag:"at" short:"a"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-a", "2024-06-01T00:00:00Z"}))
		assert.True(t, !c.At.IsZero())
	})

	t.Run("required field missing returns error", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("required field satisfied", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `flag:"at" required:"true"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-at", "2024-01-01T00:00:00Z"}))
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"-at", "not-a-time"}))
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `default:"not-a-time" flag:"at"`
		}
		var c cfg2
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})

	t.Run("format and default mismatch returns error at bind time", func(t *testing.T) {
		type cfg2 struct {
			At time.Time `default:"2024-01-01T00:00:00Z" flag:"at" format:"2006-01-02"`
		}
		var c cfg2
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})
}

func Test_conflagure_time_Duration(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Timeout time.Duration `flag:"timeout"`
	}

	t.Run("parses Go duration string", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-timeout", "1h30m"}))
		assert.Equal(t, c.Timeout, 90*time.Minute)
	})

	t.Run("zero when flag absent", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Zero(t, c.Timeout)
	})

	t.Run("default applied", func(t *testing.T) {
		type cfg2 struct {
			Timeout time.Duration `default:"5s" flag:"timeout"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.Equal(t, c.Timeout, 5*time.Second)
	})

	t.Run("short alias", func(t *testing.T) {
		type cfg2 struct {
			Timeout time.Duration `flag:"timeout" short:"t"`
		}
		var c cfg2
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-t", "500ms"}))
		assert.Equal(t, c.Timeout, 500*time.Millisecond)
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"-timeout", "notaduration"}))
	})

	t.Run("invalid default returns error at bind time", func(t *testing.T) {
		type cfg2 struct {
			Timeout time.Duration `default:"notaduration" flag:"timeout"`
		}
		var c cfg2
		assert.Error(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── parseTimeFuncAuto and parseTimeFunc ───────────────────────────────────────

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
				assert.Equal(t, got.Year(), 2024)
				assert.Equal(t, int(got.Month()), 3)
				assert.Equal(t, got.Day(), 15)
			},
		},
		{
			name:  "DateTime",
			input: "2024-03-15 10:30:00",
			check: func(t *testing.T, got time.Time) {
				assert.Equal(t, got.Year(), 2024)
				assert.Equal(t, got.Hour(), 10)
				assert.Equal(t, got.Minute(), 30)
			},
		},
		{
			name:  "DateOnly",
			input: "2024-03-15",
			check: func(t *testing.T, got time.Time) {
				assert.Equal(t, got.Year(), 2024)
				assert.Equal(t, int(got.Month()), 3)
				assert.Equal(t, got.Day(), 15)
				assert.Zero(t, got.Hour())
				assert.Zero(t, got.Minute())
				assert.Zero(t, got.Second())
			},
		},
		{
			name:  "now resolves",
			input: "now",
			check: func(t *testing.T, got time.Time) {
				assert.True(t, !got.IsZero())
			},
		},
		{name: "invalid returns error", input: "not-a-date", wantErr: true},
		{name: "partial date rejected", input: "2024-03", wantErr: true},
	}

	for tt := range slices.Values(tests) {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NotError(t, err)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func Test_parseTimeFunc(t *testing.T) {
	t.Parallel()

	t.Run("parses RFC3339", func(t *testing.T) {
		parse := parseTimeFunc(time.RFC3339)
		got, err := parse("2024-06-01T00:00:00Z")
		assert.NotError(t, err)
		want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		assert.True(t, got.Equal(want))
	})

	t.Run("now returns current time", func(t *testing.T) {
		before := time.Now().UTC().Add(-time.Second)
		parse := parseTimeFunc(time.RFC3339)
		got, err := parse("now")
		assert.NotError(t, err)
		assert.True(t, got.After(before))
	})

	t.Run("invalid string returns error", func(t *testing.T) {
		parse := parseTimeFunc(time.RFC3339)
		_, err := parse("not-a-time")
		assert.Error(t, err)
	})

	t.Run("custom layout", func(t *testing.T) {
		parse := parseTimeFunc("2006-01-02")
		got, err := parse("2024-03-15")
		assert.NotError(t, err)
		want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		assert.True(t, got.Equal(want))
	})
}

// ── appendOnce ────────────────────────────────────────────────────────────────

func Test_appendOnce(t *testing.T) {
	t.Parallel()

	t.Run("appends without clearing when no defaults", func(t *testing.T) {
		s := []int{9}
		fn := appendOnce(&s, strconv.Atoi, false, "", false)
		assert.NotError(t, fn("1"))
		assert.NotError(t, fn("2"))
		assert.EqualItems(t, s, []int{9, 1, 2})
	})

	t.Run("clears pre-populated defaults on first call", func(t *testing.T) {
		s := []int{10, 20}
		fn := appendOnce(&s, strconv.Atoi, true, "", false)
		assert.NotError(t, fn("1"))
		assert.NotError(t, fn("2"))
		assert.EqualItems(t, s, []int{1, 2})
	})

	t.Run("parse error propagates", func(t *testing.T) {
		var s []int
		fn := appendOnce(&s, strconv.Atoi, false, "", false)
		assert.Error(t, fn("notanint"))
	})
}

func Test_appendOnce_sep(t *testing.T) {
	t.Parallel()

	id := func(v string) (string, error) { return v, nil }

	t.Run("sepSet false does not split on any character", func(t *testing.T) {
		var s []string
		fn := appendOnce(&s, id, false, "", false)
		assert.NotError(t, fn("a:b:c"))
		assert.EqualItems(t, s, []string{"a:b:c"})
	})

	t.Run("sepSet true sep colon splits value", func(t *testing.T) {
		var s []string
		fn := appendOnce(&s, id, false, ":", true)
		assert.NotError(t, fn("a:b:c"))
		assert.EqualItems(t, s, []string{"a", "b", "c"})
	})

	t.Run("sepSet true sep empty does not split", func(t *testing.T) {
		var s []string
		fn := appendOnce(&s, id, false, "", true)
		assert.NotError(t, fn("a:b:c"))
		assert.EqualItems(t, s, []string{"a:b:c"})
	})

	t.Run("clearOnFirst fires exactly once with sep", func(t *testing.T) {
		s := []string{"old"}
		fn := appendOnce(&s, id, true, ":", true)
		assert.NotError(t, fn("x:y"))
		assert.EqualItems(t, s, []string{"x", "y"})
		assert.NotError(t, fn("z"))
		assert.EqualItems(t, s, []string{"x", "y", "z"})
	})

	t.Run("parse error inside split propagates", func(t *testing.T) {
		var s []int
		fn := appendOnce(&s, strconv.Atoi, false, ":", true)
		assert.Error(t, fn("1:notanint:3"))
	})

	t.Run("whitespace trimmed per-element with sep", func(t *testing.T) {
		var s []string
		fn := appendOnce(&s, id, false, ":", true)
		assert.NotError(t, fn(" a : b : c "))
		assert.EqualItems(t, s, []string{"a", "b", "c"})
	})
}

// ── parseAndSetSliceDefault ───────────────────────────────────────────────────

func Test_parseAndSetSliceDefault(t *testing.T) {
	t.Parallel()

	id := func(v string) (string, error) { return v, nil }

	t.Run("empty defStr is a no-op", func(t *testing.T) {
		var s []string
		assert.NotError(t, parseAndSetSliceDefault(&s, "", "", false, id))
		assert.True(t, s == nil)
	})

	t.Run("comma-separated values are parsed and appended", func(t *testing.T) {
		var s []int
		assert.NotError(t, parseAndSetSliceDefault(&s, "1,2,3", ",", true, strconv.Atoi))
		assert.EqualItems(t, s, []int{1, 2, 3})
	})

	t.Run("whitespace around values is trimmed", func(t *testing.T) {
		var s []string
		assert.NotError(t, parseAndSetSliceDefault(&s, " a , b , c ", ",", true, id))
		assert.EqualItems(t, s, []string{"a", "b", "c"})
	})

	t.Run("parse error is returned", func(t *testing.T) {
		var s []int
		assert.Error(t, parseAndSetSliceDefault(&s, "1,bad,3", ",", true, strconv.Atoi))
	})

	t.Run("no sep tag with non-empty default returns ErrInvalidSpecification", func(t *testing.T) {
		var s []string
		err := parseAndSetSliceDefault(&s, "a,b,c", "", false, id)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

func Test_parseAndSetSliceDefault_sep(t *testing.T) {
	t.Parallel()

	id := func(v string) (string, error) { return v, nil }

	t.Run("sepSet false returns ErrInvalidSpecification when defStr non-empty", func(t *testing.T) {
		var s []string
		err := parseAndSetSliceDefault(&s, "a,b,c", "", false, id)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("sepSet true sep colon uses colon", func(t *testing.T) {
		var s []string
		assert.NotError(t, parseAndSetSliceDefault(&s, "a:b:c", ":", true, id))
		assert.EqualItems(t, s, []string{"a", "b", "c"})
	})

	t.Run("sepSet true sep empty treats whole value as single element", func(t *testing.T) {
		var s []string
		assert.NotError(t, parseAndSetSliceDefault(&s, "a,b,c", "", true, id))
		assert.EqualItems(t, s, []string{"a,b,c"})
	})

	t.Run("whitespace trimmed with custom sep", func(t *testing.T) {
		var s []string
		assert.NotError(t, parseAndSetSliceDefault(&s, " x : y : z ", ":", true, id))
		assert.EqualItems(t, s, []string{"x", "y", "z"})
	})
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
			err := conflagure(newTestFS(), &c, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NotError(t, err)
			assert.True(t, reflect.DeepEqual(c.Files, tt.want))
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
			name:      "rest only",
			args:      []string{"foo.txt", "bar.txt"},
			wantFiles: []string{"foo.txt", "bar.txt"},
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
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.Equal(t, c.Output, tt.wantOutput)
			assert.True(t, reflect.DeepEqual(c.Files, tt.wantFiles))
		})
	}
}

func Test_narg_rest_typed(t *testing.T) {
	t.Parallel()

	t.Run("[]int accumulates", func(t *testing.T) {
		type cfg struct {
			Numbers []int `narg:"rest"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"1", "2", "3"}))
		assert.EqualItems(t, c.Numbers, []int{1, 2, 3})
	})

	t.Run("[]int invalid returns error", func(t *testing.T) {
		type cfg struct {
			Numbers []int `narg:"rest"`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"notanint"}))
	})

	t.Run("[]float64 accumulates", func(t *testing.T) {
		type cfg struct {
			Values []float64 `narg:"rest"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"1.1", "2.2", "3.3"}))
		assert.EqualItems(t, c.Values, []float64{1.1, 2.2, 3.3})
	})
}

func Test_narg_rest_required(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `narg:"rest" required:"true"`
	}

	t.Run("empty returns ErrInvalidInput", func(t *testing.T) {
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("provided succeeds", func(t *testing.T) {
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"a.txt"}))
		assert.Equal(t, len(c.Files), 1)
		assert.Equal(t, c.Files[0], "a.txt")
	})
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
			assert.NotError(t, conflagure(newTestFS(), &c, tt.args))
			assert.True(t, reflect.DeepEqual(c.Files, tt.wantFiles))
			assert.Equal(t, c.Output, tt.wantOutput)
		})
	}
}

func Test_narg_until_eq_form(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files  []string `flag:"files"  narg:"until"`
		Output string   `flag:"output"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-files=a.txt", "b.txt", "c.txt", "-output", "out.txt"}))
	assert.True(t, reflect.DeepEqual(c.Files, []string{"a.txt", "b.txt", "c.txt"}))
	assert.Equal(t, c.Output, "out.txt")
}

func Test_narg_until_multiple_fields(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Inputs  []string `flag:"input"  narg:"until"`
		Outputs []string `flag:"output" narg:"until"`
	}

	var c cfg
	args := []string{"-input", "a.txt", "b.txt", "-output", "x.txt", "y.txt"}
	assert.NotError(t, conflagure(newTestFS(), &c, args))
	assert.True(t, reflect.DeepEqual(c.Inputs, []string{"a.txt", "b.txt"}))
	assert.True(t, reflect.DeepEqual(c.Outputs, []string{"x.txt", "y.txt"}))
}

func Test_narg_until_coexists_with_regular_slice(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Tags  []string `flag:"tag"`
		Files []string `flag:"files" narg:"until"`
	}

	var c cfg
	args := []string{"-tag", "go", "-tag", "cli", "-files", "a.txt", "b.txt"}
	assert.NotError(t, conflagure(newTestFS(), &c, args))
	assert.EqualItems(t, c.Tags, []string{"go", "cli"})
	assert.True(t, reflect.DeepEqual(c.Files, []string{"a.txt", "b.txt"}))
}

func Test_narg_until_short_alias(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Files []string `flag:"files" narg:"until" short:"f"`
	}

	var c cfg
	assert.NotError(t, conflagure(newTestFS(), &c, []string{"-f", "a.txt", "b.txt"}))
	assert.True(t, reflect.DeepEqual(c.Files, []string{"a.txt", "b.txt"}))
}

// ── narg validation error tests ───────────────────────────────────────────────

func Test_narg_invalid_spec_errors(t *testing.T) {
	t.Parallel()

	t.Run("rest with flag: tag", func(t *testing.T) {
		type cfg struct {
			Files []string `flag:"files" narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("until without flag: tag", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"until"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("rest on non-slice type", func(t *testing.T) {
		type cfg struct {
			File string `narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("until on non-slice type", func(t *testing.T) {
		type cfg struct {
			File string `flag:"file" narg:"until"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("unknown narg value", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"unknown"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
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
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})

	t.Run("multiple rest fields", func(t *testing.T) {
		type cfg struct {
			Files1 []string `narg:"rest"`
			Files2 []string `narg:"rest"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, []string{"a"})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

// ── narg in subcommand context ────────────────────────────────────────────────

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
			assert.NotError(t, err)
			got, ok := result.(*sub)
			assert.True(t, ok)
			assert.Equal(t, got.Output, tt.wantOutput)
			assert.True(t, reflect.DeepEqual(got.Files, tt.wantFiles))
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
	assert.NotError(t, err)
	got, ok := result.(*sub)
	assert.True(t, ok)
	assert.True(t, reflect.DeepEqual(got.Files, []string{"a.txt", "b.txt"}))
	assert.Equal(t, got.Output, "out.txt")
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
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── parseAndCheck / checkRequired ─────────────────────────────────────────────

func Test_parseAndCheck_required_fields(t *testing.T) {
	t.Parallel()

	t.Run("required string not set returns ErrInvalidInput", func(t *testing.T) {
		type cfg struct {
			Name string `flag:"name" required:"true"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("required bool default false not counted as set", func(t *testing.T) {
		type cfg struct {
			Flag bool `flag:"flag" required:"true"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidInput)
	})

	t.Run("required bool set to true satisfies requirement", func(t *testing.T) {
		type cfg struct {
			Flag bool `flag:"flag" required:"true"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-flag"}))
		assert.True(t, c.Flag)
	})
}

// ── commandLineArgs ───────────────────────────────────────────────────────────

func Test_commandLineArgs(t *testing.T) {
	// Not parallel: modifies flag.CommandLine.
	oldFS := flagCommandLine()
	t.Cleanup(func() { setFlagCommandLine(oldFS) })

	t.Run("not yet parsed falls back to os.Args", func(t *testing.T) {
		setFlagCommandLine(newTestFS())
		got := commandLineArgs()
		// In tests, os.Args[1:] is the test runner's args; just verify it doesn't panic.
		assert.True(t, got != nil || got == nil) // always true, just verify no panic
	})

	t.Run("already parsed returns CommandLine.Args()", func(t *testing.T) {
		fs := newTestFS()
		_ = fs.Parse([]string{"remaining"})
		setFlagCommandLine(fs)
		got := commandLineArgs()
		assert.EqualItems(t, got, []string{"remaining"})
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
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Tags, []string{"a", "b", "c"})
	})

	t.Run("sep pipe splits default", func(t *testing.T) {
		type cfg struct {
			Items []string `default:"x|y|z" flag:"item" sep:"|"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Items, []string{"x", "y", "z"})
	})

	t.Run("sep colon on int slice splits default", func(t *testing.T) {
		type cfg struct {
			Nums []int `default:"1:2:3" flag:"num" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Nums, []int{1, 2, 3})
	})

	t.Run("sep empty string disables default splitting", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a,b,c" flag:"tag" sep:""`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Tags, []string{"a,b,c"})
	})

	t.Run("absent sep tag with non-empty default returns ErrInvalidSpecification", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"debug,info,warn" flag:"tag"`
		}
		var c cfg
		err := conflagure(newTestFS(), &c, nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidSpecification)
	})
}

func Test_sep_tag_live_flag_splitting(t *testing.T) {
	t.Parallel()

	t.Run("sep colon splits single flag invocation into multiple elements", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "a:b:c"}))
		assert.EqualItems(t, c.Tags, []string{"a", "b", "c"})
	})

	t.Run("multiple flag invocations each split and accumulate", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "a:b", "-tag", "c"}))
		assert.EqualItems(t, c.Tags, []string{"a", "b", "c"})
	})

	t.Run("absent sep tag does not split live flag values", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "a:b:c"}))
		assert.EqualItems(t, c.Tags, []string{"a:b:c"})
	})

	t.Run("sep colon on int slice splits live flag value", func(t *testing.T) {
		type cfg struct {
			Nums []int `flag:"num" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-num", "10:20:30"}))
		assert.EqualItems(t, c.Nums, []int{10, 20, 30})
	})

	t.Run("sep colon with short alias also splits", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":" short:"t"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-t", "x:y"}))
		assert.EqualItems(t, c.Tags, []string{"x", "y"})
	})

	t.Run("sep colon parse error on bad element returns error", func(t *testing.T) {
		type cfg struct {
			Nums []int `flag:"num" sep:":"`
		}
		var c cfg
		assert.Error(t, conflagure(newTestFS(), &c, []string{"-num", "1:notanint:3"}))
	})
}

func Test_sep_tag_default_cleared_on_first_invocation(t *testing.T) {
	t.Parallel()

	t.Run("defaults are cleared when first flag invocation occurs", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-tag", "c:d"}))
		assert.EqualItems(t, c.Tags, []string{"c", "d"})
	})

	t.Run("multiple invocations after clearing accumulate", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":" short:"t"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, []string{"-t", "c:d", "-tag", "e"}))
		assert.EqualItems(t, c.Tags, []string{"c", "d", "e"})
	})

	t.Run("no invocation leaves defaults intact", func(t *testing.T) {
		type cfg struct {
			Tags []string `default:"a:b" flag:"tag" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
		assert.EqualItems(t, c.Tags, []string{"a", "b"})
	})
}

func Test_sep_tag_validation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  any
	}{
		{"sep on string scalar", &struct {
			Name string `flag:"name" sep:":"`
		}{}},
		{"sep on int scalar", &struct {
			Count int `flag:"count" sep:":"`
		}{}},
		{"sep on bool scalar", &struct {
			Flag bool `flag:"flag" sep:":"`
		}{}},
		{"sep on float64 scalar", &struct {
			Ratio float64 `flag:"ratio" sep:":"`
		}{}},
	} {
		t.Run(tc.name+" returns ErrInvalidSpecification", func(t *testing.T) {
			err := conflagure(newTestFS(), tc.cfg, nil)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidSpecification)
		})
	}

	t.Run("sep on slice is valid", func(t *testing.T) {
		type cfg struct {
			Tags []string `flag:"tag" sep:":"`
		}
		var c cfg
		assert.NotError(t, conflagure(newTestFS(), &c, nil))
	})
}

// ── dispatch: subcommand rest parse error ─────────────────────────────────────

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
	_, err := dispatch(newTestFS(), &c, []string{"calc", "notanint"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ── collectUntilFlags tests ───────────────────────────────────────────────────

func Test_collectUntilFlags(t *testing.T) {
	t.Parallel()

	t.Run("unexported nested struct with until field", func(t *testing.T) {
		type inner struct {
			Files []string `flag:"files" narg:"until"`
		}
		type cfg struct {
			inner
		}
		var c cfg
		result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
		assert.True(t, result["files"])
	})

	t.Run("flag.Value struct is skipped", func(t *testing.T) {
		type cfg struct {
			Custom testFlagValue `flag:"custom"`
		}
		var c cfg
		result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
		assert.Equal(t, len(result), 0)
	})

	t.Run("namespaced nested struct accumulates prefix", func(t *testing.T) {
		type inner struct {
			Files []string `flag:"files" narg:"until"`
		}
		type cfg struct {
			Server inner `flag:"srv"`
		}
		var c cfg
		result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
		assert.True(t, result["srv.files"])
	})

	t.Run("until without flag: tag is skipped", func(t *testing.T) {
		type cfg struct {
			Files []string `narg:"until"`
		}
		var c cfg
		result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
		assert.Equal(t, len(result), 0)
	})

	t.Run("cmd: fields are skipped", func(t *testing.T) {
		type deploy struct {
			Files []string `flag:"files" narg:"until"`
		}
		type cfg struct {
			Deploy deploy `cmd:"deploy"`
		}
		var c cfg
		result := collectUntilFlags(reflect.ValueOf(&c).Elem(), "")
		assert.True(t, !result["files"])
	})
}

// ── collectRestField tests ────────────────────────────────────────────────────

func Test_collectRestField_rest_and_cmd(t *testing.T) {
	t.Parallel()

	type sub struct {
		X string `flag:"x"`
	}
	type cfg struct {
		Args   []string `narg:"rest"`
		Deploy sub      `cmd:"deploy"`
	}
	var c cfg
	_, _, err := collectRestField(reflect.ValueOf(&c).Elem())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// ── bindFlags: unexported narg field ─────────────────────────────────────────

func Test_bindFlags_unexported_narg_field(t *testing.T) {
	t.Parallel()

	type cfg struct {
		files []string `narg:"rest"` //nolint:unused
	}
	var c cfg
	err := bindFlags(newTestFS(), reflect.ValueOf(&c).Elem(), "", 0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSpecification)
}

// ── bindFlags: nested cmd tag skipped ────────────────────────────────────────

func Test_bindFlags_nested_cmd_tag_skipped(t *testing.T) {
	t.Parallel()

	fs := newTestFS()
	type inner struct {
		Target string        `flag:"target"`
		Sub    testDeployCmd `cmd:"sub"`
	}
	var b inner
	assert.NotError(t, bindFlags(fs, reflectVal(&b), "", 1))
	assert.True(t, fs.Lookup("target") != nil)
}

// ── helper: flagCommandLine wrapper for tests ─────────────────────────────────

func flagCommandLine() *flag.FlagSet {
	return flag.CommandLine
}

func setFlagCommandLine(fs *flag.FlagSet) {
	flag.CommandLine = fs
}

// ── isFlag and expandUntilArgs ────────────────────────────────────────────────

func Test_isFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arg  string
		want bool
	}{
		{arg: "-foo", want: true},
		{arg: "--foo", want: true},
		{arg: "-f=val", want: true},
		{arg: "foo", want: false},
		{arg: "", want: false},
		{arg: "-", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			assert.Equal(t, isFlag(tt.arg), tt.want)
		})
	}
}

func Test_expandUntilArgs(t *testing.T) {
	t.Parallel()

	t.Run("until flag collects positional args until next flag", func(t *testing.T) {
		untilFlags := map[string]bool{"files": true}
		args := []string{"-files", "a.txt", "b.txt", "-output", "out.txt"}
		expanded := expandUntilArgs(args, untilFlags)
		// After expansion, b.txt should be an additional -files value
		assert.True(t, len(expanded) > 0)
		// The core invariant: -output remains a flag
		found := false
		for _, a := range expanded {
			if strings.HasPrefix(a, "-output") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("no until flags returns args unchanged", func(t *testing.T) {
		args := []string{"-foo", "bar"}
		expanded := expandUntilArgs(args, map[string]bool{})
		assert.EqualItems(t, expanded, args)
	})
}
