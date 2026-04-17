package confl

import (
	"context"
	"errors"
	"flag"
	"reflect"
)

// ── Shared test helpers ───────────────────────────────────────────────────────

// BaseTest is an embedded struct used across multiple test configs to verify
// that anonymous embedding works correctly.
type BaseTest struct {
	DryRun bool `flag:"dry-run" help:"print what would be done without making changes" short:"n"`
}

// newTestFS returns a new FlagSet in ContinueOnError mode, suitable for unit
// tests that do not want to hit os.Exit on unknown flags.
func newTestFS() *flag.FlagSet {
	return flag.NewFlagSet("test", flag.ContinueOnError)
}

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

// ── Subcommand test helper types ──────────────────────────────────────────────

// testDeployCmd is a Commander used across multiple subcommand tests.
type testDeployCmd struct {
	Target    string `flag:"target"  help:"deployment target" required:"true"`
	DryRun    bool   `flag:"dry-run" short:"n"`
	RunCalled bool
}

func (d *testDeployCmd) Run(_ context.Context) error { d.RunCalled = true; return nil }

// testRollbackCmd is a Commander used across multiple subcommand tests.
type testRollbackCmd struct {
	Version   string `flag:"version" required:"true"`
	Force     bool   `flag:"force"`
	RunCalled bool
}

func (r *testRollbackCmd) Run(_ context.Context) error { r.RunCalled = true; return nil }

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
