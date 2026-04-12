package exc_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/exc"
)

type testCustomErr struct{ msg string }

func (e *testCustomErr) Error() string { return e.msg }

func TestCommand_Builder(t *testing.T) {
	t.Parallel()

	t.Run("FluentMethodsReturnSamePointer", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &exc.Command{}
		if cmd.WithName("true") != cmd {
			t.Error("WithName did not return same pointer")
		}
		if cmd.WithArgs("a") != cmd {
			t.Error("WithArgs did not return same pointer")
		}
		if cmd.WithDirectory("/tmp") != cmd {
			t.Error("WithDirectory did not return same pointer")
		}
		if cmd.WithStdOutput(&buf) != cmd {
			t.Error("WithStdOutput did not return same pointer")
		}
		if cmd.WithStdInput(&buf) != cmd {
			t.Error("WithStdInput did not return same pointer")
		}
		if cmd.ResetArgs() != cmd {
			t.Error("ResetArgs did not return same pointer")
		}
		if cmd.SetArgs([]string{"x"}) != cmd {
			t.Error("SetArgs did not return same pointer")
		}
		if cmd.ResetStdError() != cmd {
			t.Error("ResetStdError did not return same pointer")
		}
		if cmd.ResetIO() != cmd {
			t.Error("ResetIO did not return same pointer")
		}
	})

	t.Run("WithName", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			in   string
		}{
			{name: "simple", in: "echo"},
			{name: "path", in: "/usr/bin/true"},
			{name: "empty", in: ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := &exc.Command{}
				cmd.WithName(tc.in)
				if cmd.Name != tc.in {
					t.Errorf("Name = %q, want %q", cmd.Name, tc.in)
				}
			})
		}
	})

	t.Run("Args", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{name: "none", args: nil},
			{name: "one", args: []string{"a"}},
			{name: "many", args: []string{"a", "b", "c"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := &exc.Command{}
				cmd.WithArgs(tc.args...)
				if !slices.Equal(cmd.Args, tc.args) {
					t.Errorf("Args = %v, want %v", cmd.Args, tc.args)
				}
			})
		}
	})

	t.Run("ResetArgs", func(t *testing.T) {
		cmd := (&exc.Command{}).WithArgs("a", "b")
		cmd.ResetArgs()
		if cmd.Args != nil {
			t.Errorf("Args after reset = %v, want nil", cmd.Args)
		}
	})

	t.Run("SetArgs", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			in   []string
			want []string
		}{
			{name: "nil", in: nil, want: nil},
			{name: "empty", in: []string{}, want: []string{}},
			{name: "populated", in: []string{"x", "y", "z"}, want: []string{"x", "y", "z"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := (&exc.Command{}).WithArgs("old", "args")
				cmd.SetArgs(tc.in)
				if !slices.Equal(cmd.Args, tc.want) {
					t.Errorf("Args = %v, want %v", cmd.Args, tc.want)
				}
			})
		}
	})

	t.Run("EnvVars", func(t *testing.T) {
		cmd := &exc.Command{}
		cmd.SetEnvVar("FOO", "bar")
		cmd.SetEnvVar("BAZ", "qux")
		if cmd.Env.Len() != 2 {
			t.Fatalf("Env.Len() = %d, want 2", cmd.Env.Len())
		}
		cmd.UnsetEnvVar("FOO")
		if cmd.Env.Len() != 1 {
			t.Fatalf("Env.Len() after unset = %d, want 1", cmd.Env.Len())
		}
		cmd.ResentEnv()
		if cmd.Env.Len() != 0 {
			t.Fatalf("Env.Len() after reset = %d, want 0", cmd.Env.Len())
		}
	})

	t.Run("IO", func(t *testing.T) {
		cmd := &exc.Command{}
		var buf bytes.Buffer

		cmd.WithStdInput(&buf)
		if cmd.Input != io.Reader(&buf) {
			t.Error("Input not set correctly")
		}
		cmd.ResetStdInput()
		if cmd.Input != nil {
			t.Error("Input not nil after reset")
		}

		cmd.WithStdOutput(&buf)
		if cmd.Output != io.Writer(&buf) {
			t.Error("Output not set correctly")
		}
		cmd.ResetStdOutput()
		if cmd.Output != nil {
			t.Error("Output not nil after reset")
		}

		// ResetStdError currently clears cmd.Output (not cmd.Error).
		// Setting cmd.Output first makes the reset observable.
		cmd.WithStdOutput(&buf)
		cmd.ResetStdError()
		if cmd.Output != nil {
			t.Error("Output not nil after ResetStdError")
		}

		cmd.WithStdOutput(&buf)
		cmd.ResetIO()
		if cmd.Input != nil || cmd.Output != nil || cmd.Error != nil {
			t.Error("IO fields not nil after ResetIO")
		}
	})
}

