package strut

import (
	"iter"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// StringWriter is a common interface for Builder and Buffer for testing purposes.
type stringWriter interface { //nolint:interfacebloat
	WriteString(string) (int, error)
	WriteByte(byte) error
	WriteRune(rune) (int, error)
	String() string
	Line()
	Tab()
	NLines(int)
	NTabs(int)
	WriteLine(string)
	WriteLines(...string)
	Append([]byte)
	Join([]string, string)
	Concat(...string)
	Repeat(string, int)
	RepeatByte(byte, int)
	RepeatRune(rune, int)
	RepeatLine(string, int)
	Wprint(...any)
	Wprintf(string, ...any)
	Wprintln(...any)
	WhenWprint(bool, ...any)
	WhenWprintf(bool, string, ...any)
	WhenWprintln(bool, ...any)
	WhenLine(bool)
	WhenTab(bool)
	WhenNLines(bool, int)
	WhenNTabs(bool, int)
	WhenWrite(bool, []byte)
	WhenWriteString(bool, string)
	WhenWriteByte(bool, byte)
	WhenWriteRune(bool, rune)
	WhenWriteLine(bool, string)
	WhenWriteLines(bool, ...string)
	WhenConcat(bool, ...string)
	WhenJoin(bool, []string, string)
	Quote(string)
	QuoteASCII(string)
	QuoteGrapic(string)
	QuoteRune(rune)
	QuoteRuneASCII(rune)
	QuoteRuneGrapic(rune)
	Int(int)
	FormatBool(bool)
	FormatInt64(int64, int)
	FormatUint64(uint64, int)
	FormatFloat(float64, byte, int, int)
	FormatComplex(complex128, byte, int, int)
	WithTrimSpace(string)
	WithTrimRight(string, string)
	WithTrimLeft(string, string)
	WithTrimPrefix(string, string)
	WithTrimSuffix(string, string)
	WithReplaceAll(string, string, string)
	WithReplace(string, string, string, int)
	Extend(iter.Seq[string])
	ExtendLines(iter.Seq[string])
	ExtendJoin(iter.Seq[string], string)
	WriteBytesLine([]byte)
	WriteBytesLines(...[]byte)
	AppendQuote(string)
	AppendQuoteASCII(string)
	AppendQuoteGrapic(string)
	AppendQuoteRune(rune)
	AppendQuoteRuneASCII(rune)
	AppendQuoteRuneGrapic(rune)
	AppendBool(bool)
	AppendInt64(int64, int)
	AppendUint64(uint64, int)
	AppendFloat(float64, byte, int, int)
	AppendTrimSpace([]byte)
	AppendTrimRight([]byte, string)
	AppendTrimLeft([]byte, string)
	AppendTrimPrefix([]byte, []byte)
	AppendTrimSuffix([]byte, []byte)
	AppendReplaceAll([]byte, []byte, []byte)
	AppendReplace([]byte, []byte, []byte, int)
	ExtendBytes(iter.Seq[[]byte])
	ExtendBytesLines(iter.Seq[[]byte])
	ExtendBytesJoin(iter.Seq[[]byte], []byte)
	Len() int
	Cap() int
	Reset()
	Grow(int)
}

// testCase defines a generic test case with a build function and expected result.
type testCase[T stringWriter] struct {
	name     string
	buildFn  func(T)
	expected string
}

// runBuildTests runs a set of test cases against a string writer.
func runBuildTests[T stringWriter](t *testing.T, newWriter func() T, tests []testCase[T]) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter()
			tt.buildFn(w)
			got := w.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// validationTestCase defines a test case with custom validation logic.
type validationTestCase[T stringWriter] struct {
	name     string
	buildFn  func(T)
	validate func(*testing.T, string)
}

// runValidationTests runs test cases with custom validation.
func runValidationTests[T stringWriter](t *testing.T, newWriter func() T, tests []validationTestCase[T]) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter()
			tt.buildFn(w)
			tt.validate(t, w.String())
		})
	}
}

// basicWriteTests returns common test cases for basic write operations.
func basicWriteTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WriteString simple",
			buildFn: func(w T) {
				w.WriteString("hello")
			},
			expected: "hello",
		},
		{
			name: "WriteString unicode",
			buildFn: func(w T) {
				w.WriteString("こんにちは世界")
			},
			expected: "こんにちは世界",
		},
		{
			name: "WriteString empty",
			buildFn: func(w T) {
				w.WriteString("")
			},
			expected: "",
		},
		{
			name: "WriteString emoji",
			buildFn: func(w T) {
				w.WriteString("Hello 🌍🎉🚀")
			},
			expected: "Hello 🌍🎉🚀",
		},
		{
			name: "WriteByte",
			buildFn: func(w T) {
				w.WriteByte('a')
				w.WriteByte('b')
				w.WriteByte('c')
			},
			expected: "abc",
		},
		{
			name: "WriteByte special characters",
			buildFn: func(w T) {
				w.WriteByte('\n')
				w.WriteByte('\t')
				w.WriteByte(' ')
			},
			expected: "\n\t ",
		},
		{
			name: "WriteRune ASCII",
			buildFn: func(w T) {
				w.WriteRune('x')
				w.WriteRune('y')
			},
			expected: "xy",
		},
		{
			name: "WriteRune unicode",
			buildFn: func(w T) {
				w.WriteRune('世')
				w.WriteRune('界')
				w.WriteRune('🎉')
			},
			expected: "世界🎉",
		},
	}
}

