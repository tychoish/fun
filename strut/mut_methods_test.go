package strut

import (
	"slices"
	"testing"
)

// mutCase is a table-driven test case for *Mutable methods.
type mutCase struct {
	name     string
	fn       func(*Mutable)
	expected string
}

func runMutCases(t *testing.T, cases []mutCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			var m Mutable
			tc.fn(&m)
			if got := m.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// ---- PushString / PushBytes ----

func TestMutable_PushString(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushString("hello") }, "hello"},
		{"empty", func(m *Mutable) { m.PushString("") }, ""},
		{"unicode", func(m *Mutable) { m.PushString("世界") }, "世界"},
		{"emoji", func(m *Mutable) { m.PushString("🎉") }, "🎉"},
		{"multiple calls", func(m *Mutable) { m.PushString("foo"); m.PushString("bar") }, "foobar"},
		{"null bytes", func(m *Mutable) { m.PushString("\x00\x00") }, "\x00\x00"},
	})
}

func TestMutable_PushBytes(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushBytes([]byte("hello")) }, "hello"},
		{"empty slice", func(m *Mutable) { m.PushBytes([]byte{}) }, ""},
		{"nil slice", func(m *Mutable) { m.PushBytes(nil) }, ""},
		{"unicode", func(m *Mutable) { m.PushBytes([]byte("世界")) }, "世界"},
		{"binary data", func(m *Mutable) { m.PushBytes([]byte{0xFF, 0xFE, 0x00}) }, "\xff\xfe\x00"},
		{"multiple calls", func(m *Mutable) { m.PushBytes([]byte("foo")); m.PushBytes([]byte("bar")) }, "foobar"},
	})
}

// ---- Line / Tab ----

func TestMutable_Line(t *testing.T) {
	runMutCases(t, []mutCase{
		{"single", func(m *Mutable) { m.Line() }, "\n"},
		{"multiple", func(m *Mutable) { m.Line(); m.Line(); m.Line() }, "\n\n\n"},
		{"after content", func(m *Mutable) { m.PushString("hi"); m.Line() }, "hi\n"},
	})
}

func TestMutable_Tab(t *testing.T) {
	runMutCases(t, []mutCase{
		{"single", func(m *Mutable) { m.Tab() }, "\t"},
		{"multiple", func(m *Mutable) { m.Tab(); m.Tab() }, "\t\t"},
		{"after content", func(m *Mutable) { m.PushString("hi"); m.Tab() }, "hi\t"},
	})
}

func TestMutable_NLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"zero", func(m *Mutable) { m.NLines(0) }, ""},
		{"one", func(m *Mutable) { m.NLines(1) }, "\n"},
		{"three", func(m *Mutable) { m.NLines(3) }, "\n\n\n"},
		{"negative", func(m *Mutable) { m.NLines(-5) }, ""},
		{"after content", func(m *Mutable) { m.PushString("x"); m.NLines(2) }, "x\n\n"},
	})
}

func TestMutable_NTabs(t *testing.T) {
	runMutCases(t, []mutCase{
		{"zero", func(m *Mutable) { m.NTabs(0) }, ""},
		{"one", func(m *Mutable) { m.NTabs(1) }, "\t"},
		{"four", func(m *Mutable) { m.NTabs(4) }, "\t\t\t\t"},
		{"negative", func(m *Mutable) { m.NTabs(-1) }, ""},
		{"after content", func(m *Mutable) { m.PushString("x"); m.NTabs(2) }, "x\t\t"},
	})
}

// ---- Write helpers ----

func TestMutable_WriteMutable(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WriteMutable(Mutable("hello")) }, "hello"},
		{"empty", func(m *Mutable) { m.WriteMutable(Mutable("")) }, ""},
		{"unicode", func(m *Mutable) { m.WriteMutable(Mutable("世界🎉")) }, "世界🎉"},
		{"multiple", func(m *Mutable) { m.WriteMutable(Mutable("a")); m.WriteMutable(Mutable("b")) }, "ab"},
		{"nil mutable", func(m *Mutable) { m.WriteMutable(nil) }, ""},
		{"binary", func(m *Mutable) { m.WriteMutable(Mutable{0xFF, 0x00}) }, "\xff\x00"},
	})
}

func TestMutable_WriteLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WriteLine("hello") }, "hello\n"},
		{"empty string", func(m *Mutable) { m.WriteLine("") }, "\n"},
		{"unicode", func(m *Mutable) { m.WriteLine("世界") }, "世界\n"},
		{"multiple", func(m *Mutable) { m.WriteLine("a"); m.WriteLine("b") }, "a\nb\n"},
		{"already has newline", func(m *Mutable) { m.WriteLine("x\n") }, "x\n\n"},
	})
}

func TestMutable_WriteLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) { m.WriteLines("a", "b", "c") }, "a\nb\nc\n"},
		{"single", func(m *Mutable) { m.WriteLines("only") }, "only\n"},
		{"none", func(m *Mutable) { m.WriteLines() }, ""},
		{"empty strings", func(m *Mutable) { m.WriteLines("", "", "") }, "\n\n\n"},
		{"mixed empty", func(m *Mutable) { m.WriteLines("a", "", "b") }, "a\n\nb\n"},
		{"unicode", func(m *Mutable) { m.WriteLines("世界", "🎉") }, "世界\n🎉\n"},
	})
}

func TestMutable_WriteBytesLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WriteBytesLine([]byte("hello")) }, "hello\n"},
		{"empty", func(m *Mutable) { m.WriteBytesLine([]byte{}) }, "\n"},
		{"nil", func(m *Mutable) { m.WriteBytesLine(nil) }, "\n"},
		{"unicode", func(m *Mutable) { m.WriteBytesLine([]byte("世界")) }, "世界\n"},
		{"binary", func(m *Mutable) { m.WriteBytesLine([]byte{0x41, 0x42}) }, "AB\n"},
		{"multiple", func(m *Mutable) { m.WriteBytesLine([]byte("x")); m.WriteBytesLine([]byte("y")) }, "x\ny\n"},
	})
}

