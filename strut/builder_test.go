package strut

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestBuilder_BasicWrites(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, basicWriteTests[*Builder]())
}

func TestBuilder_Line(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, lineTests[*Builder]())
}

func TestBuilder_Concat(t *testing.T) {
	tests := concatTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			b.Concat(tt.strs...)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuilder_Append(t *testing.T) {
	tests := appendTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			b.Append(tt.buf)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuilder_Join(t *testing.T) {
	tests := joinTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			b.Join(tt.strs, tt.liminal)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuilder_Repeat(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, repeatTests[*Builder]())
}

func TestBuilder_AppendPrint(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, wprintTests[*Builder]())
}

func TestBuilder_WhenMethods(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, whenMethodTests[*Builder]())
}

func TestBuilder_Quote(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, quoteTests[*Builder]())
}

func TestBuilder_FormatNumbers(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, formatNumberTests[*Builder]())
}

func TestBuilder_Trim(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, trimTests[*Builder]())
}

func TestBuilder_Replace(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, replaceTests[*Builder]())
}

func TestBuilder_Extend(t *testing.T) {
	tests := extendTests[*Builder]()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			tt.buildFn(&b)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuilder_EdgeCases(t *testing.T) {
	runValidationTests(t, func() *Builder { return &Builder{} }, edgeCaseTests[*Builder]())
}

func TestBuilder_LenAndCap(t *testing.T) {
	var b Builder

	if b.Len() != 0 {
		t.Errorf("expected initial length 0, got %d", b.Len())
	}

	b.WriteString("hello")
	if b.Len() != 5 {
		t.Errorf("expected length 5, got %d", b.Len())
	}

	b.WriteRune('世')
	if b.Len() != 8 {
		t.Errorf("expected length 8, got %d", b.Len())
	}
}

func TestBuilder_Reset(t *testing.T) {
	var b Builder
	b.WriteString("hello")
	b.Reset()

	if b.Len() != 0 {
		t.Errorf("expected length 0 after reset, got %d", b.Len())
	}

	if b.String() != "" {
		t.Errorf("expected empty string after reset, got %q", b.String())
	}
}

func TestMakeBuilder(t *testing.T) {
	t.Run("returns non-nil with zero length", func(t *testing.T) {
		b := MakeBuilder(100)
		if b == nil {
			t.Fatal("MakeBuilder() returned nil")
		}
		if b.Len() != 0 {
			t.Errorf("initial length = %d, want 0", b.Len())
		}
	})

	t.Run("preallocates at least the requested capacity", func(t *testing.T) {
		b := MakeBuilder(100)
		if b.Cap() < 100 {
			t.Errorf("Cap() = %d, want >= 100", b.Cap())
		}
	})

	t.Run("zero capacity", func(t *testing.T) {
		b := MakeBuilder(0)
		if b == nil {
			t.Fatal("MakeBuilder(0) returned nil")
		}
		if b.Len() != 0 {
			t.Errorf("length = %d, want 0", b.Len())
		}
	})

	t.Run("usable after construction", func(t *testing.T) {
		b := MakeBuilder(64)
		b.WriteString("hello")
		if b.String() != "hello" {
			t.Errorf("got %q, want %q", b.String(), "hello")
		}
	})
}

func TestBuilder_Grow(t *testing.T) {
	var b Builder
	initialCap := b.Cap()

	b.Grow(1000)
	newCap := b.Cap()

	if newCap < initialCap+1000 {
		t.Errorf("expected capacity >= %d, got %d", initialCap+1000, newCap)
	}
}

func TestBuilder_WriteStringMultipleTimes(t *testing.T) {
	var b Builder
	strs := []string{"a", "b", "c", "d", "e"}

	for _, s := range strs {
		b.WriteString(s)
	}

	expected := "abcde"
	if b.String() != expected {
		t.Errorf("expected %q, got %q", expected, b.String())
	}
}

func TestBuilder_LargeString(t *testing.T) {
	var b Builder
	largeStr := strings.Repeat("x", 100000)

	b.WriteString(largeStr)
	got := b.String()

	if len(got) != 100000 {
		t.Errorf("expected length 100000, got %d", len(got))
	}
}

func TestBuilder_AllQuoteMethods(t *testing.T) {
	var b Builder

	b.Quote("test")
	b.WriteByte(' ')
	b.QuoteASCII("世")
	b.WriteByte(' ')
	b.QuoteGrapic("\x00")
	b.WriteByte(' ')
	b.QuoteRune('a')
	b.WriteByte(' ')
	b.QuoteRuneASCII('世')
	b.WriteByte(' ')
	b.QuoteRuneGrapic('\n')

	result := b.String()
	if !strings.Contains(result, `"test"`) {
		t.Errorf("expected quoted test in result")
	}
	if !strings.Contains(result, `'a'`) {
		t.Errorf("expected quoted rune 'a' in result")
	}
}

func TestBuilder_ChainedWrites(t *testing.T) {
	var b Builder

	b.WriteString("start")
	b.Line()
	b.WriteLine("middle")
	b.WriteLines("end1", "end2")

	expected := "start\nmiddle\nend1\nend2\n"
	got := b.String()

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestBuilder_WriteBytesLine(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, writeBytesLineTests[*Builder]())
}

func TestBuilder_AppendQuote(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, appendQuoteTests[*Builder]())
}