func TestCommand_Resolve(t *testing.T) {
	t.Parallel()

	t.Run("Fields", func(t *testing.T) {
		var out, in bytes.Buffer
		cmd := (&exc.Command{}).
			WithName("true").
			WithArgs("--foo").
			WithDirectory("/tmp").
			WithStdOutput(&out).
			WithStdInput(&in)

		cc := cmd.Resolve(context.Background())
		if !strings.HasSuffix(cc.Path, "true") {
			t.Errorf("Path = %q, want suffix 'true'", cc.Path)
		}
		if !slices.Equal(cc.Args[1:], []string{"--foo"}) {
			t.Errorf("Args = %v, want [--foo]", cc.Args[1:])
		}
		if cc.Dir != "/tmp" {
			t.Errorf("Dir = %q, want /tmp", cc.Dir)
		}
		if cc.Stdout != io.Writer(&out) {
			t.Error("Stdout not set correctly")
		}
		if cc.Stdin != io.Reader(&in) {
			t.Error("Stdin not set correctly")
		}
	})

	t.Run("Env", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			setup   func(*exc.Command)
			wantNil bool
			wantEnv []string
		}{
			{
				name:    "NoEnvIsNil",
				setup:   func(*exc.Command) {},
				wantNil: true,
			},
			{
				name:    "SingleVar",
				setup:   func(c *exc.Command) { c.SetEnvVar("KEY", "value") },
				wantEnv: []string{"KEY=value"},
			},
			{
				name: "MultipleVars",
				setup: func(c *exc.Command) {
					c.SetEnvVar("A", "1")
					c.SetEnvVar("B", "2")
				},
				wantEnv: []string{"A=1", "B=2"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := &exc.Command{Name: "true"}
				tc.setup(cmd)
				cc := cmd.Resolve(context.Background())
				if tc.wantNil {
					if cc.Env != nil {
						t.Errorf("Env = %v, want nil", cc.Env)
					}
					return
				}
				for _, want := range tc.wantEnv {
					if !slices.Contains(cc.Env, want) {
						t.Errorf("Env %v does not contain %q", cc.Env, want)
					}
				}
			})
		}
	})
}