func TestMutable_WriteBytesLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) { m.WriteBytesLines([]byte("a"), []byte("b"), []byte("c")) }, "a\nb\nc\n"},
		{"single", func(m *Mutable) { m.WriteBytesLines([]byte("only")) }, "only\n"},
		{"none", func(m *Mutable) { m.WriteBytesLines() }, ""},
		{"empty slices", func(m *Mutable) { m.WriteBytesLines([]byte{}, []byte{}) }, "\n\n"},
		{"mixed empty", func(m *Mutable) { m.WriteBytesLines([]byte("a"), nil, []byte("b")) }, "a\n\nb\n"},
	})
}

func TestMutable_WriteMutableLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WriteMutableLine(Mutable("hello")) }, "hello\n"},
		{"empty", func(m *Mutable) { m.WriteMutableLine(Mutable("")) }, "\n"},
		{"unicode", func(m *Mutable) { m.WriteMutableLine(Mutable("世界")) }, "世界\n"},
		{"nil mutable", func(m *Mutable) { m.WriteMutableLine(nil) }, "\n"},
		{"multiple", func(m *Mutable) { m.WriteMutableLine(Mutable("a")); m.WriteMutableLine(Mutable("b")) }, "a\nb\n"},
	})
}

func TestMutable_WriteMutableLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) { m.WriteMutableLines(Mutable("a"), Mutable("b"), Mutable("c")) }, "a\nb\nc\n"},
		{"single", func(m *Mutable) { m.WriteMutableLines(Mutable("only")) }, "only\n"},
		{"none", func(m *Mutable) { m.WriteMutableLines() }, ""},
		{"empty mutables", func(m *Mutable) { m.WriteMutableLines(Mutable(""), Mutable("")) }, "\n\n"},
		{"mixed empty", func(m *Mutable) { m.WriteMutableLines(Mutable("a"), nil, Mutable("b")) }, "a\n\nb\n"},
		{"unicode", func(m *Mutable) { m.WriteMutableLines(Mutable("世界"), Mutable("🎉")) }, "世界\n🎉\n"},
	})
}

// ---- Concat / JoinStrings ----

func TestMutable_Concat(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) { m.Concat("hello", " ", "world") }, "hello world"},
		{"empty strings", func(m *Mutable) { m.Concat("", "", "") }, ""},
		{"single", func(m *Mutable) { m.Concat("alone") }, "alone"},
		{"none", func(m *Mutable) { m.Concat() }, ""},
		{"unicode", func(m *Mutable) { m.Concat("Hello", "世界", "🌍") }, "Hello世界🌍"},
		{"appends to existing", func(m *Mutable) { m.PushString("pre"); m.Concat("a", "b") }, "preab"},
	})
}

func TestMutable_JoinStrings(t *testing.T) {
	runMutCases(t, []mutCase{
		{"comma sep", func(m *Mutable) { m.JoinStrings([]string{"a", "b", "c"}, ",") }, "a,b,c"},
		{"space sep", func(m *Mutable) { m.JoinStrings([]string{"hello", "world"}, " ") }, "hello world"},
		{"empty sep", func(m *Mutable) { m.JoinStrings([]string{"a", "b", "c"}, "") }, "abc"},
		{"single element", func(m *Mutable) { m.JoinStrings([]string{"alone"}, ",") }, "alone"},
		{"empty slice", func(m *Mutable) { m.JoinStrings([]string{}, ",") }, ""},
		{"empty strings", func(m *Mutable) { m.JoinStrings([]string{"", "", ""}, ",") }, ",,"},
		{"unicode sep", func(m *Mutable) { m.JoinStrings([]string{"a", "b", "c"}, "→") }, "a→b→c"},
		{"unicode values", func(m *Mutable) { m.JoinStrings([]string{"世界", "🎉"}, "|") }, "世界|🎉"},
		{"appends to existing", func(m *Mutable) { m.PushString("pre:"); m.JoinStrings([]string{"a", "b"}, ",") }, "pre:a,b"},
	})
}

// ---- Extend additions ----

func TestMutable_ExtendLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) { m.ExtendStringsLines(slices.Values([]string{"a", "b", "c"})) }, "a\nb\nc\n"},
		{"single", func(m *Mutable) { m.ExtendStringsLines(slices.Values([]string{"only"})) }, "only\n"},
		{"empty seq", func(m *Mutable) { m.ExtendStringsLines(slices.Values([]string{})) }, ""},
		{"empty strings", func(m *Mutable) { m.ExtendStringsLines(slices.Values([]string{"", ""})) }, "\n\n"},
		{"unicode", func(m *Mutable) { m.ExtendStringsLines(slices.Values([]string{"世界", "🎉"})) }, "世界\n🎉\n"},
	})
}

func TestMutable_ExtendBytesLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) {
			m.ExtendBytesLines(slices.Values([][]byte{[]byte("a"), []byte("b"), []byte("c")}))
		}, "a\nb\nc\n"},
		{"single", func(m *Mutable) {
			m.ExtendBytesLines(slices.Values([][]byte{[]byte("only")}))
		}, "only\n"},
		{"empty seq", func(m *Mutable) {
			m.ExtendBytesLines(slices.Values([][]byte{}))
		}, ""},
		{"empty slices", func(m *Mutable) {
			m.ExtendBytesLines(slices.Values([][]byte{{}, {}}))
		}, "\n\n"},
		{"unicode", func(m *Mutable) {
			m.ExtendBytesLines(slices.Values([][]byte{[]byte("世界"), []byte("🎉")}))
		}, "世界\n🎉\n"},
	})
}

