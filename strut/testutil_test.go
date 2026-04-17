package strut

import (
	"iter"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// StringWriter is a common interface for Builder and Buffer for testing purposes.
type stringWriter[T any] interface { //nolint:interfacebloat
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
	PushBytes([]byte)
	Join([]string, string)
	Concat(...string)
	Repeat(string, int)
	RepeatByte(byte, int)
	RepeatRune(rune, int)
	RepeatLine(string, int)
	Bprint(...any) T
	Bprintf(string, ...any) T
	Bprintln(...any) T
	WhenBprint(bool, ...any) T
	WhenBprintf(bool, string, ...any) T
	WhenBprintln(bool, ...any) T
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
	PushInt(int)
	PushBool(bool)
	PushInt64(int64, int)
	PushUint64(uint64, int)
	PushFloat(float64, byte, int, int)
	PushComplex(complex128, byte, int, int)
	WithTrimSpace(string)
	WithTrim(string, string)
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
	PushQuote(string)
	PushQuoteASCII(string)
	PushQuoteGraphic(string)
	PushQuoteRune(rune)
	PushQuoteRuneASCII(rune)
	PushQuoteRuneGraphic(rune)
	PushTrimSpace([]byte)
	PushTrim([]byte, string)
	PushTrimRight([]byte, string)
	PushTrimLeft([]byte, string)
	PushTrimPrefix([]byte, []byte)
	PushTrimSuffix([]byte, []byte)
	PushReplaceAll([]byte, []byte, []byte)
	PushReplace([]byte, []byte, []byte, int)
	ExtendBytes(iter.Seq[[]byte])
	ExtendBytesLines(iter.Seq[[]byte])
	ExtendBytesJoin(iter.Seq[[]byte], []byte)
	ExtendMutable(iter.Seq[Mutable])
	ExtendMutableLines(iter.Seq[Mutable])
	ExtendMutableJoin(iter.Seq[Mutable], Mutable)
	WriteMutable(Mutable)
	WriteMutableLine(Mutable)
	WriteMutableLines(...Mutable)
	WhenWriteMutable(bool, Mutable)
	WhenWriteMutableLine(bool, Mutable)
	WhenWriteMutableLines(bool, ...Mutable)
	Len() int
	Cap() int
	Reset()
	Grow(int)
}

// testCase defines a generic test case with a build function and expected result.
type testCase[T stringWriter[T]] struct {
	name     string
	buildFn  func(T)
	expected string
}

// runBuildTests runs a set of test cases against a string writer.
func runBuildTests[T stringWriter[T]](t *testing.T, newWriter func() T, tests []testCase[T]) {
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
type validationTestCase[T stringWriter[T]] struct {
	name     string
	buildFn  func(T)
	validate func(*testing.T, string)
}

// runValidationTests runs test cases with custom validation.
func runValidationTests[T stringWriter[T]](t *testing.T, newWriter func() T, tests []validationTestCase[T]) {
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
func basicWriteTests[T stringWriter[T]]() []testCase[T] {
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
func lineTests[T stringWriter[T]]() []testCase[T] {
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
func repeatTests[T stringWriter[T]]() []testCase[T] {
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
func wprintTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "Bprint simple",
			buildFn: func(w T) {
				w.Bprint("hello", " ", "world")
			},
			expected: "hello world",
		},
		{
			name: "Bprint numbers",
			buildFn: func(w T) {
				w.Bprint(42, " ", 3.14)
			},
			expected: "42 3.14",
		},
		{
			name: "Bprintf format",
			buildFn: func(w T) {
				w.Bprintf("Hello %s, number %d", "world", 42)
			},
			expected: "Hello world, number 42",
		},
		{
			name: "Bprintf empty",
			buildFn: func(w T) {
				w.Bprintf("")
			},
			expected: "",
		},
		{
			name: "Bprintln",
			buildFn: func(w T) {
				w.Bprintln("test")
			},
			expected: "test\n",
		},
		{
			name: "Bprintln multiple args",
			buildFn: func(w T) {
				w.Bprintln("a", "b", "c")
			},
			expected: "a b c\n",
		},
	}
}

// whenMethodTests returns test cases for When* conditional methods.
func whenMethodTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WhenBprint true",
			buildFn: func(w T) {
				w.WhenBprint(true, "hello")
			},
			expected: "hello",
		},
		{
			name: "WhenBprint false",
			buildFn: func(w T) {
				w.WhenBprint(false, "hello")
			},
			expected: "",
		},
		{
			name: "WhenBprintf true",
			buildFn: func(w T) {
				w.WhenBprintf(true, "num=%d", 42)
			},
			expected: "num=42",
		},
		{
			name: "WhenBprintf false",
			buildFn: func(w T) {
				w.WhenBprintf(false, "num=%d", 42)
			},
			expected: "",
		},
		{
			name: "WhenBprintln true",
			buildFn: func(w T) {
				w.WhenBprintln(true, "test")
			},
			expected: "test\n",
		},
		{
			name: "WhenBprintln false",
			buildFn: func(w T) {
				w.WhenBprintln(false, "test")
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

// pushQuoteTests returns test cases for quote operations.
func pushQuoteTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "PushQuote simple",
			buildFn: func(w T) {
				w.PushQuote("hello")
			},
			expected: `"hello"`,
		},
		{
			name: "PushQuote with newline",
			buildFn: func(w T) {
				w.PushQuote("hello\nworld")
			},
			expected: `"hello\nworld"`,
		},
		{
			name: "PushQuote with tab",
			buildFn: func(w T) {
				w.PushQuote("hello\tworld")
			},
			expected: `"hello\tworld"`,
		},
		{
			name: "PushQuote unicode",
			buildFn: func(w T) {
				w.PushQuote("世界")
			},
			expected: `"世界"`,
		},
		{
			name: "PushQuoteASCII unicode",
			buildFn: func(w T) {
				w.PushQuoteASCII("世界")
			},
			expected: `"\u4e16\u754c"`,
		},
		{
			name: "PushQuoteGraphic",
			buildFn: func(w T) {
				w.PushQuoteGraphic("hello\x00world")
			},
			expected: `"hello\x00world"`,
		},
		{
			name: "PushQuoteRune ASCII",
			buildFn: func(w T) {
				w.PushQuoteRune('a')
			},
			expected: "'a'",
		},
		{
			name: "PushQuoteRune unicode",
			buildFn: func(w T) {
				w.PushQuoteRune('世')
			},
			expected: "'世'",
		},
		{
			name: "PushQuoteRune newline",
			buildFn: func(w T) {
				w.PushQuoteRune('\n')
			},
			expected: `'\n'`,
		},
		{
			name: "PushQuoteRuneASCII unicode",
			buildFn: func(w T) {
				w.PushQuoteRuneASCII('世')
			},
			expected: `'\u4e16'`,
		},
		{
			name: "PushQuoteRuneGraphic newline",
			buildFn: func(w T) {
				w.PushQuoteRuneGraphic('\n')
			},
			expected: `'\n'`,
		},
		{
			name: "PushQuoteRuneGraphic control char",
			buildFn: func(w T) {
				w.PushQuoteRuneGraphic('\x00')
			},
			expected: `'\x00'`,
		},
	}
}

// pushNumberTests returns test cases for numeric push operations.
func pushNumberTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name:     "Int positive",
			buildFn:  func(w T) { w.PushInt(42) },
			expected: "42",
		},
		{
			name:     "Int negative",
			buildFn:  func(w T) { w.PushInt(-99) },
			expected: "-99",
		},
		{
			name:     "Int zero",
			buildFn:  func(w T) { w.PushInt(0) },
			expected: "0",
		},
		{
			name:     "PushBool true",
			buildFn:  func(w T) { w.PushBool(true) },
			expected: "true",
		},
		{
			name:     "PushBool false",
			buildFn:  func(w T) { w.PushBool(false) },
			expected: "false",
		},
		{
			name:     "PushInt64 base 10",
			buildFn:  func(w T) { w.PushInt64(42, 10) },
			expected: "42",
		},
		{
			name:     "PushInt64 base 16",
			buildFn:  func(w T) { w.PushInt64(255, 16) },
			expected: "ff",
		},
		{
			name:     "PushInt64 base 2",
			buildFn:  func(w T) { w.PushInt64(7, 2) },
			expected: "111",
		},
		{
			name:     "PushInt64 negative",
			buildFn:  func(w T) { w.PushInt64(-42, 10) },
			expected: "-42",
		},
		{
			name:     "PushInt64 zero",
			buildFn:  func(w T) { w.PushInt64(0, 10) },
			expected: "0",
		},
		{
			name:     "PushUint64 base 10",
			buildFn:  func(w T) { w.PushUint64(42, 10) },
			expected: "42",
		},
		{
			name:     "PushUint64 base 16",
			buildFn:  func(w T) { w.PushUint64(255, 16) },
			expected: "ff",
		},
		{
			name:     "PushUint64 zero",
			buildFn:  func(w T) { w.PushUint64(0, 10) },
			expected: "0",
		},
		{
			name:     "PushFloat fixed",
			buildFn:  func(w T) { w.PushFloat(3.14159, 'f', 2, 64) },
			expected: "3.14",
		},
		{
			name:     "PushFloat scientific",
			buildFn:  func(w T) { w.PushFloat(1000000.0, 'e', 2, 64) },
			expected: "1.00e+06",
		},
		{
			name:     "PushFloat negative",
			buildFn:  func(w T) { w.PushFloat(-2.5, 'f', 1, 64) },
			expected: "-2.5",
		},
		{
			name:     "PushFloat zero",
			buildFn:  func(w T) { w.PushFloat(0.0, 'f', 1, 64) },
			expected: "0.0",
		},
		{
			name: "mixed push operations",
			buildFn: func(w T) {
				w.PushBool(true)
				w.WriteByte(' ')
				w.PushInt64(42, 10)
				w.WriteByte(' ')
				w.PushUint64(255, 16)
				w.WriteByte(' ')
				w.PushFloat(3.14, 'f', 2, 64)
			},
			expected: "true 42 ff 3.14",
		},
		{
			name: "PushComplex",
			buildFn: func(w T) {
				w.PushComplex(complex(3, 4), 'f', 1, 128)
			},
			expected: "(3.0+4.0i)",
		},
	}
}