func TestCommand_Run(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		cmd     string
		args    []string
		ctx     func() context.Context
		wantErr bool
	}{
		{
			name:    "Success",
			cmd:     "true",
			wantErr: false,
		},
		{
			name:    "Failure",
			cmd:     "false",
			wantErr: true,
		},
		{
			name:    "NonexistentCommand",
			cmd:     "this-command-does-not-exist-anywhere",
			wantErr: true,
		},
		{
			name:    "NonZeroExitCode",
			cmd:     "sh",
			args:    []string{"-c", "exit 42"},
			wantErr: true,
		},
		{
			name: "CanceledContext",
			cmd:  "true",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx()
			}
			cmd := (&exc.Command{Name: tc.cmd}).WithArgs(tc.args...)
			err := cmd.Run(ctx)
			if (err != nil) != tc.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCommand_Exec(t *testing.T) {
	t.Parallel()

	t.Run("Table", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			cmd        string
			args       []string
			wantErr    bool
			wantOutput string
			wantLines  int
		}{
			{
				name:       "CapturesOutput",
				cmd:        "echo",
				args:       []string{"hello world"},
				wantOutput: "hello world",
			},
			{
				name:    "FailureReturnsError",
				cmd:     "false",
				wantErr: true,
			},
			{
				name:    "NonexistentCommand",
				cmd:     "this-command-does-not-exist-anywhere",
				wantErr: true,
			},
			{
				name:      "MultiLineOutput",
				cmd:       "sh",
				args:      []string{"-c", "echo line1; echo line2; echo line3"},
				wantLines: 3,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := (&exc.Command{Name: tc.cmd}).WithArgs(tc.args...)
				r, err := cmd.Exec(context.Background())
				if (err != nil) != tc.wantErr {
					t.Fatalf("Exec() error = %v, wantErr %v", err, tc.wantErr)
				}
				if tc.wantErr {
					if r != nil {
						t.Error("Exec() reader should be nil on error")
					}
					return
				}
				data, readErr := io.ReadAll(r)
				if readErr != nil {
					t.Fatalf("ReadAll: %v", readErr)
				}
				if tc.wantOutput != "" && !strings.Contains(string(data), tc.wantOutput) {
					t.Errorf("output %q does not contain %q", string(data), tc.wantOutput)
				}
				if tc.wantLines > 0 {
					lines := strings.Split(strings.TrimSpace(string(data)), "\n")
					if len(lines) != tc.wantLines {
						t.Errorf("got %d lines, want %d", len(lines), tc.wantLines)
					}
				}
			})
		}
	})

	t.Run("ErrorType", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			cmd           string
			args          []string
			wantNameInMsg bool
		}{
			{
				name:          "NameInError",
				cmd:           "false",
				wantNameInMsg: true,
			},
			{
				name:          "StderrCaptured",
				cmd:           "sh",
				args:          []string{"-c", "echo oops >&2; exit 1"},
				wantNameInMsg: true,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := (&exc.Command{Name: tc.cmd}).WithArgs(tc.args...)
				_, err := cmd.Exec(context.Background())
				if err == nil {
					t.Fatal("expected error")
				}
				var execErr *exc.Error
				if !errors.As(err, &execErr) {
					t.Fatalf("error %T is not *exc.Error", err)
				}
				if execErr.Name != tc.cmd {
					t.Errorf("Error.Name = %q, want %q", execErr.Name, tc.cmd)
				}
				if execErr.Err == nil {
					t.Error("Error.Err should not be nil")
				}
				if execErr.StdError == nil {
					t.Error("Error.StdError should not be nil")
				}
				if execErr.StdOutput == nil {
					t.Error("Error.StdOutput should not be nil")
				}
				if tc.wantNameInMsg && !strings.Contains(err.Error(), tc.cmd) {
					t.Errorf("error message %q does not contain command name %q", err.Error(), tc.cmd)
				}
			})
		}
	})
}

func TestCommand_Start(t *testing.T) {
	t.Parallel()

	t.Run("Table", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			cmd     string
			args    []string
			wantErr bool
		}{
			{name: "Success", cmd: "true", wantErr: false},
			{name: "Failure", cmd: "false", wantErr: true},
			{name: "NonexistentCommand", cmd: "this-command-does-not-exist-anywhere", wantErr: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := (&exc.Command{Name: tc.cmd}).WithArgs(tc.args...)
				worker := cmd.Start(context.Background())
				if worker == nil {
					t.Fatal("Start returned nil worker")
				}
				err := worker(context.Background())
				if (err != nil) != tc.wantErr {
					t.Errorf("worker() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("ContextCancellationCancelsWork", func(t *testing.T) {
		// The context passed to Start controls the process lifetime.
		// Canceling it should terminate a long-running process promptly.
		cmd := (&exc.Command{Name: "sleep"}).WithArgs("60")
		ctx, cancel := context.WithCancel(context.Background())
		worker := cmd.Start(ctx)

		go func() { time.Sleep(20 * time.Millisecond); cancel() }()

		done := make(chan struct{})
		go func() { defer close(done); _ = worker(ctx) }()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("worker did not return after context cancellation")
		}
	})

	t.Run("WorkerContextCancellation", func(t *testing.T) {
		// Canceling the context passed to the worker (not Start) should
		// also unblock the caller without waiting for the process.
		cmd := (&exc.Command{Name: "sleep"}).WithArgs("60")
		worker := cmd.Start(context.Background())

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() { defer close(done); _ = worker(ctx) }()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("worker did not return after worker context expiration")
		}
	})
}

func TestError_Format(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		err         *exc.Error
		wantSubstrs []string
	}{
		{
			name: "NameAndErrAndStderr",
			err: &exc.Error{
				Name:      "mycommand",
				Err:       errors.New("exit status 1"),
				StdError:  bytes.NewBufferString("something went wrong"),
				StdOutput: new(bytes.Buffer),
			},
			wantSubstrs: []string{"mycommand", "exit status 1", "something went wrong"},
		},
		{
			name: "EmptyStderr",
			err: &exc.Error{
				Name:      "cmd",
				Err:       errors.New("exit status 2"),
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			},
			wantSubstrs: []string{"cmd"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			for _, sub := range tc.wantSubstrs {
				if !strings.Contains(msg, sub) {
					t.Errorf("error message %q does not contain %q", msg, sub)
				}
			}
		})
	}
}

