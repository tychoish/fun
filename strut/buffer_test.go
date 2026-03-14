package strut

import (
	"strings"
	"testing"
)

func TestBuffer_BasicWrites(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, basicWriteTests[*Buffer]())
}

func TestBuffer_Line(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, lineTests[*Buffer]())
}

func TestBuffer_Concat(t *testing.T) {
	tests := concatTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Buffer
			b.Concat(tt.strs...)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuffer_Append(t *testing.T) {
	tests := appendTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Buffer
			b.Append(tt.buf)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuffer_Join(t *testing.T) {
	tests := joinTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Buffer
			b.Join(tt.strs, tt.liminal)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuffer_Repeat(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, repeatTests[*Buffer]())
}

func TestBuffer_AppendPrint(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, wprintTests[*Buffer]())
}

func TestBuffer_WhenMethods(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, whenMethodTests[*Buffer]())
}

func TestBuffer_Quote(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, quoteTests[*Buffer]())
}

func TestBuffer_FormatNumbers(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, formatNumberTests[*Buffer]())
}

func TestBuffer_Trim(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, trimTests[*Buffer]())
}

func TestBuffer_Replace(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, replaceTests[*Buffer]())
}

func TestBuffer_Extend(t *testing.T) {
	tests := extendTests[*Buffer]()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Buffer
			tt.buildFn(&b)
			got := b.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuffer_EdgeCases(t *testing.T) {
	runValidationTests(t, func() *Buffer { return &Buffer{} }, edgeCaseTests[*Buffer]())
}

func TestBuffer_LenAndCap(t *testing.T) {
	var b Buffer

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

func TestBuffer_Reset(t *testing.T) {
	var b Buffer
	b.WriteString("hello")
	b.Reset()

	if b.Len() != 0 {
		t.Errorf("expected length 0 after reset, got %d", b.Len())
	}

	if b.String() != "" {
		t.Errorf("expected empty string after reset, got %q", b.String())
	}
}

func TestBuffer_Grow(t *testing.T) {
	var b Buffer
	initialCap := b.Cap()

	b.Grow(1000)
	newCap := b.Cap()

	if newCap < initialCap+1000 {
		t.Errorf("expected capacity >= %d, got %d", initialCap+1000, newCap)
	}
}

func TestBuffer_WriteStringMultipleTimes(t *testing.T) {
	var b Buffer
	strs := []string{"a", "b", "c", "d", "e"}

	for _, s := range strs {
		b.WriteString(s)
	}

	expected := "abcde"
	if b.String() != expected {
		t.Errorf("expected %q, got %q", expected, b.String())
	}
}

func TestBuffer_LargeString(t *testing.T) {
	var b Buffer
	largeStr := strings.Repeat("x", 100000)

	b.WriteString(largeStr)
	got := b.String()

	if len(got) != 100000 {
		t.Errorf("expected length 100000, got %d", len(got))
	}
}

func TestBuffer_AllQuoteMethods(t *testing.T) {
	var b Buffer

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

func TestBuffer_ChainedWrites(t *testing.T) {
	var b Buffer

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

func TestBuffer_WriteBytesLine(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, writeBytesLineTests[*Buffer]())
}

func TestBuffer_AppendQuote(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, appendQuoteTests[*Buffer]())
}

func TestBuffer_AppendNumber(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, appendNumberTests[*Buffer]())
}

func TestBuffer_AppendTrim(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, appendTrimTests[*Buffer]())
}

func TestBuffer_AppendReplace(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, appendReplaceTests[*Buffer]())
}

func TestBuffer_ExtendBytes(t *testing.T) {
	runBuildTests(t, func() *Buffer { return &Buffer{} }, extendBytesTests[*Buffer]())
}

func TestBuffer_NewBuffer(t *testing.T) {
	t.Run("with initial content", func(t *testing.T) {
		b := NewBuffer([]byte("hello"))
		if b.String() != "hello" {
			t.Errorf("expected %q, got %q", "hello", b.String())
		}
	})

	t.Run("with empty content", func(t *testing.T) {
		b := NewBuffer([]byte{})
		if b.String() != "" {
			t.Errorf("expected empty string, got %q", b.String())
		}
	})

	t.Run("with nil", func(t *testing.T) {
		b := NewBuffer(nil)
		if b.String() != "" {
			t.Errorf("expected empty string, got %q", b.String())
		}
	})

	t.Run("can write after construction", func(t *testing.T) {
		b := NewBuffer([]byte("hello"))
		b.WriteString(" world")
		if b.String() != "hello world" {
			t.Errorf("expected %q, got %q", "hello world", b.String())
		}
	})
}

func TestMakeBuffer(t *testing.T) {
	t.Run("returns non-nil with zero length", func(t *testing.T) {
		b := MakeBuffer(100)
		if b == nil {
			t.Fatal("MakeBuffer() returned nil")
		}
		if b.Len() != 0 {
			t.Errorf("initial length = %d, want 0", b.Len())
		}
		b.Release()
	})

	t.Run("preallocates at least the requested capacity", func(t *testing.T) {
		b := MakeBuffer(100)
		if b.Cap() < 100 {
			t.Errorf("Cap() = %d, want >= 100", b.Cap())
		}
		b.Release()
	})

	t.Run("zero capacity", func(t *testing.T) {
		b := MakeBuffer(0)
		if b == nil {
			t.Fatal("MakeBuffer(0) returned nil")
		}
		if b.Len() != 0 {
			t.Errorf("length = %d, want 0", b.Len())
		}
		b.Release()
	})

	t.Run("usable after construction", func(t *testing.T) {
		b := MakeBuffer(64)
		b.WriteString("hello")
		if b.String() != "hello" {
			t.Errorf("got %q, want %q", b.String(), "hello")
		}
		b.Release()
	})

	t.Run("pool reuse resets content", func(t *testing.T) {
		b := MakeBuffer(32)
		b.WriteString("old data")
		b.Release()

		b2 := MakeBuffer(32)
		if b2.Len() != 0 {
			t.Errorf("pooled Buffer not reset: Len() = %d, want 0", b2.Len())
		}
		if b2.String() != "" {
			t.Errorf("pooled Buffer not reset: String() = %q, want empty", b2.String())
		}
		b2.Release()
	})

	t.Run("grows pooled instance to requested capacity", func(t *testing.T) {
		small := MakeBuffer(8)
		small.WriteString("x")
		small.Release()

		large := MakeBuffer(1000)
		if large.Cap() < 1000 {
			t.Errorf("Cap() = %d, want >= 1000", large.Cap())
		}
		if large.Len() != 0 {
			t.Errorf("Len() = %d, want 0", large.Len())
		}
		large.Release()
	})
}

func TestBuffer_Release(t *testing.T) {
	t.Run("resets content", func(t *testing.T) {
		b := MakeBuffer(32)
		b.WriteString("some content")
		b.Release()

		b2 := MakeBuffer(32)
		if b2.Len() != 0 {
			t.Errorf("after Release, Len() = %d, want 0", b2.Len())
		}
		b2.Release()
	})

	t.Run("large buffer not returned to pool", func(t *testing.T) {
		b := MakeBuffer(0)
		b.Write(make([]byte, 65*1024))
		// Release should not panic and should discard the oversized buffer
		b.Release()
	})

	t.Run("Release on stack-allocated Buffer does not panic", func(t *testing.T) {
		var b Buffer
		b.WriteString("data")
		b.Release() // not from pool, but Release must not panic
	})
}
