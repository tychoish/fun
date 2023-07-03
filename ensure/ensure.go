package ensure

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ensure/is"
	"github.com/tychoish/fun/ft"
)

type Assertion struct {
	substestName    *string
	check           adt.Once[*string]
	messages        dt.List[func() string]
	continueOnError bool
	alwaysLog       bool
}

func That(that is.That) *Assertion   { return (&Assertion{}).That(that) }
func Subtest(name string) *Assertion { return (&Assertion{}).Subtest(name) }

func (a *Assertion) Subtest(name string) *Assertion { a.substestName = ft.Ptr(name); return a }
func (a *Assertion) That(t is.That) *Assertion      { a.check.Set(t); return a }
func (a *Assertion) Fatal() *Assertion              { a.continueOnError = false; return a }
func (a *Assertion) Error() *Assertion              { a.continueOnError = true; return a }
func (a *Assertion) Verbose() *Assertion            { a.alwaysLog = true; return a }
func (a *Assertion) Queit() *Assertion              { a.alwaysLog = false; return a }

func (a *Assertion) Log(args ...any) *Assertion {
	a.messages.PushBack(func() string { return fmt.Sprint(args...) })
	return a
}

func (a *Assertion) Logf(tmpl string, args ...any) *Assertion {
	a.messages.PushBack(func() string { return fmt.Sprintf(tmpl, args...) })
	return a
}

func M() *dt.Pairs[string, any] { return &dt.Pairs[string, any]{} }

func (a *Assertion) Metadata(md *dt.Pairs[string, any]) *Assertion {
	a.messages.PushBack(func() string {
		out := make([]string, 0, md.Len())
		fun.Invariant.Must(md.Iterator().Observe(context.Background(),
			func(p dt.Pair[string, any]) {
				out = append(out, fmt.Sprintf("%s='%v'", p.Key, p.Value))
			}))
		return strings.Join(out, ", ")
	})
	return a
}

func (a *Assertion) Run(t *testing.T) {
	t.Helper()
	if a.substestName != nil {
		t.Run(*a.substestName, func(t *testing.T) {
			t.Helper()
			a.doRun(t)
		})
	} else {
		a.doRun(t)
	}
}

func (a *Assertion) doRun(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		t.Helper()
		if !a.alwaysLog && !t.Failed() {
			return
		}
		for it := a.messages.Front(); it.Ok(); it = it.Next() {
			if line := ft.SafeDo(it.Value()); line != "" {
				t.Log(line)
			}
		}
	})
	result := a.check.Resolve()
	if result == nil {
		return
	}
	t.Log(*result)
	if a.continueOnError {
		t.Fail()
	} else {
		t.FailNow()
	}
}
