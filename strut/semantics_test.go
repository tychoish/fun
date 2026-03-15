package strut

import (
	"slices"
	"testing"
)

func semanticCorrectnessTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "With* methods append transformed strings",
			buildFn: func(w T) {
				w.WriteString("existing ")
				w.WithTrimSpace("  new  ")
				w.WriteString(" ")
				w.WithReplaceAll("hello world", "world", "Go")
			},
			expected: "existing new hello Go",
		},
		{
			name: "WithTrimPrefix and WithTrimSuffix work correctly",
			buildFn: func(w T) {
				w.WithTrimPrefix("prefix_content", "prefix_")
				w.WriteString(" ")
				w.WithTrimSuffix("content_suffix", "_suffix")
			},
			expected: "content content", //nolint:dupword
		},
		{
			name: "WithReplace with count parameter",
			buildFn: func(w T) {
				w.WithReplace("a a a a", "a", "b", 2) //nolint:dupword
			},
			expected: "b b a a", //nolint:dupword
		},
		{
			name: "Append single operation",
			buildFn: func(w T) {
				w.Append([]byte("test data with unicode: 世界"))
			},
			expected: "test data with unicode: 世界",
		},
		{
			name: "Append multiple operations",
			buildFn: func(w T) {
				w.Append([]byte("test data with unicode: 世界"))
				w.Append([]byte(" more"))
			},
			expected: "test data with unicode: 世界 more",
		},
		{
			name: "ExtendJoin comma separator",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"a", "b", "c"}), ",")
			},
			expected: "a,b,c",
		},
		{
			name: "ExtendJoin empty separator",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"a", "b", "c"}), "")
			},
			expected: "abc",
		},
		{
			name: "ExtendJoin single item no separator",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"alone"}), ",")
			},
			expected: "alone",
		},
		{
			name: "ExtendJoin empty list",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{}), ",")
			},
			expected: "",
		},
		{
			name: "ExtendJoin unicode separator",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"a", "b", "c"}), " → ")
			},
			expected: "a → b → c",
		},
		{
			name: "Join vs Concat - Concat no separator",
			buildFn: func(w T) {
				w.Concat("a", "b", "c")
			},
			expected: "abc",
		},
		{
			name: "Join vs Concat - Join with separator",
			buildFn: func(w T) {
				w.Join([]string{"a", "b", "c"}, ",")
			},
			expected: "a,b,c",
		},
		{
			name: "conditional When methods work correctly",
			buildFn: func(w T) {
				w.WhenWriteString(true, "yes1")
				w.WhenWriteString(false, "no")
				w.WhenWriteString(true, "yes2")
			},
			expected: "yes1yes2",
		},
		{
			name: "WhenJoin with false condition",
			buildFn: func(w T) {
				w.WriteString("before")
				w.WhenJoin(false, []string{"a", "b", "c"}, ",")
				w.WriteString("after")
			},
			expected: "beforeafter",
		},
		{
			name: "Quote methods add quotes",
			buildFn: func(w T) {
				w.Quote("hello")
				w.WriteString(" ")
				w.QuoteRune('a')
			},
			expected: `"hello" 'a'`,
		},
		{
			name: "Format methods convert to strings",
			buildFn: func(w T) {
				w.Int(42)
				w.WriteString(" ")
				w.FormatBool(true)
				w.WriteString(" ")
				w.FormatInt64(255, 16)
			},
			expected: "42 true ff",
		},
		{
			name: "Repeat methods work with negative and zero",
			buildFn: func(w T) {
				w.Repeat("x", -1)
				w.Repeat("a", 0)
				w.Repeat("b", 3)
			},
			expected: "bbb",
		},
		{
			name: "RepeatLine adds newlines",
			buildFn: func(w T) {
				w.RepeatLine("test", 3)
			},
			expected: "test\ntest\ntest\n", //nolint:dupword
		},
		{
			name: "ExtendLines vs Extend - Extend no newlines",
			buildFn: func(w T) {
				w.Extend(slices.Values([]string{"a", "b", "c"}))
			},
			expected: "abc",
		},
		{
			name: "ExtendLines vs Extend - ExtendLines with newlines",
			buildFn: func(w T) {
				w.ExtendLines(slices.Values([]string{"a", "b", "c"}))
			},
			expected: "a\nb\nc\n",
		},
	}
}

func semanticEdgeCaseTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "empty strings in various methods",
			buildFn: func(w T) {
				w.WriteString("")
				w.WriteLine("")
				w.Concat("", "", "")
				w.Join([]string{"", "", ""}, ",")
				w.WithTrimSpace("")
			},
			expected: "\n,,",
		},
		{
			name: "mixed unicode operations",
			buildFn: func(w T) {
				w.WriteString("Hello ")
				w.WriteRune('世')
				w.WriteRune('界')
				w.WriteString(" ")
				w.Quote("🎉")
			},
			expected: "Hello 世界 \"🎉\"",
		},
		{
			name: "chaining multiple operations",
			buildFn: func(w T) {
				w.WriteString("Start: ")
				w.Int(1)
				w.WriteByte(' ')
				w.FormatBool(true)
				w.Line()
				w.WithReplaceAll("old old", "old", "new") //nolint:dupword
				w.Line()
				w.Join([]string{"a", "b"}, ",")
			},
			expected: "Start: 1 true\nnew new\na,b", //nolint:dupword
		},
		{
			name: "NLines with various values",
			buildFn: func(w T) {
				w.WriteString("a")
				w.NLines(0)
				w.WriteString("b")
				w.NLines(2)
				w.WriteString("c")
				w.NLines(-1)
				w.WriteString("d")
			},
			expected: "ab\n\ncd",
		},
		{
			name: "Append with empty and nil slices",
			buildFn: func(w T) {
				w.Append([]byte{})
				w.WriteString("x")
				w.Append(nil)
				w.WriteString("y")
				w.Append([]byte("z"))
			},
			expected: "xyz",
		},
		{
			name: "AppendPrintf formatting",
			buildFn: func(w T) {
				w.AppendPrintf("Value: %d, %s, %v", 42, "test", true)
			},
			expected: "Value: 42, test, true",
		},
		{
			name: "complex ExtendJoin with iterator",
			buildFn: func(w T) {
				w.WriteString("[")
				seq := func(yield func(string) bool) {
					for i := range 5 {
						if !yield(string('a' + byte(i))) {
							return
						}
					}
				}
				w.ExtendJoin(seq, ",")
				w.WriteString("]")
			},
			expected: "[a,b,c,d,e]",
		},
	}
}

func semanticNamingTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "With* prefix clearly indicates append operation",
			buildFn: func(w T) {
				w.WriteString("prefix ")
				w.WithTrimSpace("  data  ")
			},
			expected: "prefix data",
		},
		{
			name: "Write* prefix indicates direct writing",
			buildFn: func(w T) {
				w.WriteString("a")
				w.WriteByte('b')
				w.WriteRune('c')
				w.WriteLine("d")
			},
			expected: "abcd\n",
		},
		{
			name: "When* prefix indicates conditional operation",
			buildFn: func(w T) {
				w.WhenWriteString(true, "yes")
				w.WhenWriteString(false, "no")
			},
			expected: "yes",
		},
		{
			name: "Repeat* methods repeat operations",
			buildFn: func(w T) {
				w.Repeat("x", 3)
				w.RepeatByte('y', 2)
				w.RepeatRune('z', 1)
			},
			expected: "xxxyyz",
		},
		{
			name: "Extend* methods work with iterators",
			buildFn: func(w T) {
				w.Extend(slices.Values([]string{"a", "b"}))
			},
			expected: "ab",
		},
	}
}