// lineTests returns common test cases for line operations.
func lineTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "single line",
			buildFn: func(w T) {
				w.Line()
			},
			expected: "\n",
		},
		{
			name: "single tab",
			buildFn: func(w T) {
				w.Tab()
			},
			expected: "\t",
		},
		{
			name: "NLines positive",
			buildFn: func(w T) {
				w.NLines(3)
			},
			expected: "\n\n\n",
		},
		{
			name: "NTabs positive",
			buildFn: func(w T) {
				w.NTabs(3)
			},
			expected: "\t\t\t",
		},
		{
			name: "NLines zero",
			buildFn: func(w T) {
				w.NLines(0)
			},
			expected: "",
		},
		{
			name: "NTabs zero",
			buildFn: func(w T) {
				w.NTabs(0)
			},
			expected: "",
		},
		{
			name: "NLines negative",
			buildFn: func(w T) {
				w.NLines(-1)
			},
			expected: "",
		},
		{
			name: "NTabs negative",
			buildFn: func(w T) {
				w.NTabs(-1)
			},
			expected: "",
		},
		{
			name: "WriteLine",
			buildFn: func(w T) {
				w.WriteLine("test")
			},
			expected: "test\n",
		},
		{
			name: "WriteLine empty",
			buildFn: func(w T) {
				w.WriteLine("")
			},
			expected: "\n",
		},
		{
			name: "WriteLines",
			buildFn: func(w T) {
				w.WriteLines("first", "second", "third")
			},
			expected: "first\nsecond\nthird\n",
		},
		{
			name: "WriteLines empty",
			buildFn: func(w T) {
				w.WriteLines()
			},
			expected: "",
		},
	}
}

// concatTests returns test cases for concatenation operations.
type concatTestCase struct {
	name     string
	strs     []string
	expected string
}

func concatTests() []concatTestCase {
	return []concatTestCase{
		{
			name:     "multiple strings",
			strs:     []string{"hello", "world"},
			expected: "helloworld",
		},
		{
			name:     "empty strings",
			strs:     []string{"", "", ""},
			expected: "",
		},
		{
			name:     "single string",
			strs:     []string{"alone"},
			expected: "alone",
		},
		{
			name:     "no strings",
			strs:     []string{},
			expected: "",
		},
		{
			name:     "unicode strings",
			strs:     []string{"Hello", "世界", "🌍"},
			expected: "Hello世界🌍",
		},
	}
}

// appendTests returns test cases for append operations.
type appendTestCase struct {
	name     string
	buf      []byte
	expected string
}

func appendTests() []appendTestCase {
	return []appendTestCase{
		{
			name:     "ASCII bytes",
			buf:      []byte("hello"),
			expected: "hello",
		},
		{
			name:     "empty buffer",
			buf:      []byte{},
			expected: "",
		},
		{
			name:     "nil buffer",
			buf:      nil,
			expected: "",
		},
		{
			name:     "UTF-8 bytes",
			buf:      []byte("世界🎉"),
			expected: "世界🎉",
		},
		{
			name:     "binary data",
			buf:      []byte{0, 1, 2, 255},
			expected: string([]byte{0, 1, 2, 255}),
		},
	}
}

// joinTestCase defines test case for join operations.
type joinTestCase struct {
	name     string
	strs     []string
	liminal  string
	expected string
}

func joinTests() []joinTestCase {
	return []joinTestCase{
		{
			name:     "join with comma",
			strs:     []string{"a", "b", "c"},
			liminal:  ",",
			expected: "a,b,c",
		},
		{
			name:     "join with space",
			strs:     []string{"hello", "world"},
			liminal:  " ",
			expected: "hello world",
		},
		{
			name:     "join empty slice",
			strs:     []string{},
			liminal:  ",",
			expected: "",
		},
		{
			name:     "join single element",
			strs:     []string{"alone"},
			liminal:  ",",
			expected: "alone",
		},
		{
			name:     "join with empty separator",
			strs:     []string{"a", "b", "c"},
			liminal:  "",
			expected: "abc",
		},
		{
			name:     "join with unicode separator",
			strs:     []string{"a", "b", "c"},
			liminal:  "→",
			expected: "a→b→c",
		},
		{
			name:     "join empty strings",
			strs:     []string{"", "", ""},
			liminal:  ",",
			expected: ",,",
		},
	}
}