func TestMutable_ExtendMutableLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"multiple", func(m *Mutable) {
			m.ExtendLines(slices.Values([]Mutable{Mutable("a"), Mutable("b"), Mutable("c")}))
		}, "a\nb\nc\n"},
		{"single", func(m *Mutable) {
			m.ExtendLines(slices.Values([]Mutable{Mutable("only")}))
		}, "only\n"},
		{"empty seq", func(m *Mutable) {
			m.ExtendLines(slices.Values([]Mutable{}))
		}, ""},
		{"empty mutables", func(m *Mutable) {
			m.ExtendLines(slices.Values([]Mutable{nil, nil}))
		}, "\n\n"},
		{"unicode", func(m *Mutable) {
			m.ExtendLines(slices.Values([]Mutable{Mutable("世界"), Mutable("🎉")}))
		}, "世界\n🎉\n"},
	})
}

// ---- Repeat* ----

func TestMutable_RepeatByte(t *testing.T) {
	runMutCases(t, []mutCase{
		{"zero", func(m *Mutable) { m.RepeatByte('x', 0) }, ""},
		{"negative", func(m *Mutable) { m.RepeatByte('x', -1) }, ""},
		{"one", func(m *Mutable) { m.RepeatByte('x', 1) }, "x"},
		{"three", func(m *Mutable) { m.RepeatByte('-', 3) }, "---"},
		{"null byte", func(m *Mutable) { m.RepeatByte(0, 2) }, "\x00\x00"},
		{"appends to existing", func(m *Mutable) { m.PushString("ab"); m.RepeatByte('!', 2) }, "ab!!"},
	})
}

func TestMutable_RepeatRune(t *testing.T) {
	runMutCases(t, []mutCase{
		{"zero", func(m *Mutable) { m.RepeatRune('x', 0) }, ""},
		{"negative", func(m *Mutable) { m.RepeatRune('x', -1) }, ""},
		{"one ascii", func(m *Mutable) { m.RepeatRune('a', 1) }, "a"},
		{"three ascii", func(m *Mutable) { m.RepeatRune('!', 3) }, "!!!"},
		{"unicode", func(m *Mutable) { m.RepeatRune('世', 2) }, "世世"},
		{"emoji", func(m *Mutable) { m.RepeatRune('🎉', 3) }, "🎉🎉🎉"},
		{"appends to existing", func(m *Mutable) { m.PushString("hi"); m.RepeatRune('.', 2) }, "hi.."},
	})
}

func TestMutable_RepeatLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"zero", func(m *Mutable) { m.RepeatLine("hi", 0) }, ""},
		{"negative", func(m *Mutable) { m.RepeatLine("hi", -1) }, ""},
		{"one", func(m *Mutable) { m.RepeatLine("hi", 1) }, "hi\n"},
		{"three", func(m *Mutable) { m.RepeatLine("hi", 3) }, JOIN.WithConcat().Strings("hi\n", "hi\n", "hi\n")},
		{"empty string", func(m *Mutable) { m.RepeatLine("", 2) }, "\n\n"},
		{"unicode", func(m *Mutable) { m.RepeatLine("世界", 2) }, "世界\n世界\n"},
		{"appends to existing", func(m *Mutable) { m.PushString("pre\n"); m.RepeatLine("x", 2) }, "pre\nx\nx\n"},
	})
}

// ---- Mprint* ----

func TestMutable_Mprint(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.Mprint("hello world") }, "hello world"},
		{"multiple args", func(m *Mutable) { m.Mprint("a", "b", "c") }, "abc"},
		{"numbers", func(m *Mutable) { m.Mprint(42, 3.14) }, "42 3.14"},
		{"no args", func(m *Mutable) { m.Mprint() }, ""},
		{"returns self", func(m *Mutable) { m.Mprint("x").Mprint("y") }, "xy"},
	})
}

func TestMutable_Mprintf(t *testing.T) {
	runMutCases(t, []mutCase{
		{"format string", func(m *Mutable) { m.Mprintf("Hello %s, number %d", "world", 42) }, "Hello world, number 42"},
		{"empty format", func(m *Mutable) { m.Mprintf("") }, ""},
		{"no args", func(m *Mutable) { m.Mprintf("plain") }, "plain"},
		{"float format", func(m *Mutable) { m.Mprintf("%.2f", 3.14159) }, "3.14"},
		{"returns self", func(m *Mutable) { m.Mprintf("%s", "x").Mprintf("%s", "y") }, "xy"},
	})
}

func TestMutable_Mprintln(t *testing.T) {
	runMutCases(t, []mutCase{
		{"single arg", func(m *Mutable) { m.Mprintln("test") }, "test\n"},
		{"multiple args", func(m *Mutable) { m.Mprintln("a", "b", "c") }, "a b c\n"},
		{"no args", func(m *Mutable) { m.Mprintln() }, "\n"},
		{"numbers", func(m *Mutable) { m.Mprintln(1, 2) }, "1 2\n"},
		{"returns self", func(m *Mutable) { m.Mprintln("x").Mprintln("y") }, "x\ny\n"},
	})
}

// ---- Int / AppendBool / AppendInt64 / AppendUint64 / AppendFloat ----

func TestMutable_Int(t *testing.T) {
	runMutCases(t, []mutCase{
		{"positive", func(m *Mutable) { m.PushInt(42) }, "42"},
		{"zero", func(m *Mutable) { m.PushInt(0) }, "0"},
		{"negative", func(m *Mutable) { m.PushInt(-99) }, "-99"},
		{"max int32", func(m *Mutable) { m.PushInt(2147483647) }, "2147483647"},
		{"appends", func(m *Mutable) { m.PushString("n="); m.PushInt(7) }, "n=7"},
	})
}

