package irt

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
)

func TestUnmarshalJSON2ReadErrorReview(t *testing.T) {
	// a failing reader after a complete pair surfaces as an error.
	boom := errors.New("boom")
	in := io.MultiReader(strings.NewReader(`{"a":1 `), iotest.ErrReader(boom))
	var got error
	for _, err := range UnmarshalJSON2[string, int](in) {
		got = err
	}
	if !errors.Is(got, boom) {
		t.Errorf("got %v, want %v", got, boom)
	}
}

// ---- AsGenerator abandonment is covered in review_leak_test.go ----

// ---- WithMutex family ----

func TestWithMutexFamilyReview(t *testing.T) {
	t.Run("SecondIterationIsEmpty", func(t *testing.T) {
		var mu sync.Mutex
		w := WithMutex(Range(1, 5), &mu)
		if got := Collect(w); len(got) != 5 {
			t.Fatalf("first: %v", got)
		}
		if got := Collect(w); len(got) != 0 {
			t.Fatalf("second iteration should be empty (single-use), got %v", got)
		}
	})
	t.Run("EarlyBreakThenReiterate", func(t *testing.T) {
		var mu sync.Mutex
		w := WithMutex(Range(1, 100), &mu)
		for range w {
			break
		}
		if got := Collect(w); len(got) != 0 {
			t.Fatalf("after early break the iterator is released, got %v", got)
		}
	})
	t.Run("ConcurrentUse", func(t *testing.T) {
		const n = 2000
		var mu sync.Mutex
		w := WithMutex(Range(1, n), &mu)
		var total, sum atomic.Int64
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for v := range w {
					total.Add(1)
					sum.Add(int64(v))
				}
			})
		}
		wg.Wait()
		if total.Load() != n || sum.Load() != n*(n+1)/2 {
			t.Fatalf("total=%d sum=%d", total.Load(), sum.Load())
		}
	})
	t.Run("RW", func(t *testing.T) {
		var mu sync.RWMutex
		for name, w := range map[string]func() []int{
			"R": func() []int { return Collect(WithRMutex(Range(1, 4), &mu)) },
			"W": func() []int { return Collect(WithWMutex(Range(1, 4), &mu)) },
		} {
			if got := w(); !slices.Equal(got, []int{1, 2, 3, 4}) {
				t.Errorf("%s: %v", name, got)
			}
		}
		r := WithRMutex(Range(1, 4), &mu)
		Collect(r)
		if got := Collect(r); len(got) != 0 {
			t.Errorf("R second iteration: %v", got)
		}
		w := WithWMutex(Range(1, 4), &mu)
		for range w {
			break
		}
		if got := Collect(w); len(got) != 0 {
			t.Errorf("W after break: %v", got)
		}
	})
	t.Run("Pairs", func(t *testing.T) {
		var mu sync.Mutex
		var rw sync.RWMutex
		pairs := func() func(yield func(int, int) bool) {
			return func(yield func(int, int) bool) {
				for i := range 4 {
					if !yield(i, i*i) {
						return
					}
				}
			}
		}
		for name, w := range map[string]func() map[int]int{
			"M":  func() map[int]int { return Collect2(WithMutex2(pairs(), &mu)) },
			"RM": func() map[int]int { return Collect2(WithRMutex2(pairs(), &rw)) },
			"WM": func() map[int]int { return Collect2(WithWMutex2(pairs(), &rw)) },
		} {
			if got := w(); len(got) != 4 || got[3] != 9 {
				t.Errorf("%s: %v", name, got)
			}
		}
		w2 := WithMutex2(pairs(), &mu)
		for range w2 {
			break
		}
		if got := Collect2(w2); len(got) != 0 {
			t.Errorf("after break: %v", got)
		}
		w3 := WithRMutex2(pairs(), &rw)
		Collect2(w3)
		if got := Collect2(w3); len(got) != 0 {
			t.Errorf("RM second: %v", got)
		}
		w4 := WithWMutex2(pairs(), &rw)
		Collect2(w4)
		if got := Collect2(w4); len(got) != 0 {
			t.Errorf("WM second: %v", got)
		}
	})
}

