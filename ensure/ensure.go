package ensure

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ensure/is"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/irt"
)

// Assertion values are produced by the ensure.That() constructor, and
// are a chainable interface for annotating rich assertions, as a
// complement to the fun/assert and fun/assert/check libraries. Use
// ensure.That() forms to create self-documenting tests.
type Assertion struct {
	// results collection
	check    adt.Once[[]string]
	messages dt.List[fn.Future[string]]

	// subtest options
	subtests     dt.List[*Assertion]
	stname       string
	constructing bool

	// options
	continueOnError bool
	alwaysLog       bool
}

// That constructs a new Assertion with the provided check, as in:
//
//	ensure.That(is.True(1 == 1)).Run(t)
func That(that is.That) *Assertion { return (&Assertion{}).That(that) }

// That sets a new assertion handler function, overriding
// (potentially) the assertion created when the Assertion was
// created. If the assertion has been run, this operation is always a
// noop.
func (a *Assertion) That(t is.That) *Assertion { a.check.Set(t); return a }

// Fatal means that the assertion has "abort-on-error"
// semantics, and will cause the test to fail when the assertion
// fails. If there are subtests, they will always run before the test
// aborts.
//
// This is the default mode for Assertions.
func (a *Assertion) Fatal() *Assertion { a.continueOnError = false; return a }

// Error uses t.Error or b.Error assertions, which mean that the
// assertion is "continue-on-error" and will not abort the execution
// of the test in the case of a failure.
func (a *Assertion) Error() *Assertion { a.continueOnError = true; return a }

// Verbose toggles the logging behavior to log all messages even when
// the test succeeds.
func (a *Assertion) Verbose() *Assertion { a.alwaysLog = true; return a }

// Quiet toggles the logging behavior to be "quiet" (e.g. only, log in
// the case of that the Assertion fails.) This is the default.
func (a *Assertion) Quiet() *Assertion { a.alwaysLog = false; return a }

// Benchmark creates a sub-benchrmark function, which is suitable for
// use as an argument to b.Run().
func (a *Assertion) Benchmark() func(*testing.B) { return func(b *testing.B) { b.Helper(); a.Run(b) } }

// Test creates a test function, which is suitable for use as an
// argument to t.Run() as a subtest.
func (a *Assertion) Test() func(*testing.T) { return func(t *testing.T) { t.Helper(); a.Run(t) } }

// With creates a subtest that is un-conditionally executed after the
// assertions main check runs.
//
// Subtests are always run after the root of their "parent" tasks, are
// run unconditionally in the order they were added.
//
// It is invalid and will panic if you call .Run() on the assertion
// provided to the With() function.
func (a *Assertion) With(name string, op func(ensure *Assertion)) *Assertion {
	sub := &Assertion{stname: name, constructing: true}
	op(sub)
	sub.constructing = false
	return a.Add(sub)
}

// Add adds an assertion object as a subtest of the root assertion.
func (a *Assertion) Add(sub *Assertion) *Assertion { a.subtests.PushBack(sub); return a }

// Log adds a message that is printed on failure in quiet mode, and
// unconditionally in verbose mode. Operates generally like t.Log() or
// fmt.Sprint().
func (a *Assertion) Log(args ...any) *Assertion {
	a.messages.PushBack(func() string { return fmt.Sprint(args...) })
	return a
}

// Logf adds a message that is printed on failure in quiet mode, and
// unconditionally in verbose mode. Operates like t.Logf or
// fmt.Sprintf.
func (a *Assertion) Logf(tmpl string, args ...any) *Assertion {
	a.messages.PushBack(func() string { return fmt.Sprintf(tmpl, args...) })
	return a
}

// WithMetadata annotates the assertion with additional (structured, keyed) logging.
//
// Each pair is logged as it's own Log statement.
func (a *Assertion) WithMetadata(key string, value any) *Assertion {
	a.messages.PushBack(func() string { return fmt.Sprintf(`%s: "%v"`, key, value) })
	return a
}

// Run rus the test and produces the output.
func (a *Assertion) Run(t testing.TB) {
	erc.InvariantOk(!a.constructing, "cannot execute assertion during construction")
	t.Helper()
	strlogger := func(in string) {
		t.Helper()
		if in != "" {
			t.Log(in)
		}
	}
	t.Cleanup(func() {
		t.Helper()
		if a.alwaysLog || t.Failed() {
			for it := a.messages.Front(); it.Ok(); it = it.Next() {
				if op := it.Value(); op != nil {
					strlogger(op())
				}
			}
		}
	})

	result := &dt.List[string]{}

	if !a.check.Defined() && a.subtests.Len() == 0 {
		result.PushBack("no tests defined")
	}

	result.Extend(irt.Slice(a.check.Resolve()))

	for sub := a.subtests.Front(); sub.Ok(); sub = sub.Next() {
		st := sub.Value()

		switch tt := t.(type) {
		case *testing.T:
			tt.Run(st.stname, func(t *testing.T) { t.Helper(); st.Run(t) })
		case *testing.B:
			tt.Run(st.stname, func(t *testing.B) { t.Helper(); st.Run(t) })
		case testing.TB:
			tt.Errorf("unsupported test %q wrapper %T", st.stname, t)
		}
	}

	if result.Len() > 0 {
		irt.Apply(result.IteratorBack(), strlogger)

		if a.continueOnError {
			t.Fail()
		} else {
			t.FailNow()
		}
	}
}
