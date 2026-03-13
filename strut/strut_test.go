package strut

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuilder_BasicWrites(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "WriteString simple",
			buildFn: func(b *Builder) {
				b.WriteString("hello")
			},
			expected: "hello",
		},
		{
			name: "WriteString unicode",
			buildFn: func(b *Builder) {
				b.WriteString("こんにちは世界")
			},
			expected: "こんにちは世界",
		},
		{
			name: "WriteString empty",
			buildFn: func(b *Builder) {
				b.WriteString("")
			},
			expected: "",
		},
		{
			name: "WriteString emoji",
			buildFn: func(b *Builder) {
				b.WriteString("Hello 🌍🎉🚀")
			},
			expected: "Hello 🌍🎉🚀",
		},
		{
			name: "WriteByte",
			buildFn: func(b *Builder) {
				b.WriteByte('a')
				b.WriteByte('b')
				b.WriteByte('c')
			},
			expected: "abc",
		},
		{
			name: "WriteByte special characters",
			buildFn: func(b *Builder) {
				b.WriteByte('\n')
				b.WriteByte('\t')
				b.WriteByte(' ')
			},
			expected: "\n\t ",
		},
		{
			name: "WriteRune ASCII",
			buildFn: func(b *Builder) {
				b.WriteRune('x')
				b.WriteRune('y')
			},
			expected: "xy",
		},
		{
			name: "WriteRune unicode",
			buildFn: func(b *Builder) {
				b.WriteRune('世')
				b.WriteRune('界')
				b.WriteRune('🎉')
			},
			expected: "世界🎉",
		},
	}

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

func TestBuilder_Line(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "single line",
			buildFn: func(b *Builder) {
				b.Line()
			},
			expected: "\n",
		},
		{
			name: "single tab",
			buildFn: func(b *Builder) {
				b.Tab()
			},
			expected: "\t",
		},
		{
			name: "NLines positive",
			buildFn: func(b *Builder) {
				b.NLines(3)
			},
			expected: "\n\n\n",
		},
		{
			name: "NTabs positive",
			buildFn: func(b *Builder) {
				b.NTabs(3)
			},
			expected: "\t\t\t",
		},
		{
			name: "NLines zero",
			buildFn: func(b *Builder) {
				b.NLines(0)
			},
			expected: "",
		},
		{
			name: "NTabs zero",
			buildFn: func(b *Builder) {
				b.NTabs(0)
			},
			expected: "",
		},
		{
			name: "NLines negative",
			buildFn: func(b *Builder) {
				b.NLines(-1)
			},
			expected: "",
		},
		{
			name: "NTabs negative",
			buildFn: func(b *Builder) {
				b.NTabs(-1)
			},
			expected: "",
		},
		{
			name: "WriteLine",
			buildFn: func(b *Builder) {
				b.WriteLine("test")
			},
			expected: "test\n",
		},
		{
			name: "WriteLine empty",
			buildFn: func(b *Builder) {
				b.WriteLine("")
			},
			expected: "\n",
		},
		{
			name: "WriteLines",
			buildFn: func(b *Builder) {
				b.WriteLines("first", "second", "third")
			},
			expected: "first\nsecond\nthird\n",
		},
		{
			name: "WriteLines empty",
			buildFn: func(b *Builder) {
				b.WriteLines()
			},
			expected: "",
		},
	}

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