// ---- callback panics and deferred cleanup ----

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	f()
}

func TestCallbackPanicRunsAfterHookReview(t *testing.T) {
	t.Run("Keep", func(t *testing.T) {
		var after atomic.Int32
		src := WithHooks(Range(1, 5), nil, func() { after.Add(1) })
		mustPanic(t, func() {
			for range Keep(src, func(int) bool { panic("boom") }) {
			}
		})
		if after.Load() != 1 {
			t.Fatalf("after hook ran %d times", after.Load())
		}
	})
	t.Run("Apply", func(t *testing.T) {
		var after atomic.Int32
		src := WithHooks(Range(1, 5), nil, func() { after.Add(1) })
		mustPanic(t, func() { Apply(src, func(int) { panic("boom") }) })
		if after.Load() != 1 {
			t.Fatalf("after hook ran %d times", after.Load())
		}
	})
	t.Run("Resolve", func(t *testing.T) {
		var after atomic.Int32
		src := WithHooks(Slice([]func() int{func() int { panic("boom") }}), nil, func() { after.Add(1) })
		mustPanic(t, func() {
			for range Resolve(src) {
			}
		})
		if after.Load() != 1 {
			t.Fatalf("after hook ran %d times", after.Load())
		}
	})
}

// ---- marshal writer errors and large streams ----

var errWriteFail = errors.New("write failed")

type limitWriter struct {
	limit, written int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		n := max(w.limit-w.written, 0)
		w.written += n
		return n, errWriteFail
	}
	w.written += len(p)
	return len(p), nil
}

func TestMarshalWriterErrorMidStreamReview(t *testing.T) {
	var full1, full2, full3 bytes.Buffer
	seq := Slice([]string{"alpha", "beta", "gamma"})
	seq2 := Zip(Slice([]string{"a", "b", "c"}), Slice([]int{1, 2, 3}))
	if err := MarshalToJSON(seq, &full1); err != nil {
		t.Fatal(err)
	}
	if err := MarshalToJSON2(seq2, &full2); err != nil {
		t.Fatal(err)
	}
	if err := MarshalToJSONL(seq, &full3); err != nil {
		t.Fatal(err)
	}
	for limit := range full1.Len() {
		if err := MarshalToJSON(seq, &limitWriter{limit: limit}); !errors.Is(err, errWriteFail) {
			t.Errorf("MarshalToJSON limit %d: err = %v", limit, err)
		}
	}
	for limit := range full2.Len() {
		if err := MarshalToJSON2(seq2, &limitWriter{limit: limit}); !errors.Is(err, errWriteFail) {
			t.Errorf("MarshalToJSON2 limit %d: err = %v", limit, err)
		}
	}
	for limit := range full3.Len() {
		if err := MarshalToJSONL(seq, &limitWriter{limit: limit}); !errors.Is(err, errWriteFail) {
			t.Errorf("MarshalToJSONL limit %d: err = %v", limit, err)
		}
	}
	// the sequence must stop being consumed once the writer fails
	var pulled int
	counting := func(yield func(int) bool) {
		for i := range 1000 {
			pulled++
			if !yield(i) {
				return
			}
		}
	}
	if err := MarshalToJSON(counting, &limitWriter{limit: 5}); !errors.Is(err, errWriteFail) {
		t.Fatalf("err = %v", err)
	}
	if pulled > 5 {
		t.Errorf("sequence consumed %d elements after writer failure", pulled)
	}
}

