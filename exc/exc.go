// Package exc provides a thin, ergonomic wrapper around the standard library's
// 'os/exec' package.
package exc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/strut"
)

// Command describes an external process. All fields are public and may
// be set directly, or through the fluent builder methods. Always set, at
// least, 'Name' before calling 'Run', 'Exec, or 'Start'.
//
// The methods all return the same *Command pointer, so calls can be
// chained: new(Command).WithName("echo").WithArgs("hello").
type Command struct {
	ID        string
	Labels    dt.OrderedSet[string]
	Name      string
	Args      []string
	Env       dt.OrderedMap[string, string]
	Directory string
	Input     io.Reader
	Output    io.Writer
	Error     io.Writer
}

// WithID sets the user-supplied identifier for the command and returns
// the receiver. The ID is not passed to the underlying process; it is
// used only for logging and formatting.
func (cmd *Command) WithID(id string) *Command { cmd.ID = id; return cmd }

// WithLabel adds label to the command's label set and returns the receiver.
func (cmd *Command) WithLabel(label string) *Command { cmd.Labels.Add(label); return cmd }

// HasLabel reports whether label is in the command's label set.
func (cmd *Command) HasLabel(label string) bool { return cmd.Labels.Check(label) }

// ResetLabels removes all labels from the command and returns the receiver.
func (cmd *Command) ResetLabels() *Command {
	for l := range cmd.Labels.Iterator() {
		cmd.Labels.Delete(l)
	}
	return cmd
}

// Format implements fmt.Formatter. For %v and %+v the exc.Command<[+ID][+LABELS] command>. %q
// encloses with brackets.)
func (cmd *Command) Format(f fmt.State, verb rune) {
	var b strut.Buffer
	switch verb {
	case 'v':
		b.WriteString("exc.Command<")
		if f.Flag('+') {
			if cmd.ID != "" {
				b.Concat("[", cmd.ID, "]")
			}
			if cmd.Labels.Len() > 0 {
				b.WriteByte('[')
				b.Join(slices.Sorted(cmd.Labels.Iterator()), ",")
				b.WriteByte(']')
			}
		}
		b.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		b.WriteByte('>')
	case 'q':
		b.WriteByte('<')
		b.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		b.WriteByte('>')
	case 's':
		fallthrough
	default:
		b.Join(append([]string{cmd.Name}, cmd.Args...), " ")
	}
	_, _ = b.WriteTo(f)
}

// Shell runs code using the specified shell (or path to shell
// binary,) and runs the 'block' accordingly (via the '-c'
// option). Usable with bash/zsh/dash/sh/nu/fish/python/python3.
func (cmd *Command) Shell(shell string, block string) *Command {
	return cmd.WithName(shell).WithArgs("-c", block)
}

// SSH sets the command to ssh and passes host followed by args
// directly to the ssh binary. host may include a user prefix
// (user@host). args may contain ssh options, a remote command,
// or both, in the order ssh expects them.
func (cmd *Command) SSH(host string, args ...string) *Command {
	return cmd.WithName("ssh").WithArgs(append([]string{host}, args...)...)
}

// Clone returns a new Command with all fields copied. The Args slice
// is cloned so mutations to one do not affect the other. Input,
// Output, and Error share the same underlying io.Reader/Writer
// values as the original, since those cannot be meaningfully copied.
func (cmd *Command) Clone() *Command {
	out := &Command{
		ID:        cmd.ID,
		Name:      cmd.Name,
		Directory: cmd.Directory,
		Input:     cmd.Input,
		Output:    cmd.Output,
		Error:     cmd.Error,
		Args:      append(make([]string, 0, len(cmd.Args)), cmd.Args...),
	}
	out.Env.Extend(cmd.Env.Iterator())
	out.Labels.Extend(cmd.Labels.Iterator())

	return out
}

// WithName sets the executable name or path and returns the receiver.
func (cmd *Command) WithName(n string) *Command { cmd.Name = n; return cmd }

// WithDirectory sets the working directory for the process and returns the receiver.
func (cmd *Command) WithDirectory(d string) *Command { cmd.Directory = d; return cmd }

// SetEnvVar adds or replaces an environment variable in the command's explicit
// environment and returns the receiver. Variables set here are passed to the
// process instead of inheriting the parent environment.
func (cmd *Command) SetEnvVar(k, v string) *Command { cmd.Env.Set(k, v); return cmd }

// UnsetEnvVar removes an environment variable from the command's explicit
// environment and returns the receiver.
func (cmd *Command) UnsetEnvVar(k string) *Command { cmd.Env.Delete(k); return cmd }

// ResentEnv clears all explicitly set environment variables and returns the
// receiver. After this call the process inherits the parent environment.
func (cmd *Command) ResentEnv() *Command { irt.Apply(cmd.Env.Keys(), cmd.Env.Delete); return cmd }

// WithStdInput sets the reader that is connected to the process's stdin and
// returns the receiver. Pass nil to use no stdin.
func (cmd *Command) WithStdInput(r io.Reader) *Command { cmd.Input = r; return cmd }

// WithStdOutput sets the writer that receives the process's stdout and returns
// the receiver. Pass nil to discard stdout.
func (cmd *Command) WithStdOutput(w io.Writer) *Command { cmd.Output = w; return cmd }