func TestBuilder_Concat(t *testing.T) {
	tests := []struct {
		name     string
		strs     []string
		expected string
	}{
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
	tests := []struct {
		name     string
		buf      []byte
		expected string
	}{
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
	tests := []struct {
		name     string
		strs     []string
		liminal  string
		expected string
	}{
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
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "Repeat positive",
			buildFn: func(b *Builder) {
				b.Repeat("ab", 3)
			},
			expected: "ababab",
		},
		{
			name: "Repeat zero",
			buildFn: func(b *Builder) {
				b.Repeat("test", 0)
			},
			expected: "",
		},
		{
			name: "Repeat negative",
			buildFn: func(b *Builder) {
				b.Repeat("test", -1)
			},
			expected: "",
		},
		{
			name: "Repeat empty string",
			buildFn: func(b *Builder) {
				b.Repeat("", 5)
			},
			expected: "",
		},
		{
			name: "RepeatByte positive",
			buildFn: func(b *Builder) {
				b.RepeatByte('x', 4)
			},
			expected: "xxxx",
		},
		{
			name: "RepeatByte zero",
			buildFn: func(b *Builder) {
				b.RepeatByte('x', 0)
			},
			expected: "",
		},
		{
			name: "RepeatRune unicode",
			buildFn: func(b *Builder) {
				b.RepeatRune('🎉', 3)
			},
			expected: "🎉🎉🎉",
		},
		{
			name: "RepeatRune negative",
			buildFn: func(b *Builder) {
				b.RepeatRune('a', -5)
			},
			expected: "",
		},
		{
			name: "RepeatLine",
			buildFn: func(b *Builder) {
				b.RepeatLine("test", 2)
			},
			expected: "test\ntest\n",
		},
		{
			name: "RepeatLine zero",
			buildFn: func(b *Builder) {
				b.RepeatLine("test", 0)
			},
			expected: "",
		},
	}

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

func TestBuilder_Wprint(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "Wprint simple",
			buildFn: func(b *Builder) {
				b.Wprint("hello", " ", "world")
			},
			expected: "hello world",
		},
		{
			name: "Wprint numbers",
			buildFn: func(b *Builder) {
				b.Wprint(42, " ", 3.14)
			},
			expected: "42 3.14",
		},
		{
			name: "Wprintf format",
			buildFn: func(b *Builder) {
				b.Wprintf("Hello %s, number %d", "world", 42)
			},
			expected: "Hello world, number 42",
		},
		{
			name: "Wprintf empty",
			buildFn: func(b *Builder) {
				b.Wprintf("")
			},
			expected: "",
		},
		{
			name: "Wprintln",
			buildFn: func(b *Builder) {
				b.Wprintln("test")
			},
			expected: "test\n",
		},
		{
			name: "Wprintln multiple args",
			buildFn: func(b *Builder) {
				b.Wprintln("a", "b", "c")
			},
			expected: "a b c\n",
		},
	}

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

func TestBuilder_WhenMethods(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "WhenWprint true",
			buildFn: func(b *Builder) {
				b.WhenWprint(true, "hello")
			},
			expected: "hello",
		},
		{
			name: "WhenWprint false",
			buildFn: func(b *Builder) {
				b.WhenWprint(false, "hello")
			},
			expected: "",
		},
		{
			name: "WhenWprintf true",
			buildFn: func(b *Builder) {
				b.WhenWprintf(true, "num=%d", 42)
			},
			expected: "num=42",
		},
		{
			name: "WhenWprintf false",
			buildFn: func(b *Builder) {
				b.WhenWprintf(false, "num=%d", 42)
			},
			expected: "",
		},
		{
			name: "WhenWprintln true",
			buildFn: func(b *Builder) {
				b.WhenWprintln(true, "test")
			},
			expected: "test\n",
		},
		{
			name: "WhenWprintln false",
			buildFn: func(b *Builder) {
				b.WhenWprintln(false, "test")
			},
			expected: "",
		},
		{
			name: "WhenLine true",
			buildFn: func(b *Builder) {
				b.WhenLine(true)
			},
			expected: "\n",
		},
		{
			name: "WhenLine false",
			buildFn: func(b *Builder) {
				b.WhenLine(false)
			},
			expected: "",
		},
		{
			name: "WhenNLines true",
			buildFn: func(b *Builder) {
				b.WhenNLines(true, 2)
			},
			expected: "\n\n",
		},
		{
			name: "WhenNLines false",
			buildFn: func(b *Builder) {
				b.WhenNLines(false, 2)
			},
			expected: "",
		},

		{
			name: "WhenTab true",
			buildFn: func(b *Builder) {
				b.WhenTab(true)
			},
			expected: "\t",
		},
		{
			name: "WhenTab false",
			buildFn: func(b *Builder) {
				b.WhenTab(false)
			},
			expected: "",
		},
		{
			name: "WhenNTabs true",
			buildFn: func(b *Builder) {
				b.WhenNTabs(true, 2)
			},
			expected: "\t\t",
		},
		{
			name: "WhenNTabs false",
			buildFn: func(b *Builder) {
				b.WhenNTabs(false, 2)
			},
			expected: "",
		},

		{
			name: "WhenWrite true",
			buildFn: func(b *Builder) {
				b.WhenWrite(true, []byte("test"))
			},
			expected: "test",
		},
		{
			name: "WhenWrite false",
			buildFn: func(b *Builder) {
				b.WhenWrite(false, []byte("test"))
			},
			expected: "",
		},
		{
			name: "WhenWriteString true",
			buildFn: func(b *Builder) {
				b.WhenWriteString(true, "hello")
			},
			expected: "hello",
		},
		{
			name: "WhenWriteString false",
			buildFn: func(b *Builder) {
				b.WhenWriteString(false, "hello")
			},
			expected: "",
		},
		{
			name: "WhenWriteByte true",
			buildFn: func(b *Builder) {
				b.WhenWriteByte(true, 'x')
			},
			expected: "x",
		},
		{
			name: "WhenWriteByte false",
			buildFn: func(b *Builder) {
				b.WhenWriteByte(false, 'x')
			},
			expected: "",
		},
		{
			name: "WhenWriteRune true",
			buildFn: func(b *Builder) {
				b.WhenWriteRune(true, '🎉')
			},
			expected: "🎉",
		},
		{
			name: "WhenWriteRune false",
			buildFn: func(b *Builder) {
				b.WhenWriteRune(false, '🎉')
			},
			expected: "",
		},
		{
			name: "WhenWriteLine true",
			buildFn: func(b *Builder) {
				b.WhenWriteLine(true, "test")
			},
			expected: "test\n",
		},
		{
			name: "WhenWriteLine false",
			buildFn: func(b *Builder) {
				b.WhenWriteLine(false, "test")
			},
			expected: "",
		},
		{
			name: "WhenWriteLines true",
			buildFn: func(b *Builder) {
				b.WhenWriteLines(true, "a", "b")
			},
			expected: "a\nb\n",
		},
		{
			name: "WhenWriteLines false",
			buildFn: func(b *Builder) {
				b.WhenWriteLines(false, "a", "b")
			},
			expected: "",
		},
		{
			name: "WhenConcat true",
			buildFn: func(b *Builder) {
				b.WhenConcat(true, "x", "y", "z")
			},
			expected: "xyz",
		},
		{
			name: "WhenConcat false",
			buildFn: func(b *Builder) {
				b.WhenConcat(false, "x", "y", "z")
			},
			expected: "",
		},
		{
			name: "WhenJoin true",
			buildFn: func(b *Builder) {
				b.WhenJoin(true, []string{"a", "b"}, ",")
			},
			expected: "a,b",
		},
		{
			name: "WhenJoin false",
			buildFn: func(b *Builder) {
				b.WhenJoin(false, []string{"a", "b"}, ",")
			},
			expected: "",
		},
	}

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