func TestBuilder_AppendNumber(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, appendNumberTests[*Builder]())
}

func TestBuilder_AppendTrim(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, appendTrimTests[*Builder]())
}

func TestBuilder_AppendReplace(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, appendReplaceTests[*Builder]())
}

func TestBuilder_ExtendBytes(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, extendBytesTests[*Builder]())
}

func TestBuilder_ExtendMutable(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, extendMutableTests[*Builder]())
}

func TestBuilder_WriteMutableLines(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, writeMutableLinesTests[*Builder]())
}

func TestBuilder_WhenWriteMutable(t *testing.T) {
	runBuildTests(t, func() *Builder { return &Builder{} }, whenWriteMutableTests[*Builder]())
}

func TestBuilder_PushBytes(t *testing.T) {
	tests := appendTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			b.PushBytes(tt.buf)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuilder_PushString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"simple string", "hello", "hello"},
		{"string with spaces", "hello world", "hello world"},
		{"string with newline", "line\n", "line\n"},
		{"unicode string", "世界", "世界"},
		{"multiple pushes", "foo", "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			b.PushString(tt.input)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}

	t.Run("sequential pushes", func(t *testing.T) {
		var b Builder
		b.PushString("hello")
		b.PushString(", ")
		b.PushString("world")
		if got := b.String(); got != "hello, world" {
			t.Errorf("expected %q, got %q", "hello, world", got)
		}
	})
}

func TestBuilder_Bytes(t *testing.T) {
	var b Builder
	b.WriteString("hello")

	got := b.Bytes()
	if string(got) != "hello" {
		t.Errorf("Bytes() = %q, want \"hello\"", got)
	}
	if len(got) != 5 {
		t.Errorf("Bytes() len = %d, want 5", len(got))
	}
}

func TestBuilder_Format(t *testing.T) {
	var b Builder
	b.WriteString("hello")

	result := fmt.Sprintf("%s", &b)
	if result != "hello" {
		t.Errorf("Format() with %%s = %q, want \"hello\"", result)
	}

	result = fmt.Sprintf("%v", &b)
	if result != "hello" {
		t.Errorf("Format() with %%v = %q, want \"hello\"", result)
	}
}

func TestBuilder_Print(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	var b Builder
	b.WriteString("hello world")
	b.Print()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "hello world" {
		t.Errorf("Print() wrote %q, want \"hello world\"", got)
	}
}

func TestBuilder_Println(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	var b Builder
	b.WriteString("hello world")
	b.Println()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "hello world\n" {
		t.Errorf("Println() wrote %q, want \"hello world\\n\"", got)
	}
}

func TestBuilder_Mutable(t *testing.T) {
	var b Builder
	b.WriteString("hello")

	m := b.Mutable()
	if m.String() != "hello" {
		t.Errorf("Mutable() = %q, want \"hello\"", m.String())
	}
	if m.Len() != 5 {
		t.Errorf("Mutable() len = %d, want 5", m.Len())
	}
}