// trimTests returns test cases for trim operations.
func trimTests[T stringWriter[T]]() []testCase[T] {
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
			name: "Trim both sides",
			buildFn: func(w T) {
				w.WithTrim("!!!hello!!!", "!")
			},
			expected: "hello",
		},
		{
			name: "Trim multiple chars",
			buildFn: func(w T) {
				w.WithTrim("123hello321", "321")
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
func replaceTests[T stringWriter[T]]() []testCase[T] {
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
func edgeCaseTests[T stringWriter[T]]() []validationTestCase[T] {
	return []validationTestCase[T]{
		{
			name: "mixed operations",
			buildFn: func(w T) {
				w.WriteString("Hello")
				w.WriteByte(' ')
				w.WriteRune('世')
				w.Line()
				w.PushInt(42)
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
				w.PushBytes([]byte{0xFF, 0xFE})
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
				w.PushComplex(complex(0, 0), 'f', 0, 128)
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
	stringWriter[T]
	Extend(iter.Seq[string])
	ExtendLines(iter.Seq[string])
	ExtendJoin(iter.Seq[string], string)
}] struct {
	name     string
	buildFn  func(T)
	expected string
}

func extendTests[T interface {
	stringWriter[T]
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

func writeBytesLineTests[T stringWriter[T]]() []testCase[T] {
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

func appendTrimTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "PushTrimSpace leading and trailing",
			buildFn: func(w T) {
				w.PushTrimSpace([]byte("  hello  "))
			},
			expected: "hello",
		},
		{
			name: "PushTrimSpace newlines and tabs",
			buildFn: func(w T) {
				w.PushTrimSpace([]byte("\n\thello\t\n"))
			},
			expected: "hello",
		},
		{
			name: "PushTrimSpace empty",
			buildFn: func(w T) {
				w.PushTrimSpace([]byte("   "))
			},
			expected: "",
		},
		{
			name: "PushTrim both sides",
			buildFn: func(w T) {
				w.PushTrim([]byte("!!!hello!!!"), "!")
			},
			expected: "hello",
		},
		{
			name: "PushTrim multiple chars",
			buildFn: func(w T) {
				w.PushTrim([]byte("123hello321"), "321")
			},
			expected: "hello",
		},
		{
			name: "PushTrimRight",
			buildFn: func(w T) {
				w.PushTrimRight([]byte("hello!!!"), "!")
			},
			expected: "hello",
		},
		{
			name: "PushTrimRight multiple chars",
			buildFn: func(w T) {
				w.PushTrimRight([]byte("hello123"), "321")
			},
			expected: "hello",
		},
		{
			name: "PushTrimLeft",
			buildFn: func(w T) {
				w.PushTrimLeft([]byte("!!!hello"), "!")
			},
			expected: "hello",
		},
		{
			name: "PushTrimLeft multiple chars",
			buildFn: func(w T) {
				w.PushTrimLeft([]byte("123hello"), "321")
			},
			expected: "hello",
		},
		{
			name: "PushTrimPrefix exists",
			buildFn: func(w T) {
				w.PushTrimPrefix([]byte("prefix_content"), []byte("prefix_"))
			},
			expected: "content",
		},
		{
			name: "PushTrimPrefix not exists",
			buildFn: func(w T) {
				w.PushTrimPrefix([]byte("content"), []byte("prefix_"))
			},
			expected: "content",
		},
		{
			name: "PushTrimSuffix exists",
			buildFn: func(w T) {
				w.PushTrimSuffix([]byte("content_suffix"), []byte("_suffix"))
			},
			expected: "content",
		},
		{
			name: "PushTrimSuffix not exists",
			buildFn: func(w T) {
				w.PushTrimSuffix([]byte("content"), []byte("_suffix"))
			},
			expected: "content",
		},
	}
}

func appendReplaceTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "PushReplaceAll",
			buildFn: func(w T) {
				w.PushReplaceAll([]byte("hello hello"), []byte("hello"), []byte("hi")) //nolint:dupword
			},
			expected: "hi hi", //nolint:dupword
		},
		{
			name: "PushReplaceAll no match",
			buildFn: func(w T) {
				w.PushReplaceAll([]byte("hello"), []byte("world"), []byte("hi"))
			},
			expected: "hello",
		},
		{
			name: "PushReplaceAll with empty",
			buildFn: func(w T) {
				w.PushReplaceAll([]byte("abc"), []byte("b"), []byte(""))
			},
			expected: "ac",
		},
		{
			name: "PushReplace limited",
			buildFn: func(w T) {
				w.PushReplace([]byte("a a a a"), []byte("a"), []byte("b"), 2) //nolint:dupword
			},
			expected: "b b a a", //nolint:dupword
		},
		{
			name: "PushReplace zero",
			buildFn: func(w T) {
				w.PushReplace([]byte("hello"), []byte("l"), []byte("x"), 0)
			},
			expected: "hello",
		},
		{
			name: "PushReplace negative (all)",
			buildFn: func(w T) {
				w.PushReplace([]byte("a a a"), []byte("a"), []byte("b"), -1) //nolint:dupword
			},
			expected: "b b b", //nolint:dupword
		},
		{
			name: "PushReplace one",
			buildFn: func(w T) {
				w.PushReplace([]byte("hello"), []byte("l"), []byte("L"), 1)
			},
			expected: "heLlo",
		},
	}
}

func extendBytesTests[T stringWriter[T]]() []testCase[T] {
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

func extendMutableTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "ExtendMutable with values",
			buildFn: func(w T) {
				w.ExtendMutable(slices.Values([]Mutable{Mutable("a"), Mutable("b"), Mutable("c")}))
			},
			expected: "abc",
		},
		{
			name: "ExtendMutable empty",
			buildFn: func(w T) {
				w.ExtendMutable(slices.Values([]Mutable{}))
			},
			expected: "",
		},
		{
			name: "ExtendMutable unicode",
			buildFn: func(w T) {
				w.ExtendMutable(slices.Values([]Mutable{Mutable("世"), Mutable("界")}))
			},
			expected: "世界",
		},
		{
			name: "ExtendMutableLines basic",
			buildFn: func(w T) {
				w.ExtendMutableLines(slices.Values([]Mutable{Mutable("line1"), Mutable("line2")}))
			},
			expected: "line1\nline2\n",
		},
		{
			name: "ExtendMutableLines empty",
			buildFn: func(w T) {
				w.ExtendMutableLines(slices.Values([]Mutable{}))
			},
			expected: "",
		},
		{
			name: "ExtendMutableJoin multi-element",
			buildFn: func(w T) {
				w.ExtendMutableJoin(slices.Values([]Mutable{Mutable("a"), Mutable("b"), Mutable("c")}), Mutable(","))
			},
			expected: "a,b,c",
		},
		{
			name: "ExtendMutableJoin single",
			buildFn: func(w T) {
				w.ExtendMutableJoin(slices.Values([]Mutable{Mutable("alone")}), Mutable(","))
			},
			expected: "alone",
		},
		{
			name: "ExtendMutableJoin empty",
			buildFn: func(w T) {
				w.ExtendMutableJoin(slices.Values([]Mutable{}), Mutable(","))
			},
			expected: "",
		},
		{
			name: "ExtendMutableJoin empty sep",
			buildFn: func(w T) {
				w.ExtendMutableJoin(slices.Values([]Mutable{Mutable("a"), Mutable("b")}), Mutable(""))
			},
			expected: "ab",
		},
	}
}

func writeMutableLinesTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WriteMutable simple",
			buildFn: func(w T) {
				w.WriteMutable(Mutable("hello"))
			},
			expected: "hello",
		},
		{
			name: "WriteMutable empty",
			buildFn: func(w T) {
				w.WriteMutable(Mutable(""))
			},
			expected: "",
		},
		{
			name: "WriteMutable unicode",
			buildFn: func(w T) {
				w.WriteMutable(Mutable("世界"))
			},
			expected: "世界",
		},
		{
			name: "WriteMutableLine simple",
			buildFn: func(w T) {
				w.WriteMutableLine(Mutable("hello"))
			},
			expected: "hello\n",
		},
		{
			name: "WriteMutableLine empty",
			buildFn: func(w T) {
				w.WriteMutableLine(Mutable(""))
			},
			expected: "\n",
		},
		{
			name: "WriteMutableLines multiple",
			buildFn: func(w T) {
				w.WriteMutableLines(Mutable("first"), Mutable("second"), Mutable("third"))
			},
			expected: "first\nsecond\nthird\n",
		},
		{
			name: "WriteMutableLines single",
			buildFn: func(w T) {
				w.WriteMutableLines(Mutable("only"))
			},
			expected: "only\n",
		},
		{
			name: "WriteMutableLines none",
			buildFn: func(w T) {
				w.WriteMutableLines()
			},
			expected: "",
		},
		{
			name: "WriteMutableLines with empty element",
			buildFn: func(w T) {
				w.WriteMutableLines(Mutable("a"), Mutable(""), Mutable("b"))
			},
			expected: "a\n\nb\n",
		},
	}
}

