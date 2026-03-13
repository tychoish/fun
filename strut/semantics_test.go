package strut

import (
	"slices"
	"testing"
)

func TestSemanticCorrectness(t *testing.T) {
	t.Run("With* methods append transformed strings", func(t *testing.T) {
		var b Builder
		b.WriteString("existing ")
		b.WithTrimSpace("  new  ")
		b.WriteString(" ")
		b.WithReplaceAll("hello world", "world", "Go")

		expected := "existing new hello Go"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("WithTrimPrefix and WithTrimSuffix work correctly", func(t *testing.T) {
		var b Builder
		b.WithTrimPrefix("prefix_content", "prefix_")
		b.WriteString(" ")
		b.WithTrimSuffix("content_suffix", "_suffix")

		expected := "content content" //nolint:dupword
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("WithReplace with count parameter", func(t *testing.T) {
		var b Builder
		b.WithReplace("a a a a", "a", "b", 2) //nolint:dupword

		expected := "b b a a" //nolint:dupword
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Append uses bulk write not byte-by-byte", func(t *testing.T) {
		// This test verifies functionality, not performance
		var b Builder
		data := []byte("test data with unicode: 世界")
		b.Append(data)

		expected := "test data with unicode: 世界"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}

		// Verify we can append multiple times
		b.Append([]byte(" more"))
		if b.String() != expected+" more" {
			t.Errorf("multiple Append calls failed")
		}
	})

	t.Run("ExtendJoin separator behavior", func(t *testing.T) {
		tests := []struct {
			name     string
			items    []string
			sep      string
			expected string
		}{
			{
				name:     "comma separator",
				items:    []string{"a", "b", "c"},
				sep:      ",",
				expected: "a,b,c",
			},
			{
				name:     "empty separator",
				items:    []string{"a", "b", "c"},
				sep:      "",
				expected: "abc",
			},
			{
				name:     "single item no separator",
				items:    []string{"alone"},
				sep:      ",",
				expected: "alone",
			},
			{
				name:     "empty list",
				items:    []string{},
				sep:      ",",
				expected: "",
			},
			{
				name:     "unicode separator",
				items:    []string{"a", "b", "c"},
				sep:      " → ",
				expected: "a → b → c",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var b Builder
				b.ExtendJoin(slices.Values(tt.items), tt.sep)
				got := b.String()
				if got != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, got)
				}
			})
		}
	})

	t.Run("Join vs Concat clarity", func(t *testing.T) {
		var b1, b2 Builder

		// Concat: no separator
		b1.Concat("a", "b", "c")
		if b1.String() != "abc" {
			t.Errorf("Concat should not add separators, got %q", b1.String())
		}

		// Join: with separator
		b2.Join([]string{"a", "b", "c"}, ",")
		if b2.String() != "a,b,c" {
			t.Errorf("Join should add separators, got %q", b2.String())
		}
	})

	t.Run("conditional When methods work correctly", func(t *testing.T) {
		var b Builder

		b.WhenWriteString(true, "yes1")
		b.WhenWriteString(false, "no")
		b.WhenWriteString(true, "yes2")

		expected := "yes1yes2"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("WhenJoin with false condition", func(t *testing.T) {
		var b Builder
		b.WriteString("before")
		b.WhenJoin(false, []string{"a", "b", "c"}, ",")
		b.WriteString("after")

		expected := "beforeafter"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Quote methods add quotes", func(t *testing.T) {
		var b Builder
		b.Quote("hello")
		b.WriteString(" ")
		b.QuoteRune('a')

		expected := `"hello" 'a'`
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Format methods convert to strings", func(t *testing.T) {
		var b Builder
		b.Int(42)
		b.WriteString(" ")
		b.FormatBool(true)
		b.WriteString(" ")
		b.FormatInt64(255, 16)

		expected := "42 true ff"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Repeat methods work with negative and zero", func(t *testing.T) {
		var b Builder
		b.Repeat("x", -1) // should do nothing
		b.Repeat("a", 0)  // should do nothing
		b.Repeat("b", 3)  // should add "bbb"

		expected := "bbb"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("RepeatLine adds newlines", func(t *testing.T) {
		var b Builder
		b.RepeatLine("test", 3)

		expected := "test\ntest\ntest\n" //nolint:dupword
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("ExtendLines vs Extend", func(t *testing.T) {
		var b1, b2 Builder
		items := []string{"a", "b", "c"}

		// Extend: no newlines
		b1.Extend(slices.Values(items))
		if b1.String() != "abc" {
			t.Errorf("Extend should concatenate without newlines, got %q", b1.String())
		}

		// ExtendLines: each with newline
		b2.ExtendLines(slices.Values(items))
		if b2.String() != "a\nb\nc\n" {
			t.Errorf("ExtendLines should add newlines, got %q", b2.String())
		}
	})
}

func TestSemanticEdgeCases(t *testing.T) {
	t.Run("empty strings in various methods", func(t *testing.T) {
		var b Builder
		b.WriteString("")
		b.WriteLine("")
		b.Concat("", "", "")
		b.Join([]string{"", "", ""}, ",")
		b.WithTrimSpace("")

		expected := "\n,,"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("mixed unicode operations", func(t *testing.T) {
		var b Builder
		b.WriteString("Hello ")
		b.WriteRune('世')
		b.WriteRune('界')
		b.WriteString(" ")
		b.Quote("🎉")

		expected := "Hello 世界 \"🎉\""
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("chaining multiple operations", func(t *testing.T) {
		var b Builder
		b.WriteString("Start: ")
		b.Int(1)
		b.WriteByte(' ')
		b.FormatBool(true)
		b.Line()
		b.WithReplaceAll("old old", "old", "new") //nolint:dupword
		b.Line()
		b.Join([]string{"a", "b"}, ",")

		expected := "Start: 1 true\nnew new\na,b" //nolint:dupword
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("NLines with various values", func(t *testing.T) {
		var b Builder
		b.WriteString("a")
		b.NLines(0)
		b.WriteString("b")
		b.NLines(2)
		b.WriteString("c")
		b.NLines(-1)
		b.WriteString("d")

		expected := "ab\n\ncd"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Append with empty and nil slices", func(t *testing.T) {
		var b Builder
		b.Append([]byte{})
		b.WriteString("x")
		b.Append(nil)
		b.WriteString("y")
		b.Append([]byte("z"))

		expected := "xyz"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("Wprintf formatting", func(t *testing.T) {
		var b Builder
		b.Wprintf("Value: %d, %s, %v", 42, "test", true)

		expected := "Value: 42, test, true"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("complex ExtendJoin with iterator", func(t *testing.T) {
		var b Builder
		b.WriteString("[")
		seq := func(yield func(string) bool) {
			for i := range 5 {
				if !yield(string('a' + byte(i))) {
					return
				}
			}
		}
		b.ExtendJoin(seq, ",")
		b.WriteString("]")

		expected := "[a,b,c,d,e]"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestNamingConsistency(t *testing.T) {
	t.Run("With* prefix clearly indicates append operation", func(t *testing.T) {
		var b Builder
		b.WriteString("prefix ")

		// All With* methods should append, not modify existing content
		b.WithTrimSpace("  data  ")
		if b.String() != "prefix data" {
			t.Error("WithTrimSpace should append trimmed string")
		}
	})

	t.Run("Write* prefix indicates direct writing", func(t *testing.T) {
		var b Builder

		// All Write* methods should write/append directly
		b.WriteString("a")
		b.WriteByte('b')
		b.WriteRune('c')
		b.WriteLine("d")

		if b.String() != "abcd\n" {
			t.Error("Write* methods should write directly")
		}
	})

	t.Run("When* prefix indicates conditional operation", func(t *testing.T) {
		var b Builder

		// All When* methods should be conditional
		b.WhenWriteString(true, "yes")
		b.WhenWriteString(false, "no")

		if b.String() != "yes" {
			t.Error("When* methods should be conditional")
		}
	})

	t.Run("Repeat* methods repeat operations", func(t *testing.T) {
		var b Builder

		b.Repeat("x", 3)
		b.RepeatByte('y', 2)
		b.RepeatRune('z', 1)

		expected := "xxxyyz"
		got := b.String()
		if got != expected {
			t.Errorf("Repeat* methods should repeat specified times, expected %q, got %q", expected, got)
		}
	})

	t.Run("Extend* methods work with iterators", func(t *testing.T) {
		var b Builder
		seq := slices.Values([]string{"a", "b"})

		b.Extend(seq)

		if b.String() != "ab" {
			t.Error("Extend* methods should work with iterators")
		}
	})
}

// TestSemanticEquivalence verifies that Buffer and Builder have identical
// semantics for all shared methods. This test ensures that both types produce
// the same output for the same sequence of operations.
func TestSemanticEquivalence(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(stringWriter)
		expected string
	}{
		{
			name: "basic write operations",
			buildFn: func(w stringWriter) {
				w.WriteString("hello")
				w.WriteByte(' ')
				w.WriteRune('世')
			},
			expected: "hello 世",
		},
		{
			name: "line operations",
			buildFn: func(w stringWriter) {
				w.WriteLine("line1")
				w.WriteLines("line2", "line3")
				w.NLines(2)
			},
			expected: "line1\nline2\nline3\n\n\n",
		},
		{
			name: "tab operations",
			buildFn: func(w stringWriter) {
				w.Tab()
				w.WriteString("indented")
				w.NTabs(3)
			},
			expected: "\tindented\t\t\t",
		},
		{
			name: "concat and join",
			buildFn: func(w stringWriter) {
				w.Concat("a", "b", "c")
				w.WriteByte(' ')
				w.Join([]string{"x", "y", "z"}, ",")
			},
			expected: "abc x,y,z",
		},
		{
			name: "repeat operations",
			buildFn: func(w stringWriter) {
				w.Repeat("ab", 3)
				w.RepeatByte('x', 2)
				w.RepeatRune('🎉', 2)
			},
			expected: "abababxx🎉🎉",
		},
		{
			name: "repeat line",
			buildFn: func(w stringWriter) {
				w.RepeatLine("test", 2)
			},
			expected: "test\ntest\n",
		},
		{
			name: "wprint operations",
			buildFn: func(w stringWriter) {
				w.Wprint("hello", " ", "world")
				w.Wprintf(" %d", 42)
				w.Wprintln("!")
			},
			expected: "hello world 42!\n",
		},
		{
			name: "when methods - true conditions",
			buildFn: func(w stringWriter) {
				w.WhenWriteString(true, "yes")
				w.WhenLine(true)
				w.WhenWriteString(false, "no")
			},
			expected: "yes\n",
		},
		{
			name: "quote operations",
			buildFn: func(w stringWriter) {
				w.Quote("test")
				w.WriteByte(' ')
				w.QuoteRune('a')
				w.WriteByte(' ')
				w.QuoteASCII("世")
			},
			expected: `"test" 'a' "\u4e16"`,
		},
		{
			name: "format numbers",
			buildFn: func(w stringWriter) {
				w.Int(42)
				w.WriteByte(' ')
				w.FormatBool(true)
				w.WriteByte(' ')
				w.FormatInt64(255, 16)
			},
			expected: "42 true ff",
		},
		{
			name: "trim operations",
			buildFn: func(w stringWriter) {
				w.WithTrimSpace("  hello  ")
				w.WriteByte(' ')
				w.WithTrimPrefix("prefix_test", "prefix_")
			},
			expected: "hello test",
		},
		{
			name: "replace operations",
			buildFn: func(w stringWriter) {
				w.WithReplaceAll("hello hello", "hello", "hi") //nolint:dupword
				w.WriteByte(' ')
				w.WithReplace("aaa", "a", "b", 2)
			},
			expected: "hi hi bba", //nolint:dupword
		},
		{
			name: "extend operations",
			buildFn: func(w stringWriter) {
				w.Extend(slices.Values([]string{"a", "b", "c"}))
				w.WriteByte(' ')
				w.ExtendJoin(slices.Values([]string{"x", "y"}), ",")
			},
			expected: "abc x,y",
		},
		{
			name: "bytes operations",
			buildFn: func(w stringWriter) {
				w.Append([]byte("test"))
				w.WriteBytesLine([]byte("line"))
				w.WriteBytesLines([]byte("a"), []byte("b"))
			},
			expected: "testline\na\nb\n",
		},
		{
			name: "append quote operations",
			buildFn: func(w stringWriter) {
				w.AppendQuote("test")
				w.WriteByte(' ')
				w.AppendQuoteRune('x')
				w.WriteByte(' ')
				w.AppendQuoteASCII("世")
			},
			expected: `"test" 'x' "\u4e16"`,
		},
		{
			name: "append number operations",
			buildFn: func(w stringWriter) {
				w.AppendBool(true)
				w.WriteByte(' ')
				w.AppendInt64(42, 10)
				w.WriteByte(' ')
				w.AppendUint64(255, 16)
				w.WriteByte(' ')
				w.AppendFloat(3.14, 'f', 2, 64)
			},
			expected: "true 42 ff 3.14",
		},
		{
			name: "append trim operations",
			buildFn: func(w stringWriter) {
				w.AppendTrimSpace([]byte("  hello  "))
				w.WriteByte(' ')
				w.AppendTrimPrefix([]byte("prefix_test"), []byte("prefix_"))
			},
			expected: "hello test",
		},
		{
			name: "append replace operations",
			buildFn: func(w stringWriter) {
				w.AppendReplaceAll([]byte("aa"), []byte("a"), []byte("b"))
				w.WriteByte(' ')
				w.AppendReplace([]byte("aaa"), []byte("a"), []byte("b"), 1)
			},
			expected: "bb baa",
		},
		{
			name: "extend bytes operations",
			buildFn: func(w stringWriter) {
				w.ExtendBytes(slices.Values([][]byte{[]byte("x"), []byte("y")}))
				w.WriteByte(' ')
				w.ExtendBytesJoin(slices.Values([][]byte{[]byte("a"), []byte("b")}), []byte(","))
			},
			expected: "xy a,b",
		},
		{
			name: "complex mixed operations",
			buildFn: func(w stringWriter) {
				w.WriteLine("Header")
				w.Tab()
				w.Wprintf("Value: %d", 42)
				w.Line()
				w.Concat("a", "b", "c")
				w.Line()
				w.Join([]string{"x", "y"}, " | ")
				w.Line()
				w.Quote("end")
			},
			expected: "Header\n\tValue: 42\nabc\nx | y\n\"end\"",
		},
		{
			name: "conditional operations mix",
			buildFn: func(w stringWriter) {
				w.WhenWriteLine(true, "included")
				w.WhenWriteLine(false, "skipped")
				w.WhenConcat(true, "a", "b")
				w.WhenJoin(true, []string{"x", "y"}, ",")
			},
			expected: "included\nabx,y",
		},
		{
			name: "edge case - empty operations",
			buildFn: func(w stringWriter) {
				w.WriteString("")
				w.Concat()
				w.WriteLines()
				w.Repeat("x", 0)
			},
			expected: "",
		},
		{
			name: "edge case - negative counts",
			buildFn: func(w stringWriter) {
				w.NLines(-1)
				w.NTabs(-5)
				w.Repeat("x", -1)
				w.RepeatByte('a', -10)
				w.WriteString("ok")
			},
			expected: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Buffer
			var buf Buffer
			tt.buildFn(&buf)
			bufResult := buf.String()

			// Test Builder
			var bld Builder
			tt.buildFn(&bld)
			bldResult := bld.String()

			// Verify they produce the same output
			if bufResult != bldResult {
				t.Errorf("Buffer and Builder produce different output:\nBuffer:  %q\nBuilder: %q", bufResult, bldResult)
			}

			// Verify both match expected output
			if bufResult != tt.expected {
				t.Errorf("Buffer output mismatch:\nExpected: %q\nGot:      %q", tt.expected, bufResult)
			}
			if bldResult != tt.expected {
				t.Errorf("Builder output mismatch:\nExpected: %q\nGot:      %q", tt.expected, bldResult)
			}
		})
	}
}

// TestBufferBuilderStateEquivalence verifies that Buffer and Builder maintain
// equivalent state through various operations.
func TestBufferBuilderStateEquivalence(t *testing.T) {
	t.Run("Len() equivalence", func(t *testing.T) {
		var buf Buffer
		var bld Builder

		if buf.Len() != bld.Len() {
			t.Errorf("initial length mismatch: Buffer=%d, Builder=%d", buf.Len(), bld.Len())
		}

		buf.WriteString("hello")
		bld.WriteString("hello")

		if buf.Len() != bld.Len() {
			t.Errorf("length after WriteString mismatch: Buffer=%d, Builder=%d", buf.Len(), bld.Len())
		}

		buf.WriteRune('世')
		bld.WriteRune('世')

		if buf.Len() != bld.Len() {
			t.Errorf("length after WriteRune mismatch: Buffer=%d, Builder=%d", buf.Len(), bld.Len())
		}
	})

	t.Run("Reset() equivalence", func(t *testing.T) {
		var buf Buffer
		var bld Builder

		buf.WriteString("test")
		bld.WriteString("test")

		buf.Reset()
		bld.Reset()

		if buf.String() != bld.String() {
			t.Errorf("Reset() output mismatch: Buffer=%q, Builder=%q", buf.String(), bld.String())
		}

		if buf.Len() != 0 || bld.Len() != 0 {
			t.Errorf("Reset() length mismatch: Buffer=%d, Builder=%d", buf.Len(), bld.Len())
		}
	})

	t.Run("Grow() equivalence", func(t *testing.T) {
		var buf Buffer
		var bld Builder

		initialBufCap := buf.Cap()
		initialBldCap := bld.Cap()

		buf.Grow(1000)
		bld.Grow(1000)

		if buf.Cap() < initialBufCap+1000 {
			t.Errorf("Buffer Grow() did not increase capacity sufficiently")
		}
		if bld.Cap() < initialBldCap+1000 {
			t.Errorf("Builder Grow() did not increase capacity sufficiently")
		}
	})
}