func TestError_IsAsUnwrap(t *testing.T) {
	t.Parallel()

	t.Run("Is", func(t *testing.T) {
		sentinel := errors.New("sentinel error")
		unrelated := errors.New("unrelated error")

		for _, tc := range []struct {
			name string
			err  *exc.Error
			tgt  error
			want bool
		}{
			{
				name: "DirectMatch",
				err: &exc.Error{
					Name:      "cmd",
					Err:       sentinel,
					StdError:  new(bytes.Buffer),
					StdOutput: new(bytes.Buffer),
				},
				tgt:  sentinel,
				want: true,
			},
			{
				name: "WrappedMatch",
				err: &exc.Error{
					Name:      "cmd",
					Err:       fmt.Errorf("wrapped: %w", sentinel),
					StdError:  new(bytes.Buffer),
					StdOutput: new(bytes.Buffer),
				},
				tgt:  sentinel,
				want: true,
			},
			{
				name: "NoMatch",
				err: &exc.Error{
					Name:      "cmd",
					Err:       unrelated,
					StdError:  new(bytes.Buffer),
					StdOutput: new(bytes.Buffer),
				},
				tgt:  sentinel,
				want: false,
			},
			{
				name: "NilErr",
				err: &exc.Error{
					Name:      "cmd",
					Err:       nil,
					StdError:  new(bytes.Buffer),
					StdOutput: new(bytes.Buffer),
				},
				tgt:  sentinel,
				want: false,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := errors.Is(tc.err, tc.tgt)
				if got != tc.want {
					t.Errorf("errors.Is(%v, %v) = %v, want %v", tc.err, tc.tgt, got, tc.want)
				}
			})
		}
	})

	t.Run("As", func(t *testing.T) {
		customErrPtr := &testCustomErr{msg: "custom"}

		t.Run("MatchesConcreteType", func(t *testing.T) {
			excErr := &exc.Error{
				Name:      "cmd",
				Err:       customErrPtr,
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}
			var target *testCustomErr
			if !errors.As(excErr, &target) {
				t.Errorf("errors.As returned false, expected true")
			}
			if target != customErrPtr {
				t.Errorf("errors.As populated target = %v, want %v", target, customErrPtr)
			}
		})

		t.Run("NilErr", func(t *testing.T) {
			excErr := &exc.Error{
				Name:      "cmd",
				Err:       nil,
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}
			var target *testCustomErr
			if errors.As(excErr, &target) {
				t.Errorf("errors.As returned true, expected false for nil Err")
			}
		})

		t.Run("WrongType", func(t *testing.T) {
			excErr := &exc.Error{
				Name:      "cmd",
				Err:       errors.New("plain error"),
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}
			var target *testCustomErr
			if errors.As(excErr, &target) {
				t.Errorf("errors.As returned true, expected false for mismatched type")
			}
		})
	})

	t.Run("Unwrap", func(t *testing.T) {
		t.Run("NonNilErr", func(t *testing.T) {
			inner := errors.New("inner error")
			excErr := &exc.Error{
				Name:      "cmd",
				Err:       inner,
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}
			got := excErr.Unwrap()
			if got != inner {
				t.Errorf("Unwrap() = %v, want %v", got, inner)
			}
		})

		t.Run("NilErr", func(t *testing.T) {
			excErr := &exc.Error{
				Name:      "cmd",
				Err:       nil,
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}
			got := excErr.Unwrap()
			if got != nil {
				t.Errorf("Unwrap() = %v, want nil", got)
			}
		})
	})
}

func TestCommand_WithID(t *testing.T) {
	t.Parallel()

	t.Run("SetsID", func(t *testing.T) {
		cmd := (&exc.Command{}).WithID("job-1")
		if cmd.ID != "job-1" {
			t.Errorf("ID = %q, want %q", cmd.ID, "job-1")
		}
	})

	t.Run("ReturnsSamePointer", func(t *testing.T) {
		cmd := &exc.Command{}
		if cmd.WithID("x") != cmd {
			t.Error("WithID did not return same pointer")
		}
	})

	t.Run("Chainable", func(t *testing.T) {
		cmd := (&exc.Command{}).WithID("task").WithName("echo").WithArgs("hi")
		if cmd.ID != "task" || cmd.Name != "echo" {
			t.Errorf("chaining broken: ID=%q Name=%q", cmd.ID, cmd.Name)
		}
	})
}