// repeatTests returns test cases for repeat operations.
func repeatTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "Repeat positive",
			buildFn: func(w T) {
				w.Repeat("ab", 3)
			},
			expected: "ababab",
		},
		{
			name: "Repeat zero",
			buildFn: func(w T) {
				w.Repeat("test", 0)
			},
			expected: "",
		},
		{
			name: "Repeat negative",
			buildFn: func(w T) {
				w.Repeat("test", -1)
			},
			expected: "",
		},
		{
			name: "Repeat empty string",
			buildFn: func(w T) {
				w.Repeat("", 5)
			},
			expected: "",
		},
		{
			name: "RepeatByte positive",
			buildFn: func(w T) {
				w.RepeatByte('x', 4)
			},
			expected: "xxxx",
		},
		{
			name: "RepeatByte zero",
			buildFn: func(w T) {
				w.RepeatByte('x', 0)
			},
			expected: "",
		},
		{
			name: "RepeatRune unicode",
			buildFn: func(w T) {
				w.RepeatRune('🎉', 3)
			},
			expected: "🎉🎉🎉",
		},
		{
			name: "RepeatRune negative",
			buildFn: func(w T) {
				w.RepeatRune('a', -5)
			},
			expected: "",
		},
		{
			name: "RepeatLine",
			buildFn: func(w T) {
				w.RepeatLine("test", 2)
			},
			expected: "test\ntest\n",
		},
		{
			name: "RepeatLine zero",
			buildFn: func(w T) {
				w.RepeatLine("test", 0)
			},
			expected: "",
		},
	}
}

// wprintTests returns test cases for formatted print operations.
func wprintTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "Wprint simple",
			buildFn: func(w T) {
				w.Wprint("hello", " ", "world")
			},
			expected: "hello world",
		},
		{
			name: "Wprint numbers",
			buildFn: func(w T) {
				w.Wprint(42, " ", 3.14)
			},
			expected: "42 3.14",
		},
		{
			name: "Wprintf format",
			buildFn: func(w T) {
				w.Wprintf("Hello %s, number %d", "world", 42)
			},
			expected: "Hello world, number 42",
		},
		{
			name: "Wprintf empty",
			buildFn: func(w T) {
				w.Wprintf("")
			},
			expected: "",
		},
		{
			name: "Wprintln",
			buildFn: func(w T) {
				w.Wprintln("test")
			},
			expected: "test\n",
		},
		{
			name: "Wprintln multiple args",
			buildFn: func(w T) {
				w.Wprintln("a", "b", "c")
			},
			expected: "a b c\n",
		},
	}
}