func TestMutable_PushBool(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.PushBool(true) }, "true"},
		{"false", func(m *Mutable) { m.PushBool(false) }, "false"},
		{"appends to existing", func(m *Mutable) { m.PushString("val="); m.PushBool(true) }, "val=true"},
		{"chained", func(m *Mutable) { m.PushBool(true); _ = m.WriteByte(' '); m.PushBool(false) }, "true false"},
	})
}

func TestMutable_PushInt64(t *testing.T) {
	runMutCases(t, []mutCase{
		{"base 10 positive", func(m *Mutable) { m.PushInt64(42, 10) }, "42"},
		{"base 10 negative", func(m *Mutable) { m.PushInt64(-99, 10) }, "-99"},
		{"base 10 zero", func(m *Mutable) { m.PushInt64(0, 10) }, "0"},
		{"base 16", func(m *Mutable) { m.PushInt64(255, 16) }, "ff"},
		{"base 2", func(m *Mutable) { m.PushInt64(7, 2) }, "111"},
		{"base 8", func(m *Mutable) { m.PushInt64(8, 8) }, "10"},
		{"appends", func(m *Mutable) { m.PushString("0x"); m.PushInt64(255, 16) }, "0xff"},
	})
}

func TestMutable_PushUint64(t *testing.T) {
	runMutCases(t, []mutCase{
		{"base 10", func(m *Mutable) { m.PushUint64(42, 10) }, "42"},
		{"zero", func(m *Mutable) { m.PushUint64(0, 10) }, "0"},
		{"base 16", func(m *Mutable) { m.PushUint64(255, 16) }, "ff"},
		{"large value", func(m *Mutable) { m.PushUint64(18446744073709551615, 10) }, "18446744073709551615"},
		{"base 2", func(m *Mutable) { m.PushUint64(5, 2) }, "101"},
	})
}

func TestMutable_PushFloat(t *testing.T) {
	runMutCases(t, []mutCase{
		{"fixed 2 decimals", func(m *Mutable) { m.PushFloat(3.14159, 'f', 2, 64) }, "3.14"},
		{"negative", func(m *Mutable) { m.PushFloat(-2.5, 'f', 1, 64) }, "-2.5"},
		{"scientific", func(m *Mutable) { m.PushFloat(1000000.0, 'e', 2, 64) }, "1.00e+06"},
		{"zero", func(m *Mutable) { m.PushFloat(0.0, 'f', 1, 64) }, "0.0"},
		{"appends", func(m *Mutable) { m.PushString("v="); m.PushFloat(1.5, 'f', 1, 64) }, "v=1.5"},
	})
}

// ---- PushComplex ----

func TestMutable_PushComplex(t *testing.T) {
	runMutCases(t, []mutCase{
		{"real and imag", func(m *Mutable) { m.PushComplex(complex(3, 4), 'f', 1, 128) }, "(3.0+4.0i)"},
		{"zero", func(m *Mutable) { m.PushComplex(complex(0, 0), 'f', 0, 128) }, "(0+0i)"},
		{"negative imag", func(m *Mutable) { m.PushComplex(complex(1, -2), 'f', 1, 128) }, "(1.0-2.0i)"},
	})
}

// ---- PushQuote* ----

func TestMutable_PushQuote(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushQuote("hello") }, `"hello"`},
		{"with newline", func(m *Mutable) { m.PushQuote("a\nb") }, `"a\nb"`},
		{"unicode", func(m *Mutable) { m.PushQuote("世界") }, `"世界"`},
		{"empty", func(m *Mutable) { m.PushQuote("") }, `""`},
		{"appends to existing", func(m *Mutable) { m.PushString("k="); m.PushQuote("v") }, `k="v"`},
	})
}

func TestMutable_PushQuoteASCII(t *testing.T) {
	runMutCases(t, []mutCase{
		{"ascii", func(m *Mutable) { m.PushQuoteASCII("hello") }, `"hello"`},
		{"unicode escaped", func(m *Mutable) { m.PushQuoteASCII("世界") }, `"\u4e16\u754c"`},
		{"empty", func(m *Mutable) { m.PushQuoteASCII("") }, `""`},
		{"appends to existing", func(m *Mutable) { m.PushString("k="); m.PushQuoteASCII("v") }, `k="v"`},
	})
}

func TestMutable_PushQuoteGrapic(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushQuoteGrapic("hello") }, `"hello"`},
		{"control char", func(m *Mutable) { m.PushQuoteGrapic("hi\x00") }, `"hi\x00"`},
		{"empty", func(m *Mutable) { m.PushQuoteGrapic("") }, `""`},
	})
}

func TestMutable_PushQuoteRune(t *testing.T) {
	runMutCases(t, []mutCase{
		{"ascii", func(m *Mutable) { m.PushQuoteRune('a') }, "'a'"},
		{"unicode", func(m *Mutable) { m.PushQuoteRune('世') }, "'世'"},
		{"newline", func(m *Mutable) { m.PushQuoteRune('\n') }, `'\n'`},
		{"appends to existing", func(m *Mutable) { m.PushString("c="); m.PushQuoteRune('x') }, "c='x'"},
	})
}

func TestMutable_PushQuoteRuneASCII(t *testing.T) {
	runMutCases(t, []mutCase{
		{"ascii", func(m *Mutable) { m.PushQuoteRuneASCII('a') }, "'a'"},
		{"unicode escaped", func(m *Mutable) { m.PushQuoteRuneASCII('世') }, `'\u4e16'`},
		{"newline", func(m *Mutable) { m.PushQuoteRuneASCII('\n') }, `'\n'`},
	})
}