func TestCommand_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  func() *exc.Command
		format string
		want   string
	}{
		// %v: outputs only with + flag; bare %v produces empty string
		{
			name:   "v_bare_empty",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("echo").WithArgs("hello", "world") },
			format: "%v",
			want:   "exc.Command<echo hello world>",
		},
		// %q: angle-bracket wrapped cmdline
		{
			name:   "q_wrapped",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("echo").WithArgs("hello", "world") },
			format: "%q",
			want:   "<echo hello world>",
		},
		// %s: plain cmdline
		{
			name:   "s_plain",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("ls").WithArgs("-la") },
			format: "%s",
			want:   "ls -la",
		},
		// unknown verb: plain cmdline (same as %s)
		{
			name:   "unknown_verb_plain",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("echo").WithArgs("hi") },
			format: "%d",
			want:   "echo hi",
		},
		// %+v: "exc.Command<[id][labels]cmdline>"
		{
			name:   "plus_v_id_only",
			setup:  func() *exc.Command { return (&exc.Command{}).WithID("job-1").WithName("echo").WithArgs("hi") },
			format: "%+v",
			want:   "exc.Command<[job-1]echo hi>",
		},
		{
			name: "plus_v_labels_only",
			setup: func() *exc.Command {
				return (&exc.Command{}).WithName("echo").WithArgs("hi").SetLabel("prod")
			},
			format: "%+v",
			want:   "exc.Command<[prod]echo hi>",
		},
		{
			name: "plus_v_id_and_labels",
			setup: func() *exc.Command {
				return (&exc.Command{}).WithID("job-1").WithName("echo").WithArgs("hi").SetLabel("prod").SetLabel("web")
			},
			format: "%+v",
			want:   "exc.Command<[job-1][prod,web]echo hi>",
		},
		{
			name:   "plus_v_neither",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("echo").WithArgs("hi") },
			format: "%+v",
			want:   "exc.Command<echo hi>",
		},
		{
			name:   "plus_v_no_args",
			setup:  func() *exc.Command { return (&exc.Command{}).WithName("true") },
			format: "%+v",
			want:   "exc.Command<true>",
		},
		// labels sorted alphabetically regardless of insertion order
		{
			name: "plus_v_labels_sorted",
			setup: func() *exc.Command {
				return (&exc.Command{}).WithName("true").SetLabel("z").SetLabel("a").SetLabel("m")
			},
			format: "%+v",
			want:   "exc.Command<[a,m,z]true>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf(tt.format, tt.setup())
			if got != tt.want {
				t.Errorf("Sprintf(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestCommand_Labels(t *testing.T) {
	t.Parallel()

	t.Run("SetAndHas", func(t *testing.T) {
		cmd := &exc.Command{}
		if cmd.HasLabel("x") {
			t.Error("HasLabel returned true before any labels set")
		}
		result := cmd.SetLabel("x")
		if result != cmd {
			t.Error("SetLabel did not return same pointer")
		}
		if !cmd.HasLabel("x") {
			t.Error("HasLabel returned false after SetLabel")
		}
	})

	t.Run("MultipleLabels", func(t *testing.T) {
		cmd := (&exc.Command{}).SetLabel("a").SetLabel("b").SetLabel("c")
		if !cmd.HasLabel("a") || !cmd.HasLabel("b") || !cmd.HasLabel("c") {
			t.Error("not all labels present after three SetLabel calls")
		}
		if cmd.Labels.Len() != 3 {
			t.Errorf("Labels.Len() = %d, want 3", cmd.Labels.Len())
		}
	})

	t.Run("DuplicateLabel", func(t *testing.T) {
		cmd := (&exc.Command{}).SetLabel("x").SetLabel("x")
		if cmd.Labels.Len() != 1 {
			t.Errorf("Labels.Len() = %d after duplicate SetLabel, want 1", cmd.Labels.Len())
		}
	})

	t.Run("ResetLabels", func(t *testing.T) {
		cmd := (&exc.Command{}).SetLabel("a").SetLabel("b")
		result := cmd.ResetLabels()
		if result != cmd {
			t.Error("ResetLabels did not return same pointer")
		}
		if cmd.Labels.Len() != 0 {
			t.Errorf("Labels.Len() = %d after ResetLabels, want 0", cmd.Labels.Len())
		}
		if cmd.HasLabel("a") || cmd.HasLabel("b") {
			t.Error("labels still present after ResetLabels")
		}
	})

	t.Run("CloneLabelsIndependent", func(t *testing.T) {
		orig := (&exc.Command{}).WithName("true").SetLabel("orig")
		c := orig.Clone()

		if !c.HasLabel("orig") {
			t.Error("clone missing label from original")
		}
		c.SetLabel("clone-only")
		if orig.HasLabel("clone-only") {
			t.Error("label added to clone appeared in original")
		}
		orig.SetLabel("orig-only")
		if c.HasLabel("orig-only") {
			t.Error("label added to original appeared in clone")
		}
	})
}

func TestCommand_Clone(t *testing.T) {
	t.Parallel()

	t.Run("ScalarFieldsCopied", func(t *testing.T) {
		var out bytes.Buffer
		orig := (&exc.Command{}).
			WithID("orig-id").
			WithName("echo").
			WithDirectory("/tmp").
			WithStdOutput(&out)
		orig.Error = &out

		c := orig.Clone()
		if c.ID != orig.ID {
			t.Errorf("ID = %q, want %q", c.ID, orig.ID)
		}
		if c.Name != orig.Name {
			t.Errorf("Name = %q, want %q", c.Name, orig.Name)
		}
		if c.Directory != orig.Directory {
			t.Errorf("Directory = %q, want %q", c.Directory, orig.Directory)
		}
		if c.Output != orig.Output {
			t.Error("Output not copied")
		}
		if c.Input != orig.Input {
			t.Error("Input not copied")
		}
		if c.Error != orig.Error {
			t.Error("Error not copied")
		}
	})

	t.Run("ArgsIndependent", func(t *testing.T) {
		orig := (&exc.Command{}).WithName("echo").WithArgs("a", "b")
		c := orig.Clone()

		// Mutate clone's args; original must be unaffected.
		c.Args[0] = "X"
		if orig.Args[0] != "a" {
			t.Errorf("orig.Args[0] = %q after mutating clone, want %q", orig.Args[0], "a")
		}

		// Mutate original's args; clone must be unaffected.
		orig.Args[1] = "Y"
		if c.Args[1] != "b" {
			t.Errorf("clone.Args[1] = %q after mutating original, want %q", c.Args[1], "b")
		}
	})

	t.Run("EmptyArgsIsEmpty", func(t *testing.T) {
		orig := &exc.Command{Name: "true"}
		c := orig.Clone()
		if len(c.Args) != 0 {
			t.Errorf("Args = %v, want empty", c.Args)
		}
	})

	t.Run("EnvCopied", func(t *testing.T) {
		orig := &exc.Command{}
		orig.SetEnvVar("KEY", "val")
		c := orig.Clone()

		if c.Env.Len() != 1 {
			t.Fatalf("clone Env.Len() = %d, want 1", c.Env.Len())
		}

		// Mutate clone env; original must be unaffected.
		c.SetEnvVar("KEY", "changed")
		if v := orig.Env.Get("KEY"); v != "val" {
			t.Errorf("orig KEY = %q after mutating clone, want %q", v, "val")
		}

		// Add a new key to original; clone must not see it.
		orig.SetEnvVar("NEW", "x")
		if c.Env.Check("NEW") {
			t.Error("clone has NEW key that was added to original after Clone()")
		}
	})

	t.Run("EmptyEnvIndependent", func(t *testing.T) {
		orig := &exc.Command{Name: "true"}
		c := orig.Clone()
		c.SetEnvVar("K", "v")
		if orig.Env.Len() != 0 {
			t.Errorf("orig Env.Len() = %d after adding to clone, want 0", orig.Env.Len())
		}
	})

	t.Run("ReturnsNewPointer", func(t *testing.T) {
		orig := &exc.Command{Name: "true"}
		if orig.Clone() == orig {
			t.Error("Clone() returned the same pointer")
		}
	})

	t.Run("CloneRunsIndependently", func(t *testing.T) {
		var origOut, cloneOut bytes.Buffer
		orig := (&exc.Command{}).WithName("echo").WithArgs("original").WithStdOutput(&origOut)
		c := orig.Clone()
		c.WithArgs("clone").WithStdOutput(&cloneOut)

		if err := orig.Run(context.Background()); err != nil {
			t.Fatalf("orig.Run() error = %v", err)
		}
		if err := c.Run(context.Background()); err != nil {
			t.Fatalf("clone.Run() error = %v", err)
		}
		if !strings.Contains(origOut.String(), "original") {
			t.Errorf("orig output %q does not contain 'original'", origOut.String())
		}
		if !strings.Contains(cloneOut.String(), "clone") {
			t.Errorf("clone output %q does not contain 'clone'", cloneOut.String())
		}
	})
}

func TestCommand_Worker(t *testing.T) {
	t.Parallel()

	t.Run("SuccessfulCommand", func(t *testing.T) {
		cmd := (&exc.Command{}).WithName("true")
		worker := cmd.Worker()
		if worker == nil {
			t.Fatal("Worker() returned nil")
		}
		if err := worker(context.Background()); err != nil {
			t.Errorf("worker() error = %v, want nil", err)
		}
	})

	t.Run("FailingCommand", func(t *testing.T) {
		cmd := (&exc.Command{}).WithName("false")
		if err := cmd.Worker()(context.Background()); err == nil {
			t.Error("worker() expected error for failing command, got nil")
		}
	})

	t.Run("ReturnsSamePointer", func(t *testing.T) {
		// Worker() should return cmd.Run, so calling it twice on the same
		// command should produce workers with equivalent behaviour.
		cmd := (&exc.Command{}).WithName("true")
		w1 := cmd.Worker()
		w2 := cmd.Worker()
		if err := w1(context.Background()); err != nil {
			t.Fatalf("w1() error = %v", err)
		}
		if err := w2(context.Background()); err != nil {
			t.Fatalf("w2() error = %v", err)
		}
	})

	t.Run("MutatingCommandAfterWorkerAffectsWorker", func(t *testing.T) {
		// Worker captures cmd by pointer, so mutations after Worker() is
		// called are visible when the worker runs.
		cmd := &exc.Command{}
		worker := cmd.Worker()
		cmd.WithName("true")
		if err := worker(context.Background()); err != nil {
			t.Errorf("worker() error = %v after mutating cmd to 'true'", err)
		}
	})

	t.Run("CapturesOutput", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := (&exc.Command{}).WithName("echo").WithArgs("hello").WithStdOutput(&buf)
		if err := cmd.Worker()(context.Background()); err != nil {
			t.Fatalf("worker() error = %v", err)
		}
		if !strings.Contains(buf.String(), "hello") {
			t.Errorf("output %q does not contain 'hello'", buf.String())
		}
	})
}

func TestCommand_Shell(t *testing.T) {
	t.Parallel()

	t.Run("SetsNameAndArgs", func(t *testing.T) {
		cmd := &exc.Command{}
		result := cmd.Shell("sh", "echo hi")
		if result != cmd {
			t.Error("Shell() did not return the same pointer")
		}
		if cmd.Name != "sh" {
			t.Errorf("Name = %q, want %q", cmd.Name, "sh")
		}
		if !slices.Equal(cmd.Args, []string{"-c", "echo hi"}) {
			t.Errorf("Args = %v, want [-c echo hi]", cmd.Args)
		}
	})

	t.Run("RunsBlock", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := (&exc.Command{}).WithStdOutput(&buf).Shell("sh", "echo shellout")
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(buf.String(), "shellout") {
			t.Errorf("output %q does not contain 'shellout'", buf.String())
		}
	})

	t.Run("MultiStatementBlock", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := (&exc.Command{}).WithStdOutput(&buf).Shell("sh", "echo a; echo b; echo c")
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3: %v", len(lines), lines)
		}
	})

	t.Run("FailingBlockReturnsError", func(t *testing.T) {
		cmd := (&exc.Command{}).Shell("sh", "exit 1")
		if err := cmd.Run(context.Background()); err == nil {
			t.Error("Run() expected error for failing shell block, got nil")
		}
	})

	t.Run("ReplacesExistingArgs", func(t *testing.T) {
		cmd := (&exc.Command{}).WithArgs("old", "args")
		cmd.Shell("sh", "true")
		if !slices.Equal(cmd.Args, []string{"-c", "true"}) {
			t.Errorf("Args = %v, want [-c true]", cmd.Args)
		}
	})

	t.Run("ExecViaCapturedOutput", func(t *testing.T) {
		cmd := (&exc.Command{}).Shell("sh", "printf '%s' hello")
		r, err := cmd.Exec(context.Background())
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		data, _ := io.ReadAll(r)
		if string(data) != "hello" {
			t.Errorf("output = %q, want %q", string(data), "hello")
		}
	})
}