// WithStdError sets the writer that receives the process's stderr and returns
// the receiver. Pass nil to discard stderr.
func (cmd *Command) WithStdError(w io.Writer) *Command { cmd.Error = w; return cmd }

// ResetStdInput clears the stdin reader and returns the receiver.
func (cmd *Command) ResetStdInput() *Command { cmd.Input = nil; return cmd }

// ResetStdOutput clears the stdout writer and returns the receiver.
func (cmd *Command) ResetStdOutput() *Command { cmd.Output = nil; return cmd }

// ResetStdError clears the stderr writer and returns the receiver.
func (cmd *Command) ResetStdError() *Command { cmd.Output = nil; return cmd }

// ResetIO clears stdin, stdout, and stderr and returns the receiver.
func (cmd *Command) ResetIO() *Command { cmd.Input, cmd.Output, cmd.Error = nil, nil, nil; return cmd }

// ResetArgs clears the argument list and returns the receiver.
func (cmd *Command) ResetArgs() *Command { cmd.Args = nil; return cmd }

// SetArgs replaces the argument list with a and returns the receiver.
func (cmd *Command) SetArgs(a []string) *Command { cmd.Args = a; return cmd }

// WithArgs replaces the argument list with the provided values and returns the
// receiver.
func (cmd *Command) WithArgs(a ...string) *Command { cmd.Args = a; return cmd }

// Run executes the command, waits for it to finish, and returns any error.
// stdout and stderr are routed to cmd.Output and cmd.Error respectively, or
// discarded if those fields are nil. Use Exec to capture output as a reader,
// or Start for non-blocking execution.
func (cmd *Command) Run(ctx context.Context) error { return cmd.Resolve(ctx).Run() }

// Worker provides access to the command as an fnx.Worker function.
func (cmd *Command) Worker() fnx.Worker { return cmd.Run }

// Exec runs the command and returns its stdout as a reader. Both stdout and
// stderr are buffered internally. If the process fails, Exec returns a nil
// reader and an *Error containing the captured stderr, stdout, and the
// underlying exec error. On success the returned reader contains stdout.
func (cmd *Command) Exec(ctx context.Context) (io.Reader, error) {
	bufout, buferr := new(bytes.Buffer), new(bytes.Buffer)
	cc := cmd.WithStdError(bufio.NewWriter(buferr)).
		WithStdOutput(bufio.NewWriter(bufout)).
		Resolve(ctx)
	if err := cc.Run(); err != nil {
		return nil, &Error{Name: cmd.Name, StdError: buferr, StdOutput: bufout, Err: err}
	}
	return bufio.NewReader(bufout), nil
}

// Error holds the diagnostic information from a failed Exec call.
type Error struct {
	StdError  *bytes.Buffer
	StdOutput *bytes.Buffer
	Name      string
	Err       error
}

// Is reports whether the underlying error matches other, delegating to
// errors.Is. This lets errors.Is(extrErr, target) traverse the wrapped chain
// inside Err without callers needing to unwrap manually.
func (e *Error) Is(other error) bool { return errors.Is(e.Err, other) }

// As finds the first value in the underlying error's chain that matches target
// and sets target to that value, delegating to errors.As. This lets
// errors.As(extrErr, &target) extract typed errors from inside Err.
func (e *Error) As(other any) bool { return errors.As(e.Err, other) }

// Unwrap returns the underlying error so that errors.Is and errors.As can
// traverse it. Returns nil when no underlying error is set.
func (e *Error) Unwrap() error { return e.Err }

// Error returns the string summary of the error including the name, the
// underlying error, and the captured stderr output.
func (e *Error) Error() string { return fmt.Sprintf("[%s] got %v: err=%q", e.Name, e.Err, e.StdError) }

// ResolveError extracts an *Error from err if one appears anywhere in the
// error chain. Returns the *Error and true on success, or nil and false if err
// is nil or contains no *Error.
func ResolveError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Start launches the command asynchronously and returns a fnx.Worker that
// blocks until the process exits or the worker's context is cancelled. If the
// command fails to start, a worker that immediately returns the start error is
// returned. Cancelling the context passed to Start terminates the process;
// cancelling the context passed to the returned worker unblocks the caller
// without killing the process.
func (cmd *Command) Start(ctx context.Context) fnx.Worker {
	cc := cmd.Resolve(ctx)
	if err := cc.Start(); err != nil {
		return fnx.MakeWorker(func() error { return err })
	}

	return func(ctx context.Context) error {
		select {
		case err := <-fnx.MakeWorker(cc.Wait).Signal(ctx):
			return err
		case <-ctx.Done():
			return cc.Cancel()
		}
	}
}

// Resolve builds an *exec.Cmd from the Command fields without running it.
// Useful when you need direct access to the underlying exec.Cmd, for example
// to set additional fields before calling Start or Run yourself.
func (cmd *Command) Resolve(ctx context.Context) *exec.Cmd {
	cc := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	cc.Dir = cmd.Directory
	cc.Env = cmd.materializeEnv()
	cc.Stderr = cmd.Error
	cc.Stdout = cmd.Output
	cc.Stdin = cmd.Input
	return cc
}

func (cmd *Command) materializeEnv() []string {
	if cmd.Env.Len() == 0 {
		return nil
	}
	out := make([]string, 0, cmd.Env.Len())
	for k, v := range cmd.Env.Iterator() {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
