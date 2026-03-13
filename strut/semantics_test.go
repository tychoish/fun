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

	t.Run("WriteLines vs ExtendParagraph", func(t *testing.T) {
		var b1, b2 Builder

		// WriteLines: each on own line
		b1.WriteLines("a", "b", "c")
		expected1 := "a\nb\nc\n"
		if b1.String() != expected1 {
			t.Errorf("WriteLines: expected %q, got %q", expected1, b1.String())
		}

		// ExtendParagraph: each on own line plus blank line
		b2.ExtendParagraph(slices.Values([]string{"a", "b", "c"}))
		expected2 := "a\nb\nc\n\n"
		if b2.String() != expected2 {
			t.Errorf("ExtendParagraph: expected %q, got %q", expected2, b2.String())
		}
	})

	t.Run("ExtendParagraph adds blank line at end", func(t *testing.T) {
		var b Builder
		b.WriteString("Paragraph 1:\n")
		b.ExtendParagraph(slices.Values([]string{"line1", "line2"}))
		b.WriteString("Paragraph 2:\n")
		b.ExtendParagraph(slices.Values([]string{"line3", "line4"}))

		expected := "Paragraph 1:\nline1\nline2\n\nParagraph 2:\nline3\nline4\n\n"
		got := b.String()
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
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
