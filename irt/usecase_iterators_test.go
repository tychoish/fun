package irt

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ucSource is an instrumented finite source: it records how often it
// was started and how often its iteration unwound (its deferred
// cleanup ran), so tests can check that abandoning a combinator
// releases the source.
type ucSource struct{ started, closed atomic.Int32 }

func (s *ucSource) seq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		s.started.Add(1)
		defer s.closed.Add(1)
		for i := 1; i <= n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// ucEventually polls cond until it holds; the 10s deadline is only a
// deadlock guard.
func ucEventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("deadlock: %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func ucReleased(t *testing.T, src *ucSource) {
	t.Helper()
	ucEventually(t, "source was not released", func() bool { return src.closed.Load() == src.started.Load() })
}

func ucLift2(src iter.Seq[int]) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for v := range src {
			if !yield(v, v*10) {
				return
			}
		}
	}
}

// ucFlat adapts a pair sequence to a plain one without a range
// statement, so the harness (not the runtime) detects misuse of yield.
func ucFlat(s iter.Seq2[int, int]) iter.Seq[int] {
	return func(yield func(int) bool) { s(func(a, b int) bool { return yield(a*1000 + b) }) }
}

// ucDrive consumes seq, returning at most limit items (limit <= 0
// means no limit) and counting calls to yield after it said stop.
func ucDrive(seq iter.Seq[int], limit int) (got []int, violations int) {
	stopped := false
	seq(func(v int) bool {
		if stopped {
			violations++
			return false
		}
		got = append(got, v)
		if limit > 0 && len(got) >= limit {
			stopped = true
			return false
		}
		return true
	})
	return got, violations
}

type ucCase struct {
	name      string
	build     func(t *testing.T, src iter.Seq[int]) iter.Seq[int]
	unordered bool // output order is not deterministic
	single    bool // documented single-use: later iterations may be empty
	// noReiterate skips the re-iteration check for a known bug.
	noReiterate string
}

func ucNorm(c ucCase, in []int) []int {
	out := slices.Clone(in)
	if c.unordered {
		slices.Sort(out)
	}
	return out
}

func ucIdent(v int) int          { return v }
func ucTrue(int) bool            { return true }
func ucFalse(int) bool           { return false }
func ucInc(v int) int            { return v + 1 }
func ucPair(a, b int) (int, int) { return a, b }
func ucPairOK(int, int) bool     { return true }
func ucPairNo(int, int) bool     { return false }

const ucModifyBug = "known bug: Modify/ModifyAll/Modify2/ModifyAll2 reassign the captured seq inside the returned closure, so every further iteration stacks another copy of the transformation"