func TestMutable_PushQuoteRuneGrapic(t *testing.T) {
	runMutCases(t, []mutCase{
		{"printable", func(m *Mutable) { m.PushQuoteRuneGrapic('a') }, "'a'"},
		{"control char", func(m *Mutable) { m.PushQuoteRuneGrapic('\x00') }, `'\x00'`},
		{"emoji", func(m *Mutable) { m.PushQuoteRuneGrapic('🎉') }, "'🎉'"},
	})
}

// ---- With* ----

func TestMutable_WithTrimSpace(t *testing.T) {
	runMutCases(t, []mutCase{
		{"leading and trailing", func(m *Mutable) { m.WithTrimSpace("  hello  ") }, "hello"},
		{"newlines and tabs", func(m *Mutable) { m.WithTrimSpace("\n\thello\t\n") }, "hello"},
		{"all whitespace", func(m *Mutable) { m.WithTrimSpace("   ") }, ""},
		{"empty", func(m *Mutable) { m.WithTrimSpace("") }, ""},
		{"no whitespace", func(m *Mutable) { m.WithTrimSpace("hello") }, "hello"},
		{"internal space preserved", func(m *Mutable) { m.WithTrimSpace("  hello world  ") }, "hello world"},
	})
}

func TestMutable_WithTrimRight(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WithTrimRight("hello!!!", "!") }, "hello"},
		{"multiple chars", func(m *Mutable) { m.WithTrimRight("hello123", "321") }, "hello"},
		{"no match", func(m *Mutable) { m.WithTrimRight("hello", "x") }, "hello"},
		{"empty string", func(m *Mutable) { m.WithTrimRight("", "!") }, ""},
		{"trim all", func(m *Mutable) { m.WithTrimRight("!!!!", "!") }, ""},
	})
}

func TestMutable_WithTrimLeft(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WithTrimLeft("!!!hello", "!") }, "hello"},
		{"multiple chars", func(m *Mutable) { m.WithTrimLeft("123hello", "321") }, "hello"},
		{"no match", func(m *Mutable) { m.WithTrimLeft("hello", "x") }, "hello"},
		{"empty string", func(m *Mutable) { m.WithTrimLeft("", "!") }, ""},
		{"trim all", func(m *Mutable) { m.WithTrimLeft("!!!!", "!") }, ""},
	})
}

func TestMutable_WithTrimPrefix(t *testing.T) {
	runMutCases(t, []mutCase{
		{"exists", func(m *Mutable) { m.WithTrimPrefix("prefix_content", "prefix_") }, "content"},
		{"not found", func(m *Mutable) { m.WithTrimPrefix("content", "prefix_") }, "content"},
		{"empty prefix", func(m *Mutable) { m.WithTrimPrefix("hello", "") }, "hello"},
		{"empty string", func(m *Mutable) { m.WithTrimPrefix("", "prefix") }, ""},
		{"prefix equals string", func(m *Mutable) { m.WithTrimPrefix("hello", "hello") }, ""},
		{"prefix longer than string", func(m *Mutable) { m.WithTrimPrefix("hi", "hello") }, "hi"},
	})
}

func TestMutable_WithTrimSuffix(t *testing.T) {
	runMutCases(t, []mutCase{
		{"exists", func(m *Mutable) { m.WithTrimSuffix("content_suffix", "_suffix") }, "content"},
		{"not found", func(m *Mutable) { m.WithTrimSuffix("content", "_suffix") }, "content"},
		{"empty suffix", func(m *Mutable) { m.WithTrimSuffix("hello", "") }, "hello"},
		{"empty string", func(m *Mutable) { m.WithTrimSuffix("", "suffix") }, ""},
		{"suffix equals string", func(m *Mutable) { m.WithTrimSuffix("hello", "hello") }, ""},
		{"suffix longer than string", func(m *Mutable) { m.WithTrimSuffix("hi", "hello") }, "hi"},
	})
}

func TestMutable_WithReplaceAll(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.WithReplaceAll("hello hello", "hello", "hi") }, "hi hi"}, //nolint:dupword
		{"no match", func(m *Mutable) { m.WithReplaceAll("hello", "world", "hi") }, "hello"},
		{"replace with empty", func(m *Mutable) { m.WithReplaceAll("abc", "b", "") }, "ac"},
		{"empty string", func(m *Mutable) { m.WithReplaceAll("", "x", "y") }, ""},
		{"replace all", func(m *Mutable) { m.WithReplaceAll("aaa", "a", "b") }, "bbb"},
		{"old equals new", func(m *Mutable) { m.WithReplaceAll("hello", "hello", "hello") }, "hello"},
	})
}

func TestMutable_WithReplace(t *testing.T) {
	runMutCases(t, []mutCase{
		{"limited n=2", func(m *Mutable) { m.WithReplace("a a a a", "a", "b", 2) }, "b b a a"}, //nolint:dupword
		{"zero replacements", func(m *Mutable) { m.WithReplace("hello", "l", "x", 0) }, "hello"},
		{"negative (all)", func(m *Mutable) { m.WithReplace("a a a", "a", "b", -1) }, "b b b"}, //nolint:dupword
		{"no match", func(m *Mutable) { m.WithReplace("hello", "z", "x", 1) }, "hello"},
		{"n greater than count", func(m *Mutable) { m.WithReplace("aa", "a", "b", 10) }, "bb"},
	})
}

// ---- PushTrim* ----

func TestMutable_PushTrimSpace(t *testing.T) {
	runMutCases(t, []mutCase{
		{"leading and trailing", func(m *Mutable) { m.PushTrimSpace([]byte("  hello  ")) }, "hello"},
		{"newlines", func(m *Mutable) { m.PushTrimSpace([]byte("\nhello\n")) }, "hello"},
		{"all whitespace", func(m *Mutable) { m.PushTrimSpace([]byte("   ")) }, ""},
		{"empty", func(m *Mutable) { m.PushTrimSpace([]byte{}) }, ""},
		{"nil", func(m *Mutable) { m.PushTrimSpace(nil) }, ""},
		{"no whitespace", func(m *Mutable) { m.PushTrimSpace([]byte("hello")) }, "hello"},
		{"appends to existing", func(m *Mutable) { m.PushString("x"); m.PushTrimSpace([]byte(" y ")) }, "xy"},
	})
}