// whenMethodTests returns test cases for When* conditional methods.
func whenMethodTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WhenWprint true",
			buildFn: func(w T) {
				w.WhenWprint(true, "hello")
			},
			expected: "hello",
		},
		{
			name: "WhenWprint false",
			buildFn: func(w T) {
				w.WhenWprint(false, "hello")
			},
			expected: "",
		},
		{
			name: "WhenWprintf true",
			buildFn: func(w T) {
				w.WhenWprintf(true, "num=%d", 42)
			},
			expected: "num=42",
		},
		{
			name: "WhenWprintf false",
			buildFn: func(w T) {
				w.WhenWprintf(false, "num=%d", 42)
			},
			expected: "",
		},
		{
			name: "WhenWprintln true",
			buildFn: func(w T) {
				w.WhenWprintln(true, "test")
			},
			expected: "test\n",
		},
		{
			name: "WhenWprintln false",
			buildFn: func(w T) {
				w.WhenWprintln(false, "test")
			},
			expected: "",
		},
		{
			name: "WhenLine true",
			buildFn: func(w T) {
				w.WhenLine(true)
			},
			expected: "\n",
		},
		{
			name: "WhenLine false",
			buildFn: func(w T) {
				w.WhenLine(false)
			},
			expected: "",
		},
		{
			name: "WhenNLines true",
			buildFn: func(w T) {
				w.WhenNLines(true, 2)
			},
			expected: "\n\n",
		},
		{
			name: "WhenNLines false",
			buildFn: func(w T) {
				w.WhenNLines(false, 2)
			},
			expected: "",
		},
		{
			name: "WhenTab true",
			buildFn: func(w T) {
				w.WhenTab(true)
			},
			expected: "\t",
		},
		{
			name: "WhenTab false",
			buildFn: func(w T) {
				w.WhenTab(false)
			},
			expected: "",
		},
		{
			name: "WhenNTabs true",
			buildFn: func(w T) {
				w.WhenNTabs(true, 2)
			},
			expected: "\t\t",
		},
		{
			name: "WhenNTabs false",
			buildFn: func(w T) {
				w.WhenNTabs(false, 2)
			},
			expected: "",
		},
		{
			name: "WhenWrite true",
			buildFn: func(w T) {
				w.WhenWrite(true, []byte("test"))
			},
			expected: "test",
		},
		{
			name: "WhenWrite false",
			buildFn: func(w T) {
				w.WhenWrite(false, []byte("test"))
			},
			expected: "",
		},
		{
			name: "WhenWriteString true",
			buildFn: func(w T) {
				w.WhenWriteString(true, "hello")
			},
			expected: "hello",
		},
		{
			name: "WhenWriteString false",
			buildFn: func(w T) {
				w.WhenWriteString(false, "hello")
			},
			expected: "",
		},
		{
			name: "WhenWriteByte true",
			buildFn: func(w T) {
				w.WhenWriteByte(true, 'x')
			},
			expected: "x",
		},
		{
			name: "WhenWriteByte false",
			buildFn: func(w T) {
				w.WhenWriteByte(false, 'x')
			},
			expected: "",
		},
		{
			name: "WhenWriteRune true",
			buildFn: func(w T) {
				w.WhenWriteRune(true, '🎉')
			},
			expected: "🎉",
		},
		{
			name: "WhenWriteRune false",
			buildFn: func(w T) {
				w.WhenWriteRune(false, '🎉')
			},
			expected: "",
		},
		{
			name: "WhenWriteLine true",
			buildFn: func(w T) {
				w.WhenWriteLine(true, "test")
			},
			expected: "test\n",
		},
		{
			name: "WhenWriteLine false",
			buildFn: func(w T) {
				w.WhenWriteLine(false, "test")
			},
			expected: "",
		},
		{
			name: "WhenWriteLines true",
			buildFn: func(w T) {
				w.WhenWriteLines(true, "a", "b")
			},
			expected: "a\nb\n",
		},
		{
			name: "WhenWriteLines false",
			buildFn: func(w T) {
				w.WhenWriteLines(false, "a", "b")
			},
			expected: "",
		},
		{
			name: "WhenConcat true",
			buildFn: func(w T) {
				w.WhenConcat(true, "x", "y", "z")
			},
			expected: "xyz",
		},
		{
			name: "WhenConcat false",
			buildFn: func(w T) {
				w.WhenConcat(false, "x", "y", "z")
			},
			expected: "",
		},
		{
			name: "WhenJoin true",
			buildFn: func(w T) {
				w.WhenJoin(true, []string{"a", "b"}, ",")
			},
			expected: "a,b",
		},
		{
			name: "WhenJoin false",
			buildFn: func(w T) {
				w.WhenJoin(false, []string{"a", "b"}, ",")
			},
			expected: "",
		},
	}
}

// quoteTests returns test cases for quote operations.
func quoteTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "Quote simple",
			buildFn: func(w T) {
				w.Quote("hello")
			},
			expected: `"hello"`,
		},
		{
			name: "Quote with special chars",
			buildFn: func(w T) {
				w.Quote("hello\nworld")
			},
			expected: `"hello\nworld"`,
		},
		{
			name: "Quote unicode",
			buildFn: func(w T) {
				w.Quote("世界")
			},
			expected: `"世界"`,
		},
		{
			name: "QuoteASCII unicode",
			buildFn: func(w T) {
				w.QuoteASCII("世界")
			},
			expected: `"\u4e16\u754c"`,
		},
		{
			name: "QuoteGrapic",
			buildFn: func(w T) {
				w.QuoteGrapic("hello\x00world")
			},
			expected: `"hello\x00world"`,
		},
		{
			name: "QuoteRune",
			buildFn: func(w T) {
				w.QuoteRune('a')
			},
			expected: "'a'",
		},
		{
			name: "QuoteRune unicode",
			buildFn: func(w T) {
				w.QuoteRune('世')
			},
			expected: "'世'",
		},
		{
			name: "QuoteRuneASCII",
			buildFn: func(w T) {
				w.QuoteRuneASCII('世')
			},
			expected: `'\u4e16'`,
		},
		{
			name: "QuoteRuneGrapic",
			buildFn: func(w T) {
				w.QuoteRuneGrapic('\n')
			},
			expected: `'\n'`,
		},
	}
}