func TestBuilder_Quote(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "Quote simple",
			buildFn: func(b *Builder) {
				b.Quote("hello")
			},
			expected: `"hello"`,
		},
		{
			name: "Quote with special chars",
			buildFn: func(b *Builder) {
				b.Quote("hello\nworld")
			},
			expected: `"hello\nworld"`,
		},
		{
			name: "Quote unicode",
			buildFn: func(b *Builder) {
				b.Quote("世界")
			},
			expected: `"世界"`,
		},
		{
			name: "QuoteASCII unicode",
			buildFn: func(b *Builder) {
				b.QuoteASCII("世界")
			},
			expected: `"\u4e16\u754c"`,
		},
		{
			name: "QuoteGrapic",
			buildFn: func(b *Builder) {
				b.QuoteGrapic("hello\x00world")
			},
			expected: `"hello\x00world"`,
		},
		{
			name: "QuoteRune",
			buildFn: func(b *Builder) {
				b.QuoteRune('a')
			},
			expected: "'a'",
		},
		{
			name: "QuoteRune unicode",
			buildFn: func(b *Builder) {
				b.QuoteRune('世')
			},
			expected: "'世'",
		},
		{
			name: "QuoteRuneASCII",
			buildFn: func(b *Builder) {
				b.QuoteRuneASCII('世')
			},
			expected: `'\u4e16'`,
		},
		{
			name: "QuoteRuneGrapic",
			buildFn: func(b *Builder) {
				b.QuoteRuneGrapic('\n')
			},
			expected: `'\n'`,
		},
	}

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