func TestCommand_SSH(t *testing.T) {
	t.Parallel()

	t.Run("SetsNameAndHost", func(t *testing.T) {
		cmd := &exc.Command{}
		result := cmd.SSH("user@host")
		if result != cmd {
			t.Error("SSH() did not return the same pointer")
		}
		if cmd.Name != "ssh" {
			t.Errorf("Name = %q, want %q", cmd.Name, "ssh")
		}
		if !slices.Equal(cmd.Args, []string{"user@host"}) {
			t.Errorf("Args = %v, want [user@host]", cmd.Args)
		}
	})

	t.Run("HostAndRemoteCommand", func(t *testing.T) {
		cmd := (&exc.Command{}).SSH("user@host", "ls", "-la")
		if !slices.Equal(cmd.Args, []string{"user@host", "ls", "-la"}) {
			t.Errorf("Args = %v, want [user@host ls -la]", cmd.Args)
		}
	})

	t.Run("NoExtraArgs", func(t *testing.T) {
		cmd := (&exc.Command{}).SSH("192.0.2.1")
		if !slices.Equal(cmd.Args, []string{"192.0.2.1"}) {
			t.Errorf("Args = %v, want [192.0.2.1]", cmd.Args)
		}
	})

	t.Run("ReplacesExistingArgs", func(t *testing.T) {
		cmd := (&exc.Command{}).WithArgs("stale", "args")
		cmd.SSH("host", "uptime")
		if !slices.Equal(cmd.Args, []string{"host", "uptime"}) {
			t.Errorf("Args = %v, want [host uptime]", cmd.Args)
		}
	})

	t.Run("ResolvesCorrectly", func(t *testing.T) {
		cmd := (&exc.Command{}).SSH("user@host", "echo", "hello")
		cc := cmd.Resolve(context.Background())
		if !strings.HasSuffix(cc.Path, "ssh") {
			t.Errorf("Path = %q, want suffix 'ssh'", cc.Path)
		}
		if !slices.Equal(cc.Args[1:], []string{"user@host", "echo", "hello"}) {
			t.Errorf("Args = %v, want [user@host echo hello]", cc.Args[1:])
		}
	})

	t.Run("ChainableAfterOtherBuilders", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := (&exc.Command{}).
			WithDirectory("/tmp").
			WithStdOutput(&buf).
			SSH("host", "date")
		if cmd.Name != "ssh" {
			t.Errorf("Name = %q, want %q", cmd.Name, "ssh")
		}
		if cmd.Output != &buf {
			t.Error("Output was cleared by SSH()")
		}
		if cmd.Directory != "/tmp" {
			t.Errorf("Directory = %q, want /tmp", cmd.Directory)
		}
	})
}