// formatNumberTests returns test cases for number formatting.
func formatNumberTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "Int positive",
			buildFn: func(w T) {
				w.Int(42)
			},
			expected: "42",
		},
		{
			name: "Int negative",
			buildFn: func(w T) {
				w.Int(-99)
			},
			expected: "-99",
		},
		{
			name: "Int zero",
			buildFn: func(w T) {
				w.Int(0)
			},
			expected: "0",
		},
		{
			name: "FormatBool true",
			buildFn: func(w T) {
				w.FormatBool(true)
			},
			expected: "true",
		},
		{
			name: "FormatBool false",
			buildFn: func(w T) {
				w.FormatBool(false)
			},
			expected: "false",
		},
		{
			name: "FormatInt64 base 10",
			buildFn: func(w T) {
				w.FormatInt64(123, 10)
			},
			expected: "123",
		},
		{
			name: "FormatInt64 base 16",
			buildFn: func(w T) {
				w.FormatInt64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "FormatInt64 base 2",
			buildFn: func(w T) {
				w.FormatInt64(7, 2)
			},
			expected: "111",
		},
		{
			name: "FormatInt64 negative",
			buildFn: func(w T) {
				w.FormatInt64(-42, 10)
			},
			expected: "-42",
		},
		{
			name: "FormatUint64",
			buildFn: func(w T) {
				w.FormatUint64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "FormatFloat",
			buildFn: func(w T) {
				w.FormatFloat(3.14159, 'f', 2, 64)
			},
			expected: "3.14",
		},
		{
			name: "FormatFloat scientific",
			buildFn: func(w T) {
				w.FormatFloat(1000000.0, 'e', 2, 64)
			},
			expected: "1.00e+06",
		},
		{
			name: "FormatFloat negative",
			buildFn: func(w T) {
				w.FormatFloat(-2.5, 'f', 1, 64)
			},
			expected: "-2.5",
		},
		{
			name: "FormatComplex",
			buildFn: func(w T) {
				w.FormatComplex(complex(3, 4), 'f', 1, 128)
			},
			expected: "(3.0+4.0i)",
		},
	}
}

// trimTests returns test cases for trim operations.
func trimTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "TrimSpace",
			buildFn: func(w T) {
				w.WithTrimSpace("  hello  ")
			},
			expected: "hello",
		},
		{
			name: "TrimSpace newlines",
			buildFn: func(w T) {
				w.WithTrimSpace("\n\thello\t\n")
			},
			expected: "hello",
		},
		{
			name: "TrimRight",
			buildFn: func(w T) {
				w.WithTrimRight("hello!!!", "!")
			},
			expected: "hello",
		},
		{
			name: "TrimLeft",
			buildFn: func(w T) {
				w.WithTrimLeft("!!!hello", "!")
			},
			expected: "hello",
		},
		{
			name: "TrimPrefix exists",
			buildFn: func(w T) {
				w.WithTrimPrefix("prefix_content", "prefix_")
			},
			expected: "content",
		},
		{
			name: "TrimPrefix not exists",
			buildFn: func(w T) {
				w.WithTrimPrefix("content", "prefix_")
			},
			expected: "content",
		},
		{
			name: "TrimSuffix exists",
			buildFn: func(w T) {
				w.WithTrimSuffix("content_suffix", "_suffix")
			},
			expected: "content",
		},
		{
			name: "TrimSuffix not exists",
			buildFn: func(w T) {
				w.WithTrimSuffix("content", "_suffix")
			},
			expected: "content",
		},
	}
}

// replaceTests returns test cases for replace operations.
func replaceTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "ReplaceAll",
			buildFn: func(w T) {
				w.WithReplaceAll("hello hello", "hello", "hi") //nolint:dupword
			},
			expected: "hi hi", //nolint:dupword
		},
		{
			name: "ReplaceAll no match",
			buildFn: func(w T) {
				w.WithReplaceAll("hello", "world", "hi")
			},
			expected: "hello",
		},
		{
			name: "Replace limited",
			buildFn: func(w T) {
				w.WithReplace("a a a a", "a", "b", 2) //nolint:dupword
			},
			expected: "b b a a", //nolint:dupword
		},
		{
			name: "Replace zero",
			buildFn: func(w T) {
				w.WithReplace("hello", "l", "x", 0)
			},
			expected: "hello",
		},
		{
			name: "Replace negative (all)",
			buildFn: func(w T) {
				w.WithReplace("a a a", "a", "b", -1) //nolint:dupword
			},
			expected: "b b b", //nolint:dupword
		},
	}
}

