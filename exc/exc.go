package exc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
)

type Command struct {
	Name      string
	Args      []string
	Env       dt.OrderedMap[string, string]
	Directory string
	Input     io.Reader
	Output    io.Writer
	Error     io.Writer
}

func (cmd *Command) WithName(n string) *Command         { cmd.Name = n; return cmd }
func (cmd *Command) WithDirectory(d string) *Command    { cmd.Directory = d; return cmd }
func (cmd *Command) SetEnvVar(k, v string) *Command     { cmd.Env.Set(k, v); return cmd }
func (cmd *Command) UnsetEnvVar(k string) *Command      { cmd.Env.Delete(k); return cmd }
func (cmd *Command) ResentEnv() *Command                { irt.Apply(cmd.Env.Keys(), cmd.Env.Delete); return cmd }
func (cmd *Command) WithStdInput(r io.Reader) *Command  { cmd.Input = r; return cmd }
func (cmd *Command) WithStdOutput(w io.Writer) *Command { cmd.Output = w; return cmd }
func (cmd *Command) WithStdError(w io.Writer) *Command  { cmd.Output = w; return cmd }
func (cmd *Command) ResetStdInput() *Command            { cmd.Input = nil; return cmd }
func (cmd *Command) ResetStdOutput() *Command           { cmd.Output = nil; return cmd }
func (cmd *Command) ResetStdError() *Command            { cmd.Output = nil; return cmd }
func (cmd *Command) ResetIO() *Command                  { cmd.Input, cmd.Output, cmd.Error = nil, nil, nil; return cmd }
func (cmd *Command) ResetArgs() *Command                { cmd.Args = nil; return cmd }
func (cmd *Command) SetArgs(a []string) *Command        { cmd.Args = a; return cmd }
func (cmd *Command) WithArgs(a ...string) *Command      { cmd.Args = a; return cmd }
func (cmd *Command) Run(ctx context.Context) error      { cc := cmd.Resolve(ctx); return cc.Run() }

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

type Error struct {
	StdError  *bytes.Buffer
	StdOutput *bytes.Buffer
	Name      string
	Err       error
}

func (e *Error) Is(other error) bool { return errors.Is(e.Err, other) }
func (e *Error) As(other any) bool   { return errors.As(e.Err, other) }
func (e *Error) Unwrap() error       { return e.Err }
func (e *Error) Error() string       { return fmt.Sprintf("[%s] got %v: err=%q", e.Name, e.Err, e.StdError) }

// ResolveError extracts an *Error from err if it is one, returning it
// and true. If err is nil or is not an exc.Error, it returns nil and false.
func ResolveError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

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