func TestResolveError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		err      error
		wantOK   bool
		wantName string
	}{
		{
			name:   "NilError",
			err:    nil,
			wantOK: false,
		},
		{
			name:   "PlainError",
			err:    errors.New("plain error"),
			wantOK: false,
		},
		{
			name: "ExcError",
			err: &exc.Error{
				Name:      "mycmd",
				Err:       errors.New("exit status 1"),
				StdError:  bytes.NewBufferString("stderr"),
				StdOutput: new(bytes.Buffer),
			},
			wantOK:   true,
			wantName: "mycmd",
		},
		{
			name: "WrappedExcError",
			err: fmt.Errorf("outer: %w", &exc.Error{
				Name:      "inner",
				Err:       errors.New("exit status 2"),
				StdError:  new(bytes.Buffer),
				StdOutput: new(bytes.Buffer),
			}),
			wantOK:   true,
			wantName: "inner",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := exc.ResolveError(tc.err)
			if ok != tc.wantOK {
				t.Fatalf("ResolveError() ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if e != nil {
					t.Errorf("ResolveError() returned non-nil error on failure: %v", e)
				}
				return
			}
			if e.Name != tc.wantName {
				t.Errorf("Error.Name = %q, want %q", e.Name, tc.wantName)
			}
		})
	}

	t.Run("FromExecFailure", func(t *testing.T) {
		cmd := &exc.Command{Name: "false"}
		_, err := cmd.Exec(context.Background())
		if err == nil {
			t.Fatal("expected error from false")
		}
		e, ok := exc.ResolveError(err)
		if !ok {
			t.Fatal("ResolveError returned false for exec failure")
		}
		if e.Name != "false" {
			t.Errorf("Error.Name = %q, want %q", e.Name, "false")
		}
		if e.Err == nil {
			t.Error("Error.Err should not be nil")
		}
	})
}