func TestMutable_PushTrimRight(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushTrimRight([]byte("hello!!!"), "!") }, "hello"},
		{"no match", func(m *Mutable) { m.PushTrimRight([]byte("hello"), "x") }, "hello"},
		{"empty slice", func(m *Mutable) { m.PushTrimRight([]byte{}, "!") }, ""},
		{"nil", func(m *Mutable) { m.PushTrimRight(nil, "!") }, ""},
		{"trim all", func(m *Mutable) { m.PushTrimRight([]byte("!!!"), "!") }, ""},
	})
}

func TestMutable_PushTrimLeft(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushTrimLeft([]byte("!!!hello"), "!") }, "hello"},
		{"no match", func(m *Mutable) { m.PushTrimLeft([]byte("hello"), "x") }, "hello"},
		{"empty slice", func(m *Mutable) { m.PushTrimLeft([]byte{}, "!") }, ""},
		{"nil", func(m *Mutable) { m.PushTrimLeft(nil, "!") }, ""},
		{"trim all", func(m *Mutable) { m.PushTrimLeft([]byte("!!!"), "!") }, ""},
	})
}

func TestMutable_PushTrimPrefix(t *testing.T) {
	runMutCases(t, []mutCase{
		{"exists", func(m *Mutable) { m.PushTrimPrefix([]byte("prefix_content"), []byte("prefix_")) }, "content"},
		{"not found", func(m *Mutable) { m.PushTrimPrefix([]byte("content"), []byte("prefix_")) }, "content"},
		{"empty prefix", func(m *Mutable) { m.PushTrimPrefix([]byte("hello"), []byte{}) }, "hello"},
		{"nil prefix", func(m *Mutable) { m.PushTrimPrefix([]byte("hello"), nil) }, "hello"},
		{"nil slice", func(m *Mutable) { m.PushTrimPrefix(nil, []byte("p")) }, ""},
		{"prefix equals string", func(m *Mutable) { m.PushTrimPrefix([]byte("hello"), []byte("hello")) }, ""},
	})
}

func TestMutable_PushTrimSuffix(t *testing.T) {
	runMutCases(t, []mutCase{
		{"exists", func(m *Mutable) { m.PushTrimSuffix([]byte("content_suffix"), []byte("_suffix")) }, "content"},
		{"not found", func(m *Mutable) { m.PushTrimSuffix([]byte("content"), []byte("_suffix")) }, "content"},
		{"empty suffix", func(m *Mutable) { m.PushTrimSuffix([]byte("hello"), []byte{}) }, "hello"},
		{"nil suffix", func(m *Mutable) { m.PushTrimSuffix([]byte("hello"), nil) }, "hello"},
		{"nil slice", func(m *Mutable) { m.PushTrimSuffix(nil, []byte("s")) }, ""},
		{"suffix equals string", func(m *Mutable) { m.PushTrimSuffix([]byte("hello"), []byte("hello")) }, ""},
	})
}

func TestMutable_PushReplaceAll(t *testing.T) {
	runMutCases(t, []mutCase{
		{"simple", func(m *Mutable) { m.PushReplaceAll([]byte("hello hello"), []byte("hello"), []byte("hi")) }, "hi hi"}, //nolint:dupword
		{"no match", func(m *Mutable) { m.PushReplaceAll([]byte("hello"), []byte("world"), []byte("hi")) }, "hello"},
		{"replace with empty", func(m *Mutable) { m.PushReplaceAll([]byte("abc"), []byte("b"), []byte{}) }, "ac"},
		{"empty input", func(m *Mutable) { m.PushReplaceAll([]byte{}, []byte("x"), []byte("y")) }, ""},
		{"nil input", func(m *Mutable) { m.PushReplaceAll(nil, []byte("x"), []byte("y")) }, ""},
		{"appends to existing", func(m *Mutable) { m.PushString("z:"); m.PushReplaceAll([]byte("ab"), []byte("b"), []byte("B")) }, "z:aB"},
	})
}

func TestMutable_PushReplace(t *testing.T) {
	runMutCases(t, []mutCase{
		{"n=2", func(m *Mutable) { m.PushReplace([]byte("a a a a"), []byte("a"), []byte("b"), 2) }, "b b a a"}, //nolint:dupword
		{"n=0", func(m *Mutable) { m.PushReplace([]byte("hello"), []byte("l"), []byte("x"), 0) }, "hello"},
		{"n=-1 (all)", func(m *Mutable) { m.PushReplace([]byte("a a a"), []byte("a"), []byte("b"), -1) }, "b b b"}, //nolint:dupword
		{"no match", func(m *Mutable) { m.PushReplace([]byte("hello"), []byte("z"), []byte("x"), 1) }, "hello"},
		{"n=1", func(m *Mutable) { m.PushReplace([]byte("hello"), []byte("l"), []byte("L"), 1) }, "heLlo"},
		{"empty input", func(m *Mutable) { m.PushReplace([]byte{}, []byte("x"), []byte("y"), 1) }, ""},
	})
}

// ---- When* ----

func TestMutable_WhenMprint(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenMprint(true, "hello") }, "hello"},
		{"false", func(m *Mutable) { m.WhenMprint(false, "hello") }, ""},
		{"true multiple args", func(m *Mutable) { m.WhenMprint(true, "a", "b") }, "ab"},
		{"true no args", func(m *Mutable) { m.WhenMprint(true) }, ""},
		{"returns self for chaining", func(m *Mutable) { m.WhenMprint(true, "x").WhenMprint(false, "y") }, "x"},
	})
}

