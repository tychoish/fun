package strut

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
)

// helper to create a Mutable from a string without pooling concerns.
func mutFrom(s string) *Mutable { return MutableFrom(s) }

// readAll drains an io.Reader and returns the content as a string.
func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading combined reader: %v", err)
	}
	return buf.String()
}

// TestJoinerConstructors verifies that every JoinConstructors method
// produces a Joiner with the expected separator value.
func TestJoinerConstructors(t *testing.T) {
	cases := []struct {
		name   string
		joiner Joiner
		sep    string
	}{
		{"Space", JOIN.WithSpace(), " "},
		{"DoubleSpace", JOIN.WithDoubleSpace(), "  "},
		{"Tab", JOIN.WithTab(), "\t"},
		{"Line", JOIN.WithLine(), "\n"},
		{"DoubleLine", JOIN.WithDoubleLine(), "\n\n"},
		{"Concat", JOIN.WithConcat(), ""},
		{"Underscore", JOIN.WithUnderscore(), "_"},
		{"Dash", JOIN.WithDash(), "-"},
		{"Comma", JOIN.WithComma(), ","},
		{"SemiColon", JOIN.WithSemiColon(), ";"},
		{"Sep/custom", JOIN.With("::"), "::"},
		{"Sep/empty", JOIN.With(""), ""},
		{"Sep/multichar", JOIN.With(" -> "), " -> "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use Strings to observe the separator in action.
			got := tc.joiner.Strings("a", "b")
			want := "a" + tc.sep + "b"
			if got != want {
				t.Errorf("Strings separator: got %q, want %q", got, want)
			}
		})
	}
}