func TestBuilder_FormatNumbers(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "Int positive",
			buildFn: func(b *Builder) {
				b.Int(42)
			},
			expected: "42",
		},
		{
			name: "Int negative",
			buildFn: func(b *Builder) {
				b.Int(-99)
			},
			expected: "-99",
		},
		{
			name: "Int zero",
			buildFn: func(b *Builder) {
				b.Int(0)
			},
			expected: "0",
		},
		{
			name: "FormatBool true",
			buildFn: func(b *Builder) {
				b.FormatBool(true)
			},
			expected: "true",
		},
		{
			name: "FormatBool false",
			buildFn: func(b *Builder) {
				b.FormatBool(false)
			},
			expected: "false",
		},
		{
			name: "FormatInt64 base 10",
			buildFn: func(b *Builder) {
				b.FormatInt64(123, 10)
			},
			expected: "123",
		},
		{
			name: "FormatInt64 base 16",
			buildFn: func(b *Builder) {
				b.FormatInt64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "FormatInt64 base 2",
			buildFn: func(b *Builder) {
				b.FormatInt64(7, 2)
			},
			expected: "111",
		},
		{
			name: "FormatInt64 negative",
			buildFn: func(b *Builder) {
				b.FormatInt64(-42, 10)
			},
			expected: "-42",
		},
		{
			name: "FormatUint64",
			buildFn: func(b *Builder) {
				b.FormatUint64(255, 16)
			},
			expected: "ff",
		},
		{
			name: "FormatFloat",
			buildFn: func(b *Builder) {
				b.FormatFloat(3.14159, 'f', 2, 64)
			},
			expected: "3.14",
		},
		{
			name: "FormatFloat scientific",
			buildFn: func(b *Builder) {
				b.FormatFloat(1000000.0, 'e', 2, 64)
			},
			expected: "1.00e+06",
		},
		{
			name: "FormatFloat negative",
			buildFn: func(b *Builder) {
				b.FormatFloat(-2.5, 'f', 1, 64)
			},
			expected: "-2.5",
		},
		{
			name: "FormatComplex",
			buildFn: func(b *Builder) {
				b.FormatComplex(complex(3, 4), 'f', 1, 128)
			},
			expected: "(3.0+4.0i)",
		},
	}

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

func TestBuilder_Trim(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "TrimSpace",
			buildFn: func(b *Builder) {
				b.WithTrimSpace("  hello  ")
			},
			expected: "hello",
		},
		{
			name: "TrimSpace newlines",
			buildFn: func(b *Builder) {
				b.WithTrimSpace("\n\thello\t\n")
			},
			expected: "hello",
		},
		{
			name: "TrimRight",
			buildFn: func(b *Builder) {
				b.WithTrimRight("hello!!!", "!")
			},
			expected: "hello",
		},
		{
			name: "TrimLeft",
			buildFn: func(b *Builder) {
				b.WithTrimLeft("!!!hello", "!")
			},
			expected: "hello",
		},
		{
			name: "TrimPrefix exists",
			buildFn: func(b *Builder) {
				b.WithTrimPrefix("prefix_content", "prefix_")
			},
			expected: "content",
		},
		{
			name: "TrimPrefix not exists",
			buildFn: func(b *Builder) {
				b.WithTrimPrefix("content", "prefix_")
			},
			expected: "content",
		},
		{
			name: "TrimSuffix exists",
			buildFn: func(b *Builder) {
				b.WithTrimSuffix("content_suffix", "_suffix")
			},
			expected: "content",
		},
		{
			name: "TrimSuffix not exists",
			buildFn: func(b *Builder) {
				b.WithTrimSuffix("content", "_suffix")
			},
			expected: "content",
		},
	}

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