func TestMarshalLargeStreamsReview(t *testing.T) {
	const n = 50_000
	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		if err := MarshalToJSON(Range(0, n), &buf); err != nil {
			t.Fatal(err)
		}
		var got int
		for v, err := range UnmarshalJSON[int](&buf) {
			if err != nil {
				t.Fatal(err)
			}
			if v != got {
				t.Fatalf("element %d = %d", got, v)
			}
			got++
		}
		if got != n+1 {
			t.Fatalf("round-tripped %d elements, want %d", got, n+1)
		}
	})
	t.Run("JSON2", func(t *testing.T) {
		var buf bytes.Buffer
		if err := MarshalToJSON2(Zip(Range(0, n), Range(0, n)), &buf); err != nil {
			t.Fatal(err)
		}
		var got int
		for kv, err := range UnmarshalJSON2[int, int](&buf) {
			if err != nil {
				t.Fatal(err)
			}
			if kv.Key != kv.Value {
				t.Fatalf("pair %v", kv)
			}
			got++
		}
		if got != n+1 {
			t.Fatalf("round-tripped %d pairs, want %d", got, n+1)
		}
	})
	t.Run("JSONL", func(t *testing.T) {
		data, err := MarshalJSONL(Range(0, n))
		if err != nil {
			t.Fatal(err)
		}
		if lines := bytes.Count(data, []byte("\n")); lines != n+1 {
			t.Fatalf("%d lines, want %d", lines, n+1)
		}
	})
	t.Run("Text", func(t *testing.T) {
		data, err := MarshalText(Convert(Range(0, n), func(int) string { return "x" }))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != n+1 || strings.Trim(string(data), "x") != "" {
			t.Fatalf("len %d", len(data))
		}
	})
}

type textKey struct{ v string }

func (k *textKey) UnmarshalText(b []byte) error {
	if string(b) == "bad" {
		return errors.New("bad key")
	}
	k.v = "t:" + string(b)
	return nil
}

func TestDecodeJSONKeyReview(t *testing.T) {
	if k, err := decodeJSONKey[uint8]("200"); err != nil || k != 200 {
		t.Errorf("uint: %v, %v", k, err)
	}
	if _, err := decodeJSONKey[uint8]("300"); err == nil {
		t.Error("uint overflow should fail")
	}
	if _, err := decodeJSONKey[int]("x"); err == nil {
		t.Error("bad int should fail")
	}
	if k, err := decodeJSONKey[any]("s"); err != nil || k != "s" {
		t.Errorf("any: %v, %v", k, err)
	}
	if _, err := decodeJSONKey[error]("s"); err == nil {
		t.Error("non-empty interface should fail")
	}
	if _, err := decodeJSONKey[float64]("1"); err == nil {
		t.Error("float key should fail")
	}
	if k, err := decodeJSONKey[textKey]("a"); err != nil || k.v != "t:a" {
		t.Errorf("text: %v, %v", k, err)
	}
	if _, err := decodeJSONKey[textKey]("bad"); err == nil {
		t.Error("text unmarshaler error should propagate")
	}
}

func TestUnmarshalJSON2ErrorsReview(t *testing.T) {
	for name, in := range map[string]string{
		"trailing":  `{"a":1} x`,
		"badkey":    `{"bad":1}`,
		"badvalue":  `{"a":"s"}`,
		"truncated": `{"a":1`,
		"notobject": `[1]`,
		"empty":     ``,
	} {
		var sawErr bool
		for _, err := range UnmarshalJSON2[textKey, int](strings.NewReader(in)) {
			if err != nil {
				sawErr = true
			}
		}
		if !sawErr {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestShardReiterateShardReview(t *testing.T) {
	shards := Collect(Shard(t.Context(), 2, Range(1, 4)))
	Collect(shards[0])
	if got := Collect(shards[0]); len(got) != 0 {
		t.Fatalf("second iteration of a finished shard: %v", got)
	}
	Collect(shards[1])
}

func TestUnmarshalJSON2EarlyBreakReview(t *testing.T) {
	var n int
	for _, err := range UnmarshalJSON2[string, int](strings.NewReader(`{"a":1,"b":2,"c":3}`)) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		break
	}
	if n != 1 {
		t.Fatalf("n = %d", n)
	}
}