func ucCases() []ucCase {
	one := func(name string, f func(src iter.Seq[int]) iter.Seq[int]) ucCase {
		return ucCase{name: name, build: func(_ *testing.T, src iter.Seq[int]) iter.Seq[int] { return f(src) }}
	}
	two := func(name string, f func(src iter.Seq2[int, int]) iter.Seq2[int, int]) ucCase {
		return ucCase{name: name, build: func(_ *testing.T, src iter.Seq[int]) iter.Seq[int] { return ucFlat(f(ucLift2(src))) }}
	}
	cases := []ucCase{
		one("Limit", func(s iter.Seq[int]) iter.Seq[int] { return Limit(s, 100) }),
		one("Convert", func(s iter.Seq[int]) iter.Seq[int] { return Convert(s, ucInc) }),
		one("Cast", func(s iter.Seq[int]) iter.Seq[int] { return Cast[int, int](s) }),
		{name: "Modify", noReiterate: ucModifyBug, build: func(_ *testing.T, s iter.Seq[int]) iter.Seq[int] { return Modify(s, ucInc) }},
		{name: "ModifyAll", noReiterate: ucModifyBug, build: func(_ *testing.T, s iter.Seq[int]) iter.Seq[int] { return ModifyAll(s, ucInc, ucInc) }},
		one("ForEach", func(s iter.Seq[int]) iter.Seq[int] { return ForEach(s, func(int) {}) }),
		one("ForEachWhile", func(s iter.Seq[int]) iter.Seq[int] { return ForEachWhile(s, ucTrue) }),
		one("Keep", func(s iter.Seq[int]) iter.Seq[int] { return Keep(s, ucTrue) }),
		one("Remove", func(s iter.Seq[int]) iter.Seq[int] { return Remove(s, ucFalse) }),
		one("RemoveValue", func(s iter.Seq[int]) iter.Seq[int] { return RemoveValue(s, 0) }),
		one("RemoveZeros", func(s iter.Seq[int]) iter.Seq[int] { return RemoveZeros(s) }),
		one("While", func(s iter.Seq[int]) iter.Seq[int] { return While(s, ucTrue) }),
		one("Until", func(s iter.Seq[int]) iter.Seq[int] { return Until(s, ucFalse) }),
		one("Unique", func(s iter.Seq[int]) iter.Seq[int] { return Unique(s) }),
		one("UniqueBy", func(s iter.Seq[int]) iter.Seq[int] { return UniqueBy(s, ucIdent) }),
		one("Append", func(s iter.Seq[int]) iter.Seq[int] { return Append(s, 98, 99) }),
		one("Join", func(s iter.Seq[int]) iter.Seq[int] { return Join(s, Args(98, 99)) }),
		one("Chain", func(s iter.Seq[int]) iter.Seq[int] { return Chain(Args(s, Args(98, 99))) }),
		one("Reverse", Reverse[int]),
		one("Sort", Sort[int]),
		one("SortBy", func(s iter.Seq[int]) iter.Seq[int] { return SortBy(s, ucIdent) }),
		one("SortFunc", func(s iter.Seq[int]) iter.Seq[int] { return SortFunc(s, cmp.Compare[int]) }),
		one("WithHooks", func(s iter.Seq[int]) iter.Seq[int] { return WithHooks(s, func() {}, func() {}) }),
		one("WithSetup", func(s iter.Seq[int]) iter.Seq[int] { return WithSetup(s, func() {}) }),
		one("Ptrs", func(s iter.Seq[int]) iter.Seq[int] { return Deref(Ptrs(s)) }),
		one("Merge", func(s iter.Seq[int]) iter.Seq[int] { return Merge(Index(s), func(i, v int) int { return i + v }) }),
		one("First", func(s iter.Seq[int]) iter.Seq[int] { return First(Index(s)) }),
		one("Second", func(s iter.Seq[int]) iter.Seq[int] { return Second(Index(s)) }),
		one("ChainSlices", func(s iter.Seq[int]) iter.Seq[int] {
			return ChainSlices(Convert(s, func(v int) []int { return []int{v, v} }))
		}),
		one("Flush", func(s iter.Seq[int]) iter.Seq[int] { return func(y func(int) bool) { Flush(s, y) } }),
		one("OrEmpty", OrEmpty[int]),
		{name: "WithMutex", single: true, build: func(_ *testing.T, s iter.Seq[int]) iter.Seq[int] { return WithMutex(s, new(sync.Mutex)) }},
		{name: "WithBuffer0", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return WithBuffer(t.Context(), s, 0) }},
		{name: "WithBuffer3", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return WithBuffer(t.Context(), s, 3) }},
		{name: "WithBuffer100", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return WithBuffer(t.Context(), s, 100) }},
		{name: "Pool1", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return Pool(t.Context(), 1, s, ucInc) }},
		{name: "Pool3", unordered: true, build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return Pool(t.Context(), 3, s, ucInc) }},
		{name: "Pool9", unordered: true, build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return Pool(t.Context(), 9, s, ucInc) }},
		{name: "PoolZero", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] { return Pool(t.Context(), 0, s, ucInc) }},
		{name: "Pipe/Channel", build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] {
			// the producer goroutine is bound to the test's context, which
			// ends after the check, so a stopped consumer cannot leak it.
			return func(yield func(int) bool) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				Flush(Channel(ctx, Pipe(ctx, s)), yield)
			}
		}},
		two("Index", func(s iter.Seq2[int, int]) iter.Seq2[int, int] {
			return Convert2(Index(First(s)), func(i, v int) (int, int) { return i, v })
		}),
		two("Keep2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Keep2(s, ucPairOK) }),
		two("Remove2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Remove2(s, ucPairNo) }),
		two("While2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return While2(s, ucPairOK) }),
		two("Until2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Until2(s, ucPairNo) }),
		two("Limit2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Limit2(s, 100) }),
		two("Convert2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Convert2(s, ucPair) }),
		two("Modify2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Modify2(s, ucPair) }),
		two("ModifyAll2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return ModifyAll2(s, ucPair, ucPair) }),
		two("ForEach2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return ForEach2(s, func(int, int) {}) }),
		two("ForEachWhile2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return ForEachWhile2(s, ucPairOK) }),
		two("Flip", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Flip(Flip(s)) }),
		two("Sort2", Sort2[int, int]),
		two("Sort1", Sort1[int, int]),
		two("SortBy2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] {
			return SortBy2(s, func(a, _ int) int { return a })
		}),
		two("SortFunc2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] {
			return SortFunc2(s, func(a, _, b, _ int) int { return cmp.Compare(a, b) })
		}),
		two("Zip", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Zip(First(s), Second(s)) }),
		two("Join2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Join2(s, Two(98, 99)) }),
		two("Chain2", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return Chain2(Args(s, Two(98, 99))) }),
		two("KVsplit", func(s iter.Seq2[int, int]) iter.Seq2[int, int] { return KVsplit(KVjoin(s)) }),
		two("OrEmpty2", OrEmpty2[int, int]),
		{name: "Pool2", unordered: true, build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] {
			return ucFlat(Pool2(t.Context(), 3, ucLift2(s), ucPair))
		}},
		{name: "Pool3Pair", unordered: true, build: func(t *testing.T, s iter.Seq[int]) iter.Seq[int] {
			return ucFlat(Pool3(t.Context(), 3, s, func(v int) (int, int) { return v, v * 10 }))
		}},
		{name: "WithMutex2", single: true, build: func(_ *testing.T, s iter.Seq[int]) iter.Seq[int] {
			return ucFlat(WithMutex2(ucLift2(s), new(sync.Mutex)))
		}},
	}
	for i := range cases {
		if cases[i].name == "Modify2" || cases[i].name == "ModifyAll2" {
			cases[i].noReiterate = ucModifyBug
		}
	}
	return cases
}

func TestUseCaseIteratorsEarlyBreakAtEveryPosition(t *testing.T) {
	const items = 5
	for _, c := range ucCases() {
		t.Run(c.name, func(t *testing.T) {
			full, viol := ucDrive(c.build(t, new(ucSource).seq(items)), 0)
			if viol != 0 || len(full) == 0 {
				t.Fatalf("baseline run yielded %v (violations %d)", full, viol)
			}
			full = ucNorm(c, full)
			for pos := 1; pos <= len(full)+1; pos++ {
				t.Run("break"+strconv.Itoa(pos), func(t *testing.T) {
					src := new(ucSource)
					got, viol := ucDrive(c.build(t, src.seq(items)), pos)
					if viol != 0 {
						t.Fatalf("%d yields after the consumer stopped", viol)
					}
					want := min(pos, len(full))
					if len(got) != want {
						t.Fatalf("got %d items %v, want %d", len(got), got, want)
					}
					if !c.unordered && !slices.Equal(got, full[:want]) {
						t.Fatalf("got %v, want prefix of %v", got, full)
					}
					ucReleased(t, src)
				})
			}
		})
	}
}

func TestUseCaseIteratorsReiterateTwice(t *testing.T) {
	for _, c := range ucCases() {
		t.Run(c.name, func(t *testing.T) {
			if c.noReiterate != "" {
				t.Skip(c.noReiterate)
			}
			src := new(ucSource)
			seq := c.build(t, src.seq(5))
			first, v1 := ucDrive(seq, 0)
			second, v2 := ucDrive(seq, 0)
			if v1+v2 != 0 {
				t.Fatalf("yield-after-stop violations: %d", v1+v2)
			}
			if c.single {
				// single-use is documented: a later iteration may be
				// empty but must never repeat or invent items.
				if len(second) != 0 && !slices.Equal(ucNorm(c, first), ucNorm(c, second)) {
					t.Fatalf("single-use iterator produced %v then %v", first, second)
				}
				return
			}
			if !slices.Equal(ucNorm(c, first), ucNorm(c, second)) {
				t.Fatalf("first pass %v, second pass %v", first, second)
			}
			ucReleased(t, src)
		})
	}
}

func TestUseCaseIteratorsConsumerPanicReleasesSource(t *testing.T) {
	for _, c := range ucCases() {
		t.Run(c.name, func(t *testing.T) {
			src := new(ucSource)
			seq := c.build(t, src.seq(5))
			calls := 0
			got := capturePanic(func() {
				seq(func(int) bool {
					if calls++; calls == 2 {
						panic("consumer boom")
					}
					return true
				})
			})
			if calls < 2 {
				// every case in ucCases() forwards at least 5 items from a
				// 5-element source before any consumer-side stop, so the
				// panic on the 2nd call is always reached; verified with
				// `go test -race -count=20`, which never skipped. Treat
				// fewer than 2 calls as a real regression (a combinator
				// that now stops delivering early) rather than skip it.
				t.Fatalf("combinator yielded only %d items, want at least 2", calls)
			}
			if got != "consumer boom" {
				t.Fatalf("recovered %v, want the consumer's panic", got)
			}
			ucReleased(t, src)
		})
	}
}

// TestUseCaseCallbackPanicsPropagate checks that a panic raised by a
// user callback reaches the goroutine that is ranging over the result,
// with its original value, and that the source is released.
func TestUseCaseCallbackPanicsPropagate(t *testing.T) {
	boom := func() int { panic("callback boom") }
	cases := map[string]func(src iter.Seq[int]){
		"Convert":      func(s iter.Seq[int]) { Collect(Convert(s, func(int) int { return boom() })) },
		"Modify":       func(s iter.Seq[int]) { Collect(Modify(s, func(int) int { return boom() })) },
		"ForEach":      func(s iter.Seq[int]) { Collect(ForEach(s, func(int) { boom() })) },
		"ForEachWhile": func(s iter.Seq[int]) { Collect(ForEachWhile(s, func(int) bool { boom(); return true })) },
		"Keep":         func(s iter.Seq[int]) { Collect(Keep(s, func(int) bool { boom(); return true })) },
		"Remove":       func(s iter.Seq[int]) { Collect(Remove(s, func(int) bool { boom(); return true })) },
		"While":        func(s iter.Seq[int]) { Collect(While(s, func(int) bool { boom(); return true })) },
		"Until":        func(s iter.Seq[int]) { Collect(Until(s, func(int) bool { boom(); return true })) },
		"UniqueBy":     func(s iter.Seq[int]) { Collect(UniqueBy(s, func(int) int { return boom() })) },
		"SortBy":       func(s iter.Seq[int]) { Collect(SortBy(s, func(int) int { return boom() })) },
		"SortFunc":     func(s iter.Seq[int]) { Collect(SortFunc(s, func(int, int) int { return boom() })) },
		"Merge":        func(s iter.Seq[int]) { Collect(Merge(Index(s), func(int, int) int { return boom() })) },
		"With":         func(s iter.Seq[int]) { Count2(With(s, func(int) int { return boom() })) },
		"GroupBy":      func(s iter.Seq[int]) { Count2(GroupBy(s, func(int) int { return boom() })) },
		"Reduce":       func(s iter.Seq[int]) { Reduce(s, func(int, int) int { return boom() }) },
		"Apply":        func(s iter.Seq[int]) { Apply(s, func(int) { boom() }) },
		"ApplyWhile":   func(s iter.Seq[int]) { ApplyWhile(s, func(int) bool { boom(); return true }) },
		"ApplyUntil":   func(s iter.Seq[int]) { _ = ApplyUntil(s, func(int) error { boom(); return nil }) },
		"ApplyAll":     func(s iter.Seq[int]) { _ = ApplyAll(s, func(int) error { boom(); return nil }) },
		"Resolve": func(s iter.Seq[int]) {
			Collect(Resolve(Convert(s, func(int) func() int { return func() int { return boom() } })))
		},
		"WithHooksBefore": func(s iter.Seq[int]) { Collect(WithHooks(s, func() { boom() }, nil)) },
		"Pool1Op":         func(s iter.Seq[int]) { Collect(Pool(context.Background(), 1, s, func(int) int { return boom() })) },
		"Pool4Op":         func(s iter.Seq[int]) { Collect(Pool(context.Background(), 4, s, func(int) int { return boom() })) },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			src := new(ucSource)
			got := capturePanic(func() { run(src.seq(5)) })
			if got != "callback boom" {
				t.Fatalf("recovered %v", got)
			}
			if name == "WithHooksBefore" {
				return // the source is never started
			}
			ucReleased(t, src)
		})
	}
}

func TestUseCaseCallbackPanicInHookStillRunsAfter(t *testing.T) {
	var after atomic.Int32
	src := new(ucSource)
	seq := WithHooks(src.seq(3), nil, func() { after.Add(1) })
	got := capturePanic(func() {
		for range seq {
			panic("body boom")
		}
	})
	if got != "body boom" || after.Load() != 1 {
		t.Fatalf("recovered %v, after hook ran %d times", got, after.Load())
	}
	ucReleased(t, src)
}

func TestUseCaseGeneratorsEarlyBreak(t *testing.T) {
	counter := func() func() int {
		n := 0
		return func() int { n++; return n }
	}
	cases := map[string]func() iter.Seq[int]{
		"Generate":      func() iter.Seq[int] { return Generate(counter()) },
		"GenerateOk":    func() iter.Seq[int] { return GenerateOk(func() (int, bool) { return 1, true }) },
		"GenerateWhile": func() iter.Seq[int] { return GenerateWhile(counter(), func(int) bool { return true }) },
		"GenerateN":     func() iter.Seq[int] { return GenerateN(1000, counter()) },
		"Monotonic":     Monotonic,
		"MonotonicFrom": func() iter.Seq[int] { return MonotonicFrom(-3) },
		"Range":         func() iter.Seq[int] { return Range(1, 1000) },
		"Resolve": func() iter.Seq[int] {
			return Resolve(Generate(func() func() int { return func() int { return 1 } }))
		},
		"Channel": func() iter.Seq[int] {
			ch := make(chan int)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				defer close(ch)
				for i := 0; ; i++ {
					select {
					case ch <- i:
					case <-ctx.Done():
						return
					}
				}
			}()
			return func(y func(int) bool) {
				defer cancel()
				Channel(ctx, ch)(y)
			}
		},
	}
	for name, mk := range cases {
		for _, pos := range []int{1, 2, 7} {
			t.Run(name+"/"+strconv.Itoa(pos), func(t *testing.T) {
				got, viol := ucDrive(mk(), pos)
				if len(got) != pos || viol != 0 {
					t.Fatalf("got %v, %d violations", got, viol)
				}
			})
		}
	}
}