func TestBuilder_Replace(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "ReplaceAll",
			buildFn: func(b *Builder) {
				b.WithReplaceAll("hello hello", "hello", "hi") //nolint:dupword
			},
			expected: "hi hi", //nolint:dupword
		},
		{
			name: "ReplaceAll no match",
			buildFn: func(b *Builder) {
				b.WithReplaceAll("hello", "world", "hi")
			},
			expected: "hello",
		},
		{
			name: "Replace limited",
			buildFn: func(b *Builder) {
				b.WithReplace("a a a a", "a", "b", 2) //nolint:dupword
			},
			expected: "b b a a", //nolint:dupword
		},
		{
			name: "Replace zero",
			buildFn: func(b *Builder) {
				b.WithReplace("hello", "l", "x", 0)
			},
			expected: "hello",
		},
		{
			name: "Replace negative (all)",
			buildFn: func(b *Builder) {
				b.WithReplace("a a a", "a", "b", -1) //nolint:dupword
			},
			expected: "b b b", //nolint:dupword
		},
	}

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

func TestBuilder_Extend(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		expected string
	}{
		{
			name: "Extend with values",
			buildFn: func(b *Builder) {
				b.Extend(slices.Values([]string{"a", "b", "c"}))
			},
			expected: "abc",
		},
		{
			name: "Extend empty",
			buildFn: func(b *Builder) {
				b.Extend(slices.Values([]string{}))
			},
			expected: "",
		},
		{
			name: "ExtendLines",
			buildFn: func(b *Builder) {
				b.ExtendLines(slices.Values([]string{"line1", "line2"}))
			},
			expected: "line1\nline2\n",
		},
		{
			name: "ExtendParagraph",
			buildFn: func(b *Builder) {
				b.ExtendParagraph(slices.Values([]string{"line1", "line2"}))
			},
			expected: "line1\nline2\n\n", // ExtendParagraph calls ExtendLines + Line
		},
		{
			name: "ExtendJoin",
			buildFn: func(b *Builder) {
				b.ExtendJoin(slices.Values([]string{"a", "b", "c"}), ",")
			},
			expected: "a,b,c",
		},
		{
			name: "ExtendJoin single element",
			buildFn: func(b *Builder) {
				b.ExtendJoin(slices.Values([]string{"alone"}), ",")
			},
			expected: "alone",
		},
		{
			name: "ExtendJoin empty",
			buildFn: func(b *Builder) {
				b.ExtendJoin(slices.Values([]string{}), ",")
			},
			expected: "",
		},
	}

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
	tests := []struct {
		name     string
		buildFn  func(*Builder)
		validate func(*testing.T, string)
	}{
		{
			name: "mixed operations",
			buildFn: func(b *Builder) {
				b.WriteString("Hello")
				b.WriteByte(' ')
				b.WriteRune('世')
				b.Line()
				b.Int(42)
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
			buildFn: func(b *Builder) {
				b.WhenWriteString(true, "yes1")
				b.WhenWriteString(false, "no")
				b.WhenWriteString(true, "yes2")
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
			buildFn: func(b *Builder) {
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
			buildFn: func(b *Builder) {
				b.Repeat("x", 1000)
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
			buildFn: func(b *Builder) {
				b.Append([]byte{0xFF, 0xFE})
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
			buildFn: func(b *Builder) {
				b.FormatComplex(complex(0, 0), 'f', 0, 128)
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
			buildFn: func(b *Builder) {
				b.Join([]string{"🌍", "🎉", "🚀"}, " → ")
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
			buildFn: func(b *Builder) {
				for i := range 3 {
					b.WhenWriteLine(i%2 == 0, "even")
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			tt.buildFn(&b)
			tt.validate(t, b.String())
		})
	}
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

	// WriteParagraph writes each string as a line, then adds an extra blank line
	expected := "start\nmiddle\nend1\nend2\n"
	got := b.String()

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