// edgeCaseTests returns test cases for edge cases.
func edgeCaseTests[T stringWriter]() []validationTestCase[T] {
	return []validationTestCase[T]{
		{
			name: "mixed operations",
			buildFn: func(w T) {
				w.WriteString("Hello")
				w.WriteByte(' ')
				w.WriteRune('世')
				w.Line()
				w.Int(42)
			},
			validate: func(t *testing.T, got string) {
				expected := "Hello 世\n42"
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			},
		},
		{
			name: "multiple conditional writes",
			buildFn: func(w T) {
				w.WhenWriteString(true, "yes1")
				w.WhenWriteString(false, "no")
				w.WhenWriteString(true, "yes2")
			},
			validate: func(t *testing.T, got string) {
				expected := "yes1yes2"
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			},
		},
		{
			name: "empty builder",
			buildFn: func(w T) {
				// Do nothing
			},
			validate: func(t *testing.T, got string) {
				if got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
			},
		},
		{
			name: "large repeat count",
			buildFn: func(w T) {
				w.Repeat("x", 1000)
			},
			validate: func(t *testing.T, got string) {
				if len(got) != 1000 {
					t.Errorf("expected length 1000, got %d", len(got))
				}
				if strings.Count(got, "x") != 1000 {
					t.Errorf("expected 1000 'x' characters")
				}
			},
		},
		{
			name: "non-UTF-8 bytes",
			buildFn: func(w T) {
				w.Append([]byte{0xFF, 0xFE})
			},
			validate: func(t *testing.T, got string) {
				if !utf8.ValidString(got) {
					// This is expected - we're testing that invalid UTF-8 is preserved
					if len(got) != 2 {
						t.Errorf("expected length 2, got %d", len(got))
					}
				}
			},
		},
		{
			name: "complex number formatting",
			buildFn: func(w T) {
				w.FormatComplex(complex(0, 0), 'f', 0, 128)
			},
			validate: func(t *testing.T, got string) {
				expected := "(0+0i)"
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			},
		},
		{
			name: "join with unicode separator",
			buildFn: func(w T) {
				w.Join([]string{"🌍", "🎉", "🚀"}, " → ")
			},
			validate: func(t *testing.T, got string) {
				expected := "🌍 → 🎉 → 🚀"
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			},
		},
		{
			name: "nested conditionals effect",
			buildFn: func(w T) {
				for i := range 3 {
					w.WhenWriteLine(i%2 == 0, "even")
				}
			},
			validate: func(t *testing.T, got string) {
				expected := "even\neven\n"
				if got != expected {
					t.Errorf("expected %q, got %q", expected, got)
				}
			},
		},
	}
}

// extendTestsBuilderOnly returns test cases specific to types with Extend methods.
type extendTestBuilderOnly[T interface {
	stringWriter
	Extend(iter.Seq[string])
	ExtendLines(iter.Seq[string])
	ExtendJoin(iter.Seq[string], string)
}] struct {
	name     string
	buildFn  func(T)
	expected string
}

func extendTests[T interface {
	stringWriter
	Extend(iter.Seq[string])
	ExtendLines(iter.Seq[string])
	ExtendJoin(iter.Seq[string], string)
}]() []extendTestBuilderOnly[T] {
	return []extendTestBuilderOnly[T]{
		{
			name: "Extend with values",
			buildFn: func(w T) {
				w.Extend(slices.Values([]string{"a", "b", "c"}))
			},
			expected: "abc",
		},
		{
			name: "Extend empty",
			buildFn: func(w T) {
				w.Extend(slices.Values([]string{}))
			},
			expected: "",
		},
		{
			name: "ExtendLines",
			buildFn: func(w T) {
				w.ExtendLines(slices.Values([]string{"line1", "line2"}))
			},
			expected: "line1\nline2\n",
		},
		{
			name: "ExtendJoin",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"a", "b", "c"}), ",")
			},
			expected: "a,b,c",
		},
		{
			name: "ExtendJoin single element",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{"alone"}), ",")
			},
			expected: "alone",
		},
		{
			name: "ExtendJoin empty",
			buildFn: func(w T) {
				w.ExtendJoin(slices.Values([]string{}), ",")
			},
			expected: "",
		},
	}
}

func writeBytesLineTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WriteBytesLine simple",
			buildFn: func(w T) {
				w.WriteBytesLine([]byte("hello"))
			},
			expected: "hello\n",
		},
		{
			name: "WriteBytesLine empty",
			buildFn: func(w T) {
				w.WriteBytesLine([]byte{})
			},
			expected: "\n",
		},
		{
			name: "WriteBytesLine unicode",
			buildFn: func(w T) {
				w.WriteBytesLine([]byte("世界"))
			},
			expected: "世界\n",
		},
		{
			name: "WriteBytesLines multiple",
			buildFn: func(w T) {
				w.WriteBytesLines([]byte("first"), []byte("second"), []byte("third"))
			},
			expected: "first\nsecond\nthird\n",
		},
		{
			name: "WriteBytesLines empty",
			buildFn: func(w T) {
				w.WriteBytesLines()
			},
			expected: "",
		},
		{
			name: "WriteBytesLines with empty bytes",
			buildFn: func(w T) {
				w.WriteBytesLines([]byte(""), []byte("middle"), []byte(""))
			},
			expected: "\nmiddle\n\n",
		},
	}
}

func appendQuoteTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "AppendQuote simple",
			buildFn: func(w T) {
				w.AppendQuote("hello")
			},
			expected: `"hello"`,
		},
		{
			name: "AppendQuote with newline",
			buildFn: func(w T) {
				w.AppendQuote("hello\nworld")
			},
			expected: `"hello\nworld"`,
		},
		{
			name: "AppendQuote with tab",
			buildFn: func(w T) {
				w.AppendQuote("hello\tworld")
			},
			expected: `"hello\tworld"`,
		},
		{
			name: "AppendQuote unicode",
			buildFn: func(w T) {
				w.AppendQuote("世界")
			},
			expected: `"世界"`,
		},
		{
			name: "AppendQuoteASCII unicode",
			buildFn: func(w T) {
				w.AppendQuoteASCII("世界")
			},
			expected: `"\u4e16\u754c"`,
		},
		{
			name: "AppendQuoteGrapic control char",
			buildFn: func(w T) {
				w.AppendQuoteGrapic("hello\x00world")
			},
			expected: `"hello\x00world"`,
		},
		{
			name: "AppendQuoteRune ASCII",
			buildFn: func(w T) {
				w.AppendQuoteRune('a')
			},
			expected: "'a'",
		},
		{
			name: "AppendQuoteRune unicode",
			buildFn: func(w T) {
				w.AppendQuoteRune('世')
			},
			expected: "'世'",
		},
		{
			name: "AppendQuoteRune newline",
			buildFn: func(w T) {
				w.AppendQuoteRune('\n')
			},
			expected: `'\n'`,
		},
		{
			name: "AppendQuoteRuneASCII unicode",
			buildFn: func(w T) {
				w.AppendQuoteRuneASCII('世')
			},
			expected: `'\u4e16'`,
		},
		{
			name: "AppendQuoteRuneGrapic control char",
			buildFn: func(w T) {
				w.AppendQuoteRuneGrapic('\x00')
			},
			expected: `'\x00'`,
		},
	}
}

func appendNumberTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "AppendBool true",
			buildFn: func(w T) {
				w.AppendBool(true)
			},
			expected: "true",
		},
		{
			name: "AppendBool false",
			buildFn: func(w T) {
				w.AppendBool(false)
			},
			expected: "false",
		},
		{
			name: "AppendInt64 positive base 10",
			buildFn: func(w T) {
				w.AppendInt64(42, 10)
			},
			expected: "42",
		},
		{
			name: "AppendInt64 negative",
			buildFn: func(w T) {
				w.AppendInt64(-99, 10)
			},
			expected: "-99",
		},
		{
			name: "AppendInt64 zero",
			buildFn: func(w T) {
				w.AppendInt64(0, 10)
			},
			expected: "0",
		},
		{
			name: "AppendInt64 base 16",
			buildFn: func(w T) {
				w.AppendInt64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "AppendInt64 base 2",
			buildFn: func(w T) {
				w.AppendInt64(7, 2)
			},
			expected: "111",
		},
		{
			name: "AppendUint64 base 10",
			buildFn: func(w T) {
				w.AppendUint64(42, 10)
			},
			expected: "42",
		},
		{
			name: "AppendUint64 base 16",
			buildFn: func(w T) {
				w.AppendUint64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "AppendUint64 zero",
			buildFn: func(w T) {
				w.AppendUint64(0, 10)
			},
			expected: "0",
		},
		{
			name: "AppendFloat positive",
			buildFn: func(w T) {
				w.AppendFloat(3.14159, 'f', 2, 64)
			},
			expected: "3.14",
		},
		{
			name: "AppendFloat negative",
			buildFn: func(w T) {
				w.AppendFloat(-2.5, 'f', 1, 64)
			},
			expected: "-2.5",
		},
		{
			name: "AppendFloat scientific",
			buildFn: func(w T) {
				w.AppendFloat(1000000.0, 'e', 2, 64)
			},
			expected: "1.00e+06",
		},
		{
			name: "AppendFloat zero",
			buildFn: func(w T) {
				w.AppendFloat(0.0, 'f', 1, 64)
			},
			expected: "0.0",
		},
		{
			name: "mixed append operations",
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
	}
}

func appendTrimTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "AppendTrimSpace leading and trailing",
			buildFn: func(w T) {
				w.AppendTrimSpace([]byte("  hello  "))
			},
			expected: "hello",
		},
		{
			name: "AppendTrimSpace newlines and tabs",
			buildFn: func(w T) {
				w.AppendTrimSpace([]byte("\n\thello\t\n"))
			},
			expected: "hello",
		},
		{
			name: "AppendTrimSpace empty",
			buildFn: func(w T) {
				w.AppendTrimSpace([]byte("   "))
			},
			expected: "",
		},
		{
			name: "AppendTrimRight",
			buildFn: func(w T) {
				w.AppendTrimRight([]byte("hello!!!"), "!")
			},
			expected: "hello",
		},
		{
			name: "AppendTrimRight multiple chars",
			buildFn: func(w T) {
				w.AppendTrimRight([]byte("hello123"), "321")
			},
			expected: "hello",
		},
		{
			name: "AppendTrimLeft",
			buildFn: func(w T) {
				w.AppendTrimLeft([]byte("!!!hello"), "!")
			},
			expected: "hello",
		},
		{
			name: "AppendTrimLeft multiple chars",
			buildFn: func(w T) {
				w.AppendTrimLeft([]byte("123hello"), "321")
			},
			expected: "hello",
		},
		{
			name: "AppendTrimPrefix exists",
			buildFn: func(w T) {
				w.AppendTrimPrefix([]byte("prefix_content"), []byte("prefix_"))
			},
			expected: "content",
		},
		{
			name: "AppendTrimPrefix not exists",
			buildFn: func(w T) {
				w.AppendTrimPrefix([]byte("content"), []byte("prefix_"))
			},
			expected: "content",
		},
		{
			name: "AppendTrimSuffix exists",
			buildFn: func(w T) {
				w.AppendTrimSuffix([]byte("content_suffix"), []byte("_suffix"))
			},
			expected: "content",
		},
		{
			name: "AppendTrimSuffix not exists",
			buildFn: func(w T) {
				w.AppendTrimSuffix([]byte("content"), []byte("_suffix"))
			},
			expected: "content",
		},
	}
}

func appendReplaceTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "AppendReplaceAll",
			buildFn: func(w T) {
				w.AppendReplaceAll([]byte("hello hello"), []byte("hello"), []byte("hi")) //nolint:dupword
			},
			expected: "hi hi", //nolint:dupword
		},
		{
			name: "AppendReplaceAll no match",
			buildFn: func(w T) {
				w.AppendReplaceAll([]byte("hello"), []byte("world"), []byte("hi"))
			},
			expected: "hello",
		},
		{
			name: "AppendReplaceAll with empty",
			buildFn: func(w T) {
				w.AppendReplaceAll([]byte("abc"), []byte("b"), []byte(""))
			},
			expected: "ac",
		},
		{
			name: "AppendReplace limited",
			buildFn: func(w T) {
				w.AppendReplace([]byte("a a a a"), []byte("a"), []byte("b"), 2) //nolint:dupword
			},
			expected: "b b a a", //nolint:dupword
		},
		{
			name: "AppendReplace zero",
			buildFn: func(w T) {
				w.AppendReplace([]byte("hello"), []byte("l"), []byte("x"), 0)
			},
			expected: "hello",
		},
		{
			name: "AppendReplace negative (all)",
			buildFn: func(w T) {
				w.AppendReplace([]byte("a a a"), []byte("a"), []byte("b"), -1) //nolint:dupword
			},
			expected: "b b b", //nolint:dupword
		},
		{
			name: "AppendReplace one",
			buildFn: func(w T) {
				w.AppendReplace([]byte("hello"), []byte("l"), []byte("L"), 1)
			},
			expected: "heLlo",
		},
	}
}

func extendBytesTests[T stringWriter]() []testCase[T] {
	return []testCase[T]{
		{
			name: "ExtendBytes with values",
			buildFn: func(w T) {
				w.ExtendBytes(slices.Values([][]byte{[]byte("a"), []byte("b"), []byte("c")}))
			},
			expected: "abc",
		},
		{
			name: "ExtendBytes empty",
			buildFn: func(w T) {
				w.ExtendBytes(slices.Values([][]byte{}))
			},
			expected: "",
		},
		{
			name: "ExtendBytes with unicode",
			buildFn: func(w T) {
				w.ExtendBytes(slices.Values([][]byte{[]byte("世"), []byte("界")}))
			},
			expected: "世界",
		},
		{
			name: "ExtendBytesLines",
			buildFn: func(w T) {
				w.ExtendBytesLines(slices.Values([][]byte{[]byte("line1"), []byte("line2")}))
			},
			expected: "line1\nline2\n",
		},
		{
			name: "ExtendBytesLines empty",
			buildFn: func(w T) {
				w.ExtendBytesLines(slices.Values([][]byte{}))
			},
			expected: "",
		},
		{
			name: "ExtendBytesJoin",
			buildFn: func(w T) {
				w.ExtendBytesJoin(slices.Values([][]byte{[]byte("a"), []byte("b"), []byte("c")}), []byte(","))
			},
			expected: "a,b,c",
		},
		{
			name: "ExtendBytesJoin single element",
			buildFn: func(w T) {
				w.ExtendBytesJoin(slices.Values([][]byte{[]byte("alone")}), []byte(","))
			},
			expected: "alone",
		},
		{
			name: "ExtendBytesJoin empty",
			buildFn: func(w T) {
				w.ExtendBytesJoin(slices.Values([][]byte{}), []byte(","))
			},
			expected: "",
		},
		{
			name: "ExtendBytesJoin empty separator",
			buildFn: func(w T) {
				w.ExtendBytesJoin(slices.Values([][]byte{[]byte("a"), []byte("b")}), []byte(""))
			},
			expected: "ab",
		},
	}
}