func whenWriteMutableTests[T stringWriter[T]]() []testCase[T] {
	return []testCase[T]{
		{
			name: "WhenWriteMutable true",
			buildFn: func(w T) {
				w.WhenWriteMutable(true, Mutable("hello"))
			},
			expected: "hello",
		},
		{
			name: "WhenWriteMutable false",
			buildFn: func(w T) {
				w.WhenWriteMutable(false, Mutable("hello"))
			},
			expected: "",
		},
		{
			name: "WhenWriteMutable true empty",
			buildFn: func(w T) {
				w.WhenWriteMutable(true, Mutable(""))
			},
			expected: "",
		},
		{
			name: "WhenWriteMutable true unicode",
			buildFn: func(w T) {
				w.WhenWriteMutable(true, Mutable("世界"))
			},
			expected: "世界",
		},
		{
			name: "WhenWriteMutableLine true",
			buildFn: func(w T) {
				w.WhenWriteMutableLine(true, Mutable("hello"))
			},
			expected: "hello\n",
		},
		{
			name: "WhenWriteMutableLine false",
			buildFn: func(w T) {
				w.WhenWriteMutableLine(false, Mutable("hello"))
			},
			expected: "",
		},
		{
			name: "WhenWriteMutableLine true empty",
			buildFn: func(w T) {
				w.WhenWriteMutableLine(true, Mutable(""))
			},
			expected: "\n",
		},
		{
			name: "WhenWriteMutableLines true",
			buildFn: func(w T) {
				w.WhenWriteMutableLines(true, Mutable("a"), Mutable("b"))
			},
			expected: "a\nb\n",
		},
		{
			name: "WhenWriteMutableLines false",
			buildFn: func(w T) {
				w.WhenWriteMutableLines(false, Mutable("a"), Mutable("b"))
			},
			expected: "",
		},
		{
			name: "WhenWriteMutableLines true none",
			buildFn: func(w T) {
				w.WhenWriteMutableLines(true)
			},
			expected: "",
		},
		{
			name: "WhenWriteMutableLines false none",
			buildFn: func(w T) {
				w.WhenWriteMutableLines(false)
			},
			expected: "",
		},
		{
			name: "WhenWriteMutableLines true with empty element",
			buildFn: func(w T) {
				w.WhenWriteMutableLines(true, Mutable("x"), Mutable(""), Mutable("y"))
			},
			expected: "x\n\ny\n",
		},
	}
}