func TestMutable_WhenMprintf(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenMprintf(true, "n=%d", 42) }, "n=42"},
		{"false", func(m *Mutable) { m.WhenMprintf(false, "n=%d", 42) }, ""},
		{"true no format args", func(m *Mutable) { m.WhenMprintf(true, "plain") }, "plain"},
		{"returns self for chaining", func(m *Mutable) { m.WhenMprintf(true, "x").WhenMprintf(true, "y") }, "xy"},
	})
}

func TestMutable_WhenMprintln(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenMprintln(true, "test") }, "test\n"},
		{"false", func(m *Mutable) { m.WhenMprintln(false, "test") }, ""},
		{"true no args", func(m *Mutable) { m.WhenMprintln(true) }, "\n"},
		{"returns self for chaining", func(m *Mutable) { m.WhenMprintln(true, "a").WhenMprintln(true, "b") }, "a\nb\n"},
	})
}

func TestMutable_WhenLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenLine(true) }, "\n"},
		{"false", func(m *Mutable) { m.WhenLine(false) }, ""},
		{"true after content", func(m *Mutable) { m.PushString("x"); m.WhenLine(true) }, "x\n"},
		{"false after content", func(m *Mutable) { m.PushString("x"); m.WhenLine(false) }, "x"},
	})
}

func TestMutable_WhenTab(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenTab(true) }, "\t"},
		{"false", func(m *Mutable) { m.WhenTab(false) }, ""},
		{"true after content", func(m *Mutable) { m.PushString("x"); m.WhenTab(true) }, "x\t"},
		{"false after content", func(m *Mutable) { m.PushString("x"); m.WhenTab(false) }, "x"},
	})
}

func TestMutable_WhenNLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true n=2", func(m *Mutable) { m.WhenNLines(true, 2) }, "\n\n"},
		{"false n=2", func(m *Mutable) { m.WhenNLines(false, 2) }, ""},
		{"true n=0", func(m *Mutable) { m.WhenNLines(true, 0) }, ""},
		{"true n=-1", func(m *Mutable) { m.WhenNLines(true, -1) }, ""},
	})
}

func TestMutable_WhenNTabs(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true n=3", func(m *Mutable) { m.WhenNTabs(true, 3) }, "\t\t\t"},
		{"false n=3", func(m *Mutable) { m.WhenNTabs(false, 3) }, ""},
		{"true n=0", func(m *Mutable) { m.WhenNTabs(true, 0) }, ""},
		{"true n=-1", func(m *Mutable) { m.WhenNTabs(true, -1) }, ""},
	})
}

func TestMutable_WhenWrite(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWrite(true, []byte("hello")) }, "hello"},
		{"false", func(m *Mutable) { m.WhenWrite(false, []byte("hello")) }, ""},
		{"true empty", func(m *Mutable) { m.WhenWrite(true, []byte{}) }, ""},
		{"true nil", func(m *Mutable) { m.WhenWrite(true, nil) }, ""},
		{"true binary", func(m *Mutable) { m.WhenWrite(true, []byte{0xFF, 0x00}) }, "\xff\x00"},
	})
}

func TestMutable_WhenWriteString(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteString(true, "hello") }, "hello"},
		{"false", func(m *Mutable) { m.WhenWriteString(false, "hello") }, ""},
		{"true empty", func(m *Mutable) { m.WhenWriteString(true, "") }, ""},
		{"true unicode", func(m *Mutable) { m.WhenWriteString(true, "世界") }, "世界"},
	})
}

func TestMutable_WhenWriteByte(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteByte(true, 'x') }, "x"},
		{"false", func(m *Mutable) { m.WhenWriteByte(false, 'x') }, ""},
		{"true newline", func(m *Mutable) { m.WhenWriteByte(true, '\n') }, "\n"},
		{"true null", func(m *Mutable) { m.WhenWriteByte(true, 0x00) }, "\x00"},
	})
}

func TestMutable_WhenWriteRune(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true ascii", func(m *Mutable) { m.WhenWriteRune(true, 'a') }, "a"},
		{"false", func(m *Mutable) { m.WhenWriteRune(false, 'a') }, ""},
		{"true unicode", func(m *Mutable) { m.WhenWriteRune(true, '世') }, "世"},
		{"true emoji", func(m *Mutable) { m.WhenWriteRune(true, '🎉') }, "🎉"},
	})
}

func TestMutable_WhenWriteLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteLine(true, "hello") }, "hello\n"},
		{"false", func(m *Mutable) { m.WhenWriteLine(false, "hello") }, ""},
		{"true empty", func(m *Mutable) { m.WhenWriteLine(true, "") }, "\n"},
		{"true unicode", func(m *Mutable) { m.WhenWriteLine(true, "世界") }, "世界\n"},
	})
}

func TestMutable_WhenWriteLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteLines(true, "a", "b") }, "a\nb\n"},
		{"false", func(m *Mutable) { m.WhenWriteLines(false, "a", "b") }, ""},
		{"true none", func(m *Mutable) { m.WhenWriteLines(true) }, ""},
		{"true single", func(m *Mutable) { m.WhenWriteLines(true, "only") }, "only\n"},
		{"true empty strings", func(m *Mutable) { m.WhenWriteLines(true, "", "") }, "\n\n"},
	})
}

func TestMutable_WhenConcat(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenConcat(true, "x", "y", "z") }, "xyz"},
		{"false", func(m *Mutable) { m.WhenConcat(false, "x", "y", "z") }, ""},
		{"true none", func(m *Mutable) { m.WhenConcat(true) }, ""},
		{"true single", func(m *Mutable) { m.WhenConcat(true, "only") }, "only"},
		{"true empty strings", func(m *Mutable) { m.WhenConcat(true, "", "") }, ""},
	})
}