// semanticEquivalenceTests returns test cases that verify Buffer and Builder
// produce identical output for all shared operations. Running these against
// both types implicitly proves semantic equivalence.
func semanticEquivalenceTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "basic write operations",
			buildFn: func(w T) {
				w.WriteString("hello")
				w.WriteByte(' ')
				w.WriteRune('世')
			},
			expected: "hello 世",
		},
		{
			name: "line operations",
			buildFn: func(w T) {
				w.WriteLine("line1")
				w.WriteLines("line2", "line3")
				w.NLines(2)
			},
			expected: "line1\nline2\nline3\n\n\n",
		},
		{
			name: "tab operations",
			buildFn: func(w T) {
				w.Tab()
				w.WriteString("indented")
				w.NTabs(3)
			},
			expected: "\tindented\t\t\t",
		},
		{
			name: "concat and join",
			buildFn: func(w T) {
				w.Concat("a", "b", "c")
				w.WriteByte(' ')
				w.Join([]string{"x", "y", "z"}, ",")
			},
			expected: "abc x,y,z",
		},
		{
			name: "repeat operations",
			buildFn: func(w T) {
				w.Repeat("ab", 3)
				w.RepeatByte('x', 2)
				w.RepeatRune('🎉', 2)
			},
			expected: "abababxx🎉🎉",
		},
		{
			name: "repeat line",
			buildFn: func(w T) {
				w.RepeatLine("test", 2)
			},
			expected: "test\ntest\n",
		},
		{
			name: "when methods - true conditions",
			buildFn: func(w T) {
				w.WhenWriteString(true, "yes")
				w.WhenLine(true)
				w.WhenWriteString(false, "no")
			},
			expected: "yes\n",
		},
		{
			name: "quote operations",
			buildFn: func(w T) {
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
			buildFn: func(w T) {
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
			buildFn: func(w T) {
				w.WithTrimSpace("  hello  ")
				w.WriteByte(' ')
				w.WithTrimPrefix("prefix_test", "prefix_")
			},
			expected: "hello test",
		},
		{
			name: "replace operations",
			buildFn: func(w T) {
				w.WithReplaceAll("hello hello", "hello", "hi") //nolint:dupword
				w.WriteByte(' ')
				w.WithReplace("aaa", "a", "b", 2)
			},
			expected: "hi hi bba", //nolint:dupword
		},
		{
			name: "extend operations",
			buildFn: func(w T) {
				w.Extend(slices.Values([]string{"a", "b", "c"}))
				w.WriteByte(' ')
				w.ExtendJoin(slices.Values([]string{"x", "y"}), ",")
			},
			expected: "abc x,y",
		},
		{
			name: "bytes operations",
			buildFn: func(w T) {
				w.Append([]byte("test"))
				w.WriteBytesLine([]byte("line"))
				w.WriteBytesLines([]byte("a"), []byte("b"))
			},
			expected: "testline\na\nb\n",
		},
		{
			name: "append quote operations",
			buildFn: func(w T) {
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
			buildFn: func(w T) {
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
			buildFn: func(w T) {
				w.AppendTrimSpace([]byte("  hello  "))
				w.WriteByte(' ')
				w.AppendTrimPrefix([]byte("prefix_test"), []byte("prefix_"))
			},
			expected: "hello test",
		},
		{
			name: "append replace operations",
			buildFn: func(w T) {
				w.AppendReplaceAll([]byte("aa"), []byte("a"), []byte("b"))
				w.WriteByte(' ')
				w.AppendReplace([]byte("aaa"), []byte("a"), []byte("b"), 1)
			},
			expected: "bb baa",
		},
		{
			name: "extend bytes operations",
			buildFn: func(w T) {
				w.ExtendBytes(slices.Values([][]byte{[]byte("x"), []byte("y")}))
				w.WriteByte(' ')
				w.ExtendBytesJoin(slices.Values([][]byte{[]byte("a"), []byte("b")}), []byte(","))
			},
			expected: "xy a,b",
		},
		{
			name: "complex mixed operations",
			buildFn: func(w T) {
				w.WriteLine("Header")
				w.Tab()
				w.AppendPrintf("Value: %d", 42)
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
			buildFn: func(w T) {
				w.WhenWriteLine(true, "included")
				w.WhenWriteLine(false, "skipped")
				w.WhenConcat(true, "a", "b")
				w.WhenJoin(true, []string{"x", "y"}, ",")
			},
			expected: "included\nabx,y",
		},
		{
			name: "edge case - empty operations",
			buildFn: func(w T) {
				w.WriteString("")
				w.Concat()
				w.WriteLines()
				w.Repeat("x", 0)
			},
			expected: "",
		},
		{
			name: "edge case - negative counts",
			buildFn: func(w T) {
				w.NLines(-1)
				w.NTabs(-5)
				w.Repeat("x", -1)
				w.RepeatByte('a', -10)
				w.WriteString("ok")
			},
			expected: "ok",
		},
	}
}

func TestSemanticCorrectness(t *testing.T) {
	t.Run("Builder", func(t *testing.T) {
		runBuildTests(t, func() *Builder { return &Builder{} }, semanticCorrectnessTests[*Builder]())
	})
	t.Run("Buffer", func(t *testing.T) {
		runBuildTests(t, func() *Buffer { return &Buffer{} }, semanticCorrectnessTests[*Buffer]())
	})
}

func TestSemanticEdgeCases(t *testing.T) {
	t.Run("Builder", func(t *testing.T) {
		runBuildTests(t, func() *Builder { return &Builder{} }, semanticEdgeCaseTests[*Builder]())
	})
	t.Run("Buffer", func(t *testing.T) {
		runBuildTests(t, func() *Buffer { return &Buffer{} }, semanticEdgeCaseTests[*Buffer]())
	})
}

func TestNamingConsistency(t *testing.T) {
	t.Run("Builder", func(t *testing.T) {
		runBuildTests(t, func() *Builder { return &Builder{} }, semanticNamingTests[*Builder]())
	})
	t.Run("Buffer", func(t *testing.T) {
		runBuildTests(t, func() *Buffer { return &Buffer{} }, semanticNamingTests[*Buffer]())
	})
}

// TestSemanticEquivalence verifies that Buffer and Builder have identical
// semantics for all shared methods. Both types must produce the same output
// for the same sequence of operations.
func TestSemanticEquivalence(t *testing.T) {
	t.Run("Builder", func(t *testing.T) {
		runBuildTests(t, func() *Builder { return &Builder{} }, semanticEquivalenceTests[*Builder]())
	})
	t.Run("Buffer", func(t *testing.T) {
		runBuildTests(t, func() *Buffer { return &Buffer{} }, semanticEquivalenceTests[*Buffer]())
	})
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