// TestJoinerStrings covers Strings, StringSeq, and StringSlice.
func TestJoinerStrings(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		input  []string
		want   string
	}
	cases := []tc{
		{"empty input", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithSpace(), []string{"hello"}, "hello"},
		{"two", JOIN.WithSpace(), []string{"foo", "bar"}, "foo bar"},
		{"multi", JOIN.WithComma(), []string{"a", "b", "c"}, "a,b,c"},
		{"empty sep", JOIN.WithConcat(), []string{"x", "y", "z"}, "xyz"},
		{"multi-char sep", JOIN.With("::"), []string{"a", "b"}, "a::b"},
		{"empty strings", JOIN.WithDash(), []string{"", ""}, "-"},
	}

	for _, tc := range cases {
		t.Run("Strings/"+tc.name, func(t *testing.T) {
			got := tc.joiner.Strings(tc.input...)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("StringSeq/"+tc.name, func(t *testing.T) {
			got := tc.joiner.StringSeq(slices.Values(tc.input))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("StringSlice/"+tc.name, func(t *testing.T) {
			got := tc.joiner.StringSlice(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerStringsMutable covers StringsMutable, StringMutableSeq, and StringMutableSlice.
func TestJoinerStringsMutable(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		input  []string
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithDash(), []string{"only"}, "only"},
		{"two", JOIN.WithSpace(), []string{"hello", "world"}, "hello world"},
		{"multi", JOIN.WithComma(), []string{"a", "b", "c"}, "a,b,c"},
		{"concat sep", JOIN.WithConcat(), []string{"go", "lang"}, "golang"},
		{"multi-char sep", JOIN.With(" | "), []string{"x", "y"}, "x | y"},
	}

	for _, tc := range cases {
		t.Run("StringsMutable/"+tc.name, func(t *testing.T) {
			m := tc.joiner.StringsMutable(tc.input...)
			got := m.String()
			m.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("StringMutableSeq/"+tc.name, func(t *testing.T) {
			m := tc.joiner.StringMutableSeq(slices.Values(tc.input))
			got := m.String()
			m.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("StringMutableSlice/"+tc.name, func(t *testing.T) {
			m := tc.joiner.StringMutableSlice(tc.input)
			got := m.String()
			m.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerMutables covers Mutables, MutableSeq, and MutableSlice.
// Mutable methods release their inputs; we capture output before any Release call.
func TestJoinerMutables(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		inputs []string
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithComma(), []string{"only"}, "only"},
		{"two", JOIN.WithSpace(), []string{"foo", "bar"}, "foo bar"},
		{"multi", JOIN.WithDash(), []string{"a", "b", "c"}, "a-b-c"},
		{"concat sep", JOIN.WithConcat(), []string{"go", "lang"}, "golang"},
		{"multi-char sep", JOIN.With("::"), []string{"p", "q"}, "p::q"},
	}

	for _, tc := range cases {
		t.Run("Mutables/"+tc.name, func(t *testing.T) {
			muts := make([]*Mutable, len(tc.inputs))
			for i, s := range tc.inputs {
				muts[i] = mutFrom(s)
			}
			out := tc.joiner.Mutables(muts...)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("MutableSlice/"+tc.name, func(t *testing.T) {
			muts := make([]*Mutable, len(tc.inputs))
			for i, s := range tc.inputs {
				muts[i] = mutFrom(s)
			}
			out := tc.joiner.MutableSlice(muts)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("MutableSeq/"+tc.name, func(t *testing.T) {
			muts := make([]*Mutable, len(tc.inputs))
			for i, s := range tc.inputs {
				muts[i] = mutFrom(s)
			}
			out := tc.joiner.MutableSeq(slices.Values(muts))
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerMutablesReleasesInputs verifies that Mutables releases all inputs.
func TestJoinerMutablesReleasesInputs(t *testing.T) {
	// We can't directly observe that Release was called, but we can verify
	// that the joiner does not panic and produces the correct output.
	j := JOIN.WithSpace()
	a := mutFrom("alpha")
	b := mutFrom("beta")
	out := j.Mutables(a, b)
	got := out.String()
	out.Release()
	if got != "alpha beta" {
		t.Errorf("got %q, want %q", got, "alpha beta")
	}
}

// TestJoinerBytes covers Bytes, BytesSeq, and BytesSlice.
func TestJoinerBytes(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		inputs [][]byte
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithComma(), [][]byte{[]byte("only")}, "only"},
		{"two", JOIN.WithSpace(), [][]byte{[]byte("foo"), []byte("bar")}, "foo bar"},
		{"multi", JOIN.WithDash(), [][]byte{[]byte("a"), []byte("b"), []byte("c")}, "a-b-c"},
		{"double-dash", JOIN.WithDoubleDash(), [][]byte{[]byte("a"), []byte("b"), []byte("c")}, "a--b--c"},
		{"concat sep", JOIN.WithConcat(), [][]byte{[]byte("go"), []byte("lang")}, "golang"},
		{"multi-char sep", JOIN.With("::"), [][]byte{[]byte("p"), []byte("q")}, "p::q"},
		{"empty byte slices", JOIN.WithComma(), [][]byte{{}, {}}, ","},
	}

	for _, tc := range cases {
		t.Run("Bytes/"+tc.name, func(t *testing.T) {
			out := tc.joiner.Bytes(tc.inputs...)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("BytesSeq/"+tc.name, func(t *testing.T) {
			out := tc.joiner.BytesSeq(slices.Values(tc.inputs))
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("BytesSlice/"+tc.name, func(t *testing.T) {
			out := tc.joiner.BytesSlice(tc.inputs)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerBuffers covers Buffers, BufferSeq, BufferSlice, and BufferPtrsSlice.
func TestJoinerBuffers(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		inputs []string
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithComma(), []string{"only"}, "only"},
		{"two", JOIN.WithSpace(), []string{"hello", "world"}, "hello world"},
		{"multi", JOIN.WithDash(), []string{"a", "b", "c"}, "a-b-c"},
		{"double-dash", JOIN.WithDoubleDash(), []string{"a", "b", "c"}, "a--b--c"},
		{"concat sep", JOIN.WithConcat(), []string{"go", "lang"}, "golang"},
		{"multi-char sep", JOIN.With(" | "), []string{"x", "y"}, "x | y"},
	}

	for _, tc := range cases {
		t.Run("Buffers/"+tc.name, func(t *testing.T) {
			bufs := make([]*Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = NewBuffer([]byte(s))
			}
			out := tc.joiner.Buffers(bufs...)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("BufferSeq/"+tc.name, func(t *testing.T) {
			bufs := make([]*Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = NewBuffer([]byte(s))
			}
			out := tc.joiner.BufferSeq(slices.Values(bufs))
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("BufferSlice/"+tc.name, func(t *testing.T) {
			rawBufs := make([]Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				rawBufs[i].WriteString(s)
			}
			out := tc.joiner.BufferSlice(rawBufs)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("BufferPtrsSlice/"+tc.name, func(t *testing.T) {
			bufs := make([]*Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = NewBuffer([]byte(s))
			}
			out := tc.joiner.BufferPtrsSlice(bufs)
			got := out.String()
			out.Release()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerBytesBuffer covers BytesBuffer, BytesBufferSeq, BytesBufferSlice,
// and BytesBufferPtrsSlice.
func TestJoinerBytesBuffer(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		inputs []string
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithComma(), []string{"only"}, "only"},
		{"two", JOIN.WithSpace(), []string{"hello", "world"}, "hello world"},
		{"multi", JOIN.WithDash(), []string{"a", "b", "c"}, "a-b-c"},
		{"concat sep", JOIN.WithConcat(), []string{"go", "lang"}, "golang"},
		{"multi-char sep", JOIN.With("::"), []string{"p", "q"}, "p::q"},
	}

	for _, tc := range cases {
		t.Run("BytesBuffer/"+tc.name, func(t *testing.T) {
			bufs := make([]*bytes.Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = bytes.NewBufferString(s)
			}
			out := tc.joiner.BytesBuffer(bufs...)
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
		t.Run("BytesBufferSeq/"+tc.name, func(t *testing.T) {
			bufs := make([]*bytes.Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = bytes.NewBufferString(s)
			}
			out := tc.joiner.BytesBufferSeq(slices.Values(bufs))
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
		t.Run("BytesBufferSlice/"+tc.name, func(t *testing.T) {
			rawBufs := make([]bytes.Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				rawBufs[i].WriteString(s)
			}
			out := tc.joiner.BytesBufferSlice(rawBufs)
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
		t.Run("BytesBufferPtrsSlice/"+tc.name, func(t *testing.T) {
			bufs := make([]*bytes.Buffer, len(tc.inputs))
			for i, s := range tc.inputs {
				bufs[i] = bytes.NewBufferString(s)
			}
			out := tc.joiner.BytesBufferPtrsSlice(bufs)
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestJoinerReaders covers Readers, ReaderSeq, and ReaderSlice.
func TestJoinerReaders(t *testing.T) {
	type tc struct {
		name   string
		joiner Joiner
		inputs []string
		want   string
	}
	cases := []tc{
		{"empty", JOIN.WithSpace(), nil, ""},
		{"single", JOIN.WithComma(), []string{"only"}, "only"},
		{"two", JOIN.WithSpace(), []string{"hello", "world"}, "hello world"},
		{"multi", JOIN.WithDash(), []string{"a", "b", "c"}, "a-b-c"},
		{"double-dash", JOIN.WithDoubleDash(), []string{"a", "b", "c"}, "a--b--c"},
		{"concat sep", JOIN.WithConcat(), []string{"go", "lang"}, "golang"},
		{"multi-char sep", JOIN.With(" -> "), []string{"x", "y"}, "x -> y"},
		{"tab sep", JOIN.WithTab(), []string{"col1", "col2", "col3"}, "col1\tcol2\tcol3"},
		{"line sep", JOIN.WithLine(), []string{"line1", "line2"}, "line1\nline2"},
	}

	makeReaders := func(inputs []string) []io.Reader {
		rs := make([]io.Reader, len(inputs))
		for i, s := range inputs {
			rs[i] = strings.NewReader(s)
		}
		return rs
	}

	for _, tc := range cases {
		t.Run("Readers/"+tc.name, func(t *testing.T) {
			r := tc.joiner.Readers(makeReaders(tc.inputs)...)
			got := readAll(t, r)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("ReaderSeq/"+tc.name, func(t *testing.T) {
			rs := makeReaders(tc.inputs)
			r := tc.joiner.ReaderSeq(slices.Values(rs))
			got := readAll(t, r)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
		t.Run("ReaderSlice/"+tc.name, func(t *testing.T) {
			r := tc.joiner.ReaderSlice(makeReaders(tc.inputs))
			got := readAll(t, r)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJoinerReaderSeparatorPlacement verifies separators appear only between
// elements (not before the first or after the last) for Reader variants.
func TestJoinerReaderSeparatorPlacement(t *testing.T) {
	j := JOIN.WithComma()
	inputs := []io.Reader{
		strings.NewReader("one"),
		strings.NewReader("two"),
		strings.NewReader("three"),
	}
	got := readAll(t, j.Readers(inputs...))
	want := "one,two,three"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if strings.HasPrefix(got, ",") {
		t.Error("separator should not appear before first element")
	}
	if strings.HasSuffix(got, ",") {
		t.Error("separator should not appear after last element")
	}
}

// TestJoinerSeparatorNotInSingleInput verifies no separator appears for single-element input.
func TestJoinerSeparatorNotInSingleInput(t *testing.T) {
	sep := ","
	j := JOIN.WithComma()
	input := "hello"

	t.Run("Strings", func(t *testing.T) {
		got := j.Strings(input)
		if strings.Contains(got, sep) {
			t.Errorf("single input should not contain separator; got %q", got)
		}
	})
	t.Run("StringsMutable", func(t *testing.T) {
		m := j.StringsMutable(input)
		got := m.String()
		m.Release()
		if strings.Contains(got, sep) {
			t.Errorf("single input should not contain separator; got %q", got)
		}
	})
	t.Run("Bytes", func(t *testing.T) {
		m := j.Bytes([]byte(input))
		got := m.String()
		m.Release()
		if strings.Contains(got, sep) {
			t.Errorf("single input should not contain separator; got %q", got)
		}
	})
	t.Run("Readers", func(t *testing.T) {
		got := readAll(t, j.Readers(strings.NewReader(input)))
		if strings.Contains(got, sep) {
			t.Errorf("single input should not contain separator; got %q", got)
		}
	})
}

// TestJoinerEmptyInput verifies that all methods handle zero-element input gracefully.
func TestJoinerEmptyInput(t *testing.T) {
	j := JOIN.WithSpace()

	t.Run("Strings", func(t *testing.T) {
		if got := j.Strings(); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("StringSeq", func(t *testing.T) {
		if got := j.StringSeq(slices.Values([]string{})); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("StringSlice", func(t *testing.T) {
		if got := j.StringSlice(nil); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("StringsMutable", func(t *testing.T) {
		m := j.StringsMutable()
		got := m.String()
		m.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("StringMutableSeq", func(t *testing.T) {
		m := j.StringMutableSeq(slices.Values([]string{}))
		got := m.String()
		m.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("StringMutableSlice", func(t *testing.T) {
		m := j.StringMutableSlice(nil)
		got := m.String()
		m.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("Mutables", func(t *testing.T) {
		out := j.Mutables()
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("MutableSeq", func(t *testing.T) {
		out := j.MutableSeq(slices.Values([]*Mutable{}))
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("MutableSlice", func(t *testing.T) {
		out := j.MutableSlice(nil)
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("Bytes", func(t *testing.T) {
		out := j.Bytes()
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BytesSeq", func(t *testing.T) {
		out := j.BytesSeq(slices.Values([][]byte{}))
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BytesSlice", func(t *testing.T) {
		out := j.BytesSlice(nil)
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("Buffers", func(t *testing.T) {
		out := j.Buffers()
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BufferSeq", func(t *testing.T) {
		out := j.BufferSeq(slices.Values([]*Buffer{}))
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BufferSlice", func(t *testing.T) {
		out := j.BufferSlice(nil)
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BufferPtrsSlice", func(t *testing.T) {
		out := j.BufferPtrsSlice(nil)
		got := out.String()
		out.Release()
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("BytesBuffer", func(t *testing.T) {
		out := j.BytesBuffer()
		if out.String() != "" {
			t.Errorf("got %q, want empty", out.String())
		}
	})
	t.Run("BytesBufferSeq", func(t *testing.T) {
		out := j.BytesBufferSeq(slices.Values([]*bytes.Buffer{}))
		if out.String() != "" {
			t.Errorf("got %q, want empty", out.String())
		}
	})
	t.Run("BytesBufferSlice", func(t *testing.T) {
		out := j.BytesBufferSlice(nil)
		if out.String() != "" {
			t.Errorf("got %q, want empty", out.String())
		}
	})
	t.Run("BytesBufferPtrsSlice", func(t *testing.T) {
		out := j.BytesBufferPtrsSlice(nil)
		if out.String() != "" {
			t.Errorf("got %q, want empty", out.String())
		}
	})
	t.Run("Readers", func(t *testing.T) {
		got := readAll(t, j.Readers())
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("ReaderSeq", func(t *testing.T) {
		got := readAll(t, j.ReaderSeq(slices.Values([]io.Reader{})))
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
	t.Run("ReaderSlice", func(t *testing.T) {
		got := readAll(t, j.ReaderSlice(nil))
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestJoinerMultiCharSep exercises every method with a multi-character separator.
func TestJoinerMultiCharSep(t *testing.T) {
	j := JOIN.With(" -> ")
	want := "first -> second -> third"

	t.Run("Strings", func(t *testing.T) {
		if got := j.Strings("first", "second", "third"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("StringSeq", func(t *testing.T) {
		if got := j.StringSeq(slices.Values([]string{"first", "second", "third"})); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("StringSlice", func(t *testing.T) {
		if got := j.StringSlice([]string{"first", "second", "third"}); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("StringsMutable", func(t *testing.T) {
		m := j.StringsMutable("first", "second", "third")
		got := m.Resolve()
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("Bytes", func(t *testing.T) {
		m := j.Bytes([]byte("first"), []byte("second"), []byte("third"))
		got := m.Resolve()
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("Readers", func(t *testing.T) {
		r := j.Readers(
			strings.NewReader("first"),
			strings.NewReader("second"),
			strings.NewReader("third"),
		)
		got := readAll(t, r)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestJoinerInterfaceCompliance verifies that joiner satisfies the Joiner interface
// and that JOIN is a valid JoinConstructors value.
func TestJoinerInterfaceCompliance(t *testing.T) {
	var _ Joiner = JOIN.WithSpace()
	var _ Joiner = JOIN.WithComma()
	var _ Joiner = JOIN.With("custom")

	// Verify JOIN is the zero value of JoinConstructors.
	var jc JoinConstructors
	if jc != JOIN {
		t.Error("JOIN should equal zero value of JoinConstructors")
	}
}

// TestJoinerStringConsistency verifies that Strings, StringSeq, and StringSlice
// always produce identical output for the same logical inputs.
func TestJoinerStringConsistency(t *testing.T) {
	joiners := []struct {
		name   string
		joiner Joiner
	}{
		{"Space", JOIN.WithSpace()},
		{"Comma", JOIN.WithComma()},
		{"Dash", JOIN.WithDash()},
		{"Concat", JOIN.WithConcat()},
		{"Sep(||)", JOIN.With("||")},
	}
	inputs := []string{"alpha", "bravo", "charlie"}

	for _, jc := range joiners {
		t.Run(jc.name, func(t *testing.T) {
			v := jc.joiner.Strings(inputs...)
			seq := jc.joiner.StringSeq(slices.Values(inputs))
			sl := jc.joiner.StringSlice(inputs)
			if v != seq {
				t.Errorf("Strings %q != StringSeq %q", v, seq)
			}
			if v != sl {
				t.Errorf("Strings %q != StringSlice %q", v, sl)
			}
		})
	}
}

// TestJoinerBytesMatchStrings verifies that Bytes output matches Strings output
// (same logical content, different input type).
func TestJoinerBytesMatchStrings(t *testing.T) {
	inputs := []string{"hello", "world", "test"}
	byteInputs := make([][]byte, len(inputs))
	for i, s := range inputs {
		byteInputs[i] = []byte(s)
	}

	joiners := []struct {
		name   string
		joiner Joiner
	}{
		{"Space", JOIN.WithSpace()},
		{"Comma", JOIN.WithComma()},
		{"Concat", JOIN.WithConcat()},
	}

	for _, jc := range joiners {
		t.Run(jc.name, func(t *testing.T) {
			strResult := jc.joiner.Strings(inputs...)
			byteResult := jc.joiner.Bytes(byteInputs...)
			got := byteResult.String()
			byteResult.Release()
			if strResult != got {
				t.Errorf("Strings %q != Bytes %q", strResult, got)
			}
		})
	}
}

// TestJoinerReadersContentMatchStrings verifies that Readers produce identical
// content to Strings for the same inputs and separator.
func TestJoinerReadersContentMatchStrings(t *testing.T) {
	inputs := []string{"alpha", "beta", "gamma"}
	joiners := []struct {
		name   string
		joiner Joiner
	}{
		{"Space", JOIN.WithSpace()},
		{"Comma", JOIN.WithComma()},
		{"Concat", JOIN.WithConcat()},
		{"Line", JOIN.WithLine()},
	}

	for _, jc := range joiners {
		t.Run(jc.name, func(t *testing.T) {
			wantStr := jc.joiner.Strings(inputs...)

			rs := make([]io.Reader, len(inputs))
			for i, s := range inputs {
				rs[i] = strings.NewReader(s)
			}
			got := readAll(t, jc.joiner.Readers(rs...))
			if got != wantStr {
				t.Errorf("Readers %q != Strings %q", got, wantStr)
			}
		})
	}
}