func TestMutable_WhenWriteMutable(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteMutable(true, Mutable("hello")) }, "hello"},
		{"false", func(m *Mutable) { m.WhenWriteMutable(false, Mutable("hello")) }, ""},
		{"true empty", func(m *Mutable) { m.WhenWriteMutable(true, Mutable("")) }, ""},
		{"true nil", func(m *Mutable) { m.WhenWriteMutable(true, nil) }, ""},
		{"true unicode", func(m *Mutable) { m.WhenWriteMutable(true, Mutable("世界")) }, "世界"},
	})
}

func TestMutable_WhenWriteMutableLine(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteMutableLine(true, Mutable("hello")) }, "hello\n"},
		{"false", func(m *Mutable) { m.WhenWriteMutableLine(false, Mutable("hello")) }, ""},
		{"true empty", func(m *Mutable) { m.WhenWriteMutableLine(true, Mutable("")) }, "\n"},
		{"true nil", func(m *Mutable) { m.WhenWriteMutableLine(true, nil) }, "\n"},
	})
}

func TestMutable_WhenWriteMutableLines(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenWriteMutableLines(true, Mutable("a"), Mutable("b")) }, "a\nb\n"},
		{"false", func(m *Mutable) { m.WhenWriteMutableLines(false, Mutable("a"), Mutable("b")) }, ""},
		{"true none", func(m *Mutable) { m.WhenWriteMutableLines(true) }, ""},
		{"true with empty element", func(m *Mutable) { m.WhenWriteMutableLines(true, Mutable("x"), nil, Mutable("y")) }, "x\n\ny\n"},
		{"false none", func(m *Mutable) { m.WhenWriteMutableLines(false) }, ""},
	})
}

func TestMutable_WhenJoin(t *testing.T) {
	runMutCases(t, []mutCase{
		{"true", func(m *Mutable) { m.WhenJoin(true, []string{"a", "b", "c"}, ",") }, "a,b,c"},
		{"false", func(m *Mutable) { m.WhenJoin(false, []string{"a", "b", "c"}, ",") }, ""},
		{"true single", func(m *Mutable) { m.WhenJoin(true, []string{"only"}, ",") }, "only"},
		{"true empty slice", func(m *Mutable) { m.WhenJoin(true, []string{}, ",") }, ""},
		{"true empty sep", func(m *Mutable) { m.WhenJoin(true, []string{"a", "b"}, "") }, "ab"},
		{"true unicode sep", func(m *Mutable) { m.WhenJoin(true, []string{"a", "b"}, "→") }, "a→b"},
	})
}

// ---- Composition / integration tests ----

func TestMutable_MethodsReturnSelfForChaining(t *testing.T) {
	// Mprint* return *Mutable for chaining; verify the returned value IS the receiver.
	var m Mutable
	result := m.Mprint("a").Mprintf("%s", "b").Mprintln("c")
	if result != &m {
		t.Error("chained Mprint* did not return the receiver")
	}
	if got := m.String(); got != "abc\n" {
		t.Errorf("got %q, want %q", got, "abc\n")
	}
}

func TestMutable_MixedHighLevelOps(t *testing.T) {
	var m Mutable
	m.WriteLine("header")
	m.NTabs(1)
	m.Concat("key", "=")
	m.PushQuote("value")
	m.Line()
	m.Mprintf("count=%d", 3)
	m.Line()

	want := "header\n\tkey=\"value\"\ncount=3\n"
	if got := m.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMutable_ConditionalBranching(t *testing.T) {
	for _, cond := range []bool{true, false} {
		t.Run("cond="+func() string {
			if cond {
				return "true"
			}
			return "false"
		}(), func(t *testing.T) {
			var m Mutable
			m.WhenWriteString(cond, "yes")
			m.WhenWriteString(!cond, "no")
			if got := m.String(); got != "yes" && got != "no" {
				t.Errorf("unexpected result %q", got)
			}
			if cond && m.String() != "yes" {
				t.Errorf("got %q, want %q", m.String(), "yes")
			}
			if !cond && m.String() != "no" {
				t.Errorf("got %q, want %q", m.String(), "no")
			}
		})
	}
}

func TestMutable_ExtendLinesAppendsBytesLinesConsistency(t *testing.T) {
	// ExtendLines and ExtendBytesLines on the same data should produce equal output.
	data := []string{"alpha", "beta", "gamma"}
	var a, b Mutable
	a.ExtendStringsLines(slices.Values(data))
	byteData := make([][]byte, len(data))
	for i, s := range data {
		byteData[i] = []byte(s)
	}
	b.ExtendBytesLines(slices.Values(byteData))
	if a.String() != b.String() {
		t.Errorf("ExtendLines=%q, ExtendBytesLines=%q", a, b)
	}
}

func TestMutable_PushTrimReturnSubsliceNotCopy(t *testing.T) {
	// bytes.TrimSpace returns a subslice; PushBytes appends it into mut.
	// Verify content is correct and the original slice is unmodified.
	orig := []byte("  hello  ")
	var m Mutable
	m.PushTrimSpace(orig)
	if got := m.String(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	// original must be unmodified
	if string(orig) != "  hello  " {
		t.Errorf("original slice was modified: %q", orig)
	}
}

func TestMutable_PushBytePushString(t *testing.T) {
	// PushByte is WriteByte; verify it's equivalent to PushString for single-byte ASCII.
	var a, b Mutable
	_ = a.WriteByte('x')
	b.PushString("x")
	if a.String() != b.String() {
		t.Errorf("WriteByte('x')=%q != PushString(\"x\")=%q", a, b)
	}
}