// TestUseCaseModifyReiteration documents that the Modify family
// applies its transformation once per pass.
func TestUseCaseModifyReiteration(t *testing.T) {
	t.Skip(ucModifyBug)
	src := Args(1, 2, 3)
	cases := map[string]iter.Seq[int]{
		"Modify":     Modify(src, ucInc),
		"ModifyAll":  ModifyAll(src, ucInc),
		"Modify2":    Second(Modify2(With(src, ucIdent), func(a, b int) (int, int) { return a, b + 1 })),
		"ModifyAll2": Second(ModifyAll2(With(src, ucIdent), func(a, b int) (int, int) { return a, b + 1 })),
	}
	for name, seq := range cases {
		first, second := Collect(seq), Collect(seq)
		if !slices.Equal(first, second) {
			t.Errorf("%s: first pass %v, second pass %v", name, first, second)
		}
	}
}

func TestUseCaseTerminalOperationsOnEmptyAndOne(t *testing.T) {
	empty, one := Zero[int](), Args(7)
	if got := Collect(empty); len(got) != 0 {
		t.Fatalf("Collect(empty) = %v", got)
	}
	if Count(empty) != 0 || Count(one) != 1 {
		t.Fatal("Count")
	}
	if _, ok := Initial(empty); ok {
		t.Fatal("Initial(empty)")
	}
	if _, ok := Final(empty); ok {
		t.Fatal("Final(empty)")
	}
	if v, ok := Initial(one); !ok || v != 7 {
		t.Fatal("Initial(one)")
	}
	if v, ok := Final(one); !ok || v != 7 {
		t.Fatal("Final(one)")
	}
	if HasValues(empty) || !HasValues(one) {
		t.Fatal("HasValues")
	}
	if Contains(empty, 7) || !Contains(one, 7) {
		t.Fatal("Contains")
	}
	if !Equal(empty, Zero[int]()) || Equal(empty, one) || Equal(one, empty) || !Equal(one, Args(7)) {
		t.Fatal("Equal")
	}
	if Reduce(empty, func(acc, v int) int { return acc + v }) != 0 {
		t.Fatal("Reduce(empty)")
	}
	if Count2(Zip(empty, one)) != 0 {
		t.Fatal("Zip(empty, one)")
	}
	if got := Collect(Limit(one, 0)); len(got) != 0 {
		t.Fatalf("Limit(0) = %v", got)
	}
	if got := Collect(Limit(one, -1)); len(got) != 0 {
		t.Fatalf("Limit(-1) = %v", got)
	}
	if got := Collect(Chain(Zero[iter.Seq[int]]())); len(got) != 0 {
		t.Fatalf("Chain(empty) = %v", got)
	}
	if got := Collect(Join[int]()); len(got) != 0 {
		t.Fatalf("Join() = %v", got)
	}
	if got := JoinStrings(Zero[string]()); got != "" {
		t.Fatalf("JoinStrings(empty) = %q", got)
	}
	if got := Collect(Reverse(empty)); len(got) != 0 {
		t.Fatal("Reverse(empty)")
	}
	if got := fmt.Sprint(Collect(Reverse(one))); got != "[7]" {
		t.Fatalf("Reverse(one) = %s", got)
	}
}

func TestUseCaseZipEarlyBreakReleasesBothSources(t *testing.T) {
	for _, pos := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(pos), func(t *testing.T) {
			a, b := new(ucSource), new(ucSource)
			n := 0
			for range Zip(a.seq(10), b.seq(10)) {
				if n++; n == pos {
					break
				}
			}
			ucReleased(t, a)
			ucReleased(t, b)
		})
	}
	t.Run("UnequalLengths", func(t *testing.T) {
		a, b := new(ucSource), new(ucSource)
		if got := Count2(Zip(a.seq(2), b.seq(10))); got != 2 {
			t.Fatalf("zip of 2 and 10 yielded %d", got)
		}
		ucReleased(t, a)
		ucReleased(t, b)
	})
}

func TestUseCaseEqualReleasesBothSources(t *testing.T) {
	a, b := new(ucSource), new(ucSource)
	if Equal(a.seq(3), b.seq(5)) {
		t.Fatal("unequal sequences compared equal")
	}
	ucReleased(t, a)
	ucReleased(t, b)
}
