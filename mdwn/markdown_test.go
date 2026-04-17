package mdwn

import (
	"fmt"
	"iter"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tychoish/fun/irt"
)

// build is a test helper that runs fn against a fresh Builder and returns the
// accumulated string.
func build(fn func(*Builder)) string {
	var mb Builder
	fn(&mb)
	return mb.String()
}

// --- Headings ---

func TestHeadings(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"H1", func(m *Builder) { m.H1("Title") }, "# Title\n\n"},
		{"H2", func(m *Builder) { m.H2("Section") }, "## Section\n\n"},
		{"H3", func(m *Builder) { m.H3("Sub") }, "### Sub\n\n"},
		{"H1 empty", func(m *Builder) { m.H1("") }, "# \n\n"},
		{"H1 variadic concat", func(m *Builder) { m.H1("Foo", "Bar") }, "# FooBar\n\n"},
		{"H1Words single", func(m *Builder) { m.H1Words("Title") }, "# Title\n\n"},
		{"H1Words multi", func(m *Builder) { m.H1Words("Release", "1.2.3") }, "# Release 1.2.3\n\n"},
		{"H1Words none", func(m *Builder) { m.H1Words() }, "# \n\n"},
		{"H2Words", func(m *Builder) { m.H2Words("Section", "One") }, "## Section One\n\n"},
		{"H3Words", func(m *Builder) { m.H3Words("Sub", "Section") }, "### Sub Section\n\n"},
		{"H4", func(m *Builder) { m.H4("Deep") }, "#### Deep\n\n"},
		{"H5", func(m *Builder) { m.H5("Deeper") }, "##### Deeper\n\n"},
		{"H6", func(m *Builder) { m.H6("Deepest") }, "###### Deepest\n\n"},
		{"H4Words", func(m *Builder) { m.H4Words("Deep", "Section") }, "#### Deep Section\n\n"},
		{"H5Words", func(m *Builder) { m.H5Words("Deeper", "Section") }, "##### Deeper Section\n\n"},
		{"H6Words", func(m *Builder) { m.H6Words("Deepest", "Section") }, "###### Deepest Section\n\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Paragraphs ---

func TestParagraphs(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"Paragraph", func(m *Builder) { m.Paragraph("Hello world.") }, "Hello world.\n\n"},
		{"Paragraph empty", func(m *Builder) { m.Paragraph("") }, "\n\n"},
		{"Paragraph variadic", func(m *Builder) { m.Paragraph("Hello", " world.") }, "Hello world.\n\n"},
		{"ParagraphWords", func(m *Builder) { m.ParagraphWords("hello", "world") }, "hello world\n\n"},
		{"ParagraphWords single", func(m *Builder) { m.ParagraphWords("hello") }, "hello\n\n"},
		{"ParagraphWords none", func(m *Builder) { m.ParagraphWords() }, "\n\n"},
		{"ItalicParagraph", func(m *Builder) { m.ItalicParagraph("note") }, "_note_\n\n"},
		{"ItalicParagraph empty", func(m *Builder) { m.ItalicParagraph("") }, "__\n\n"},
		{"ItalicParagraph variadic", func(m *Builder) { m.ItalicParagraph("a", "b") }, "_ab_\n\n"},
		{"ParagraphBreak", func(m *Builder) { m.PushString("a").ParagraphBreak().PushString("b") }, "a\n\nb"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- KV methods ---

func TestKVMethods(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"KV", func(m *Builder) { m.KV("Name", "Alice") }, "**Name**: Alice\n"},
		{"KV empty", func(m *Builder) { m.KV("", "") }, "****: \n"},
		{"KVany string", func(m *Builder) { m.KVany("Name", "Alice") }, "**Name**: Alice\n"},
		{"KVany int", func(m *Builder) { m.KVany("Count", 42) }, "**Count**: 42\n"},
		{"KVany bool", func(m *Builder) { m.KVany("Flag", true) }, "**Flag**: true\n"},
		{"FromKV", func(m *Builder) { m.FromKV(irt.MakeKV("K", "V")) }, "**K**: V\n"},
		{"FromKVany", func(m *Builder) { m.FromKVany(irt.MakeKV[string, any]("N", 7)) }, "**N**: 7\n"},
		{"KVs", func(m *Builder) {
			m.KVs(irt.MakeKV("Name", "Alice"), irt.MakeKV("Role", "Engineer"))
		}, "**Name**: Alice\n**Role**: Engineer\n"},
		{"KVanys", func(m *Builder) {
			m.KVanys(irt.MakeKV[string, any]("Count", 42), irt.MakeKV[string, any]("Flag", true))
		}, "**Count**: 42\n**Flag**: true\n"},
		{"ExtendKV", func(m *Builder) {
			m.ExtendKV(func(yield func(string, string) bool) {
				yield("A", "1")
				yield("B", "2")
			})
		}, "**A**: 1\n**B**: 2\n"},
		{"ExtendKVany", func(m *Builder) {
			m.ExtendKVany(func(yield func(string, any) bool) {
				yield("X", 99)
				yield("Y", false)
			})
		}, "**X**: 99\n**Y**: false\n"},
		{"ExtendKVSeq", func(m *Builder) {
			m.ExtendKVSeq(irt.Slice([]irt.KV[string, string]{
				irt.MakeKV("A", "1"),
				irt.MakeKV("B", "2"),
			}))
		}, "**A**: 1\n**B**: 2\n"},
		{"ExtendKVanySeq", func(m *Builder) {
			m.ExtendKVanySeq(irt.Slice([]irt.KV[string, any]{
				irt.MakeKV[string, any]("X", 99),
				irt.MakeKV[string, any]("Y", false),
			}))
		}, "**X**: 99\n**Y**: false\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Bullet lists ---

func TestBulletList(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"BulletListItem single", func(m *Builder) { m.BulletListItem("one") }, "- one\n"},
		{"BulletListItem empty", func(m *Builder) { m.BulletListItem("") }, "- \n"},
		{"BulletListItem variadic", func(m *Builder) { m.BulletListItem("foo", " ", "bar") }, "- foo bar\n"},
		{"BulletListItem multiple calls", func(m *Builder) {
			m.BulletListItem("one")
			m.BulletListItem("two")
		}, "- one\n- two\n"},
		{"BulletListItemWords none", func(m *Builder) { m.BulletListItemWords() }, "- \n"},
		{"BulletListItemWords single", func(m *Builder) { m.BulletListItemWords("alpha") }, "- alpha\n"},
		{"BulletListItemWords multi", func(m *Builder) { m.BulletListItemWords("a", "b", "c") }, "- a b c\n"},
		{"BulletList", func(m *Builder) { m.BulletList("alpha", "beta", "gamma") }, "- alpha\n- beta\n- gamma\n\n"},
		{"BulletList empty", func(m *Builder) { m.BulletList() }, ""},
		{"BulletList all empty strings", func(m *Builder) { m.BulletList("", "", "") }, ""},
		{"ExtendBulletList", func(m *Builder) {
			m.ExtendBulletList(func(yield func(string) bool) {
				for _, s := range []string{"x", "y", "z"} {
					if !yield(s) {
						return
					}
				}
			})
		}, "- x\n- y\n- z\n\n"},
		{"ExtendBulletList empty", func(m *Builder) {
			m.ExtendBulletList(func(yield func(string) bool) {})
		}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Ordered lists ---

func TestOrderedList(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"OrderedListItem single", func(m *Builder) { m.OrderedListItem("a") }, "1. a\n"},
		{"OrderedListItem empty", func(m *Builder) { m.OrderedListItem("") }, "1. \n"},
		{"OrderedListItem variadic", func(m *Builder) { m.OrderedListItem("step", " one") }, "1. step one\n"},
		{"OrderedListItem multiple calls", func(m *Builder) {
			m.OrderedListItem("a")
			m.OrderedListItem("b")
		}, "1. a\n1. b\n"},
		{"OrderedListItemWords none", func(m *Builder) { m.OrderedListItemWords() }, "1. \n"},
		{"OrderedListItemWords single", func(m *Builder) { m.OrderedListItemWords("first") }, "1. first\n"},
		{"OrderedListItemWords multi", func(m *Builder) { m.OrderedListItemWords("first", "item") }, "1. first item\n"},
		{"OrderedList", func(m *Builder) { m.OrderedList("first", "second", "third") }, "1. first\n1. second\n1. third\n\n"},
		{"OrderedList empty", func(m *Builder) { m.OrderedList() }, ""},
		{"ExtendOrderedList", func(m *Builder) {
			m.ExtendOrderedList(func(yield func(string) bool) {
				for _, s := range []string{"one", "two", "three"} {
					if !yield(s) {
						return
					}
				}
			})
		}, "1. one\n1. two\n1. three\n\n"},
		{"ExtendOrderedList empty", func(m *Builder) {
			m.ExtendOrderedList(func(yield func(string) bool) {})
		}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Block quotes ---

func TestBlockQuote(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"basic", func(m *Builder) { m.BlockQuote("line one\nline two") }, "> line one\n> line two\n\n"},
		{"trailing newline trimmed", func(m *Builder) { m.BlockQuote("text\n\n") }, "> text\n\n"},
		{"blank lines preserved", func(m *Builder) { m.BlockQuote("para one\n\npara two") }, "> para one\n> \n> para two\n\n"},
		{"empty string", func(m *Builder) { m.BlockQuote("") }, ""},
		{"only newlines", func(m *Builder) { m.BlockQuote("\n\n") }, ""},
		{"variadic parts", func(m *Builder) { m.BlockQuote("line one\n", "line two") }, "> line one\n> line two\n\n"},
		{"variadic all empty", func(m *Builder) { m.BlockQuote("", "") }, ""},
		{"BlockQuoteWith", func(m *Builder) {
			m.BlockQuoteWith(func(inner *Builder) { inner.BulletList("item a", "item b") })
		}, "> - item a\n> - item b\n\n"},
		{"BlockQuoteWith empty fn", func(m *Builder) { m.BlockQuoteWith(func(*Builder) {}) }, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Fenced code ---

func TestFencedCode(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"with lang", func(m *Builder) { m.FencedCode("go", "fmt.Println()") }, "```go\nfmt.Println()\n```\n\n"},
		{"no lang", func(m *Builder) { m.FencedCode("", "x := 1\n") }, "```\nx := 1\n```\n\n"},
		{"empty code", func(m *Builder) { m.FencedCode("", "") }, "```\n\n```\n\n"},
		{"FencedCodeWith lang", func(m *Builder) {
			m.FencedCodeWith("go", func(inner *Builder) { inner.PushString("fmt.Println()\n") })
		}, "```go\nfmt.Println()\n```\n\n"},
		{"FencedCodeWith no lang", func(m *Builder) {
			m.FencedCodeWith("", func(inner *Builder) { inner.PushString("x := 1\n") })
		}, "```\nx := 1\n```\n\n"},
		{"FencedCodeWith multi-line", func(m *Builder) {
			m.FencedCodeWith("sh", func(inner *Builder) { inner.PushString("echo hello\necho world\n") })
		}, "```sh\necho hello\necho world\n```\n\n"},
		{"FencedCodeWith empty fn", func(m *Builder) { m.FencedCodeWith("go", func(*Builder) {}) }, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Indentation ---

func TestIndent(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"IndentWith 4-space", func(m *Builder) {
			m.IndentWith("    ", func(inner *Builder) { inner.PushString("line one\nline two\n") })
		}, "    line one\n    line two\n\n"},
		{"IndentWith tab", func(m *Builder) {
			m.IndentWith("\t", func(inner *Builder) { inner.PushString("a\nb\n") })
		}, "\ta\n\tb\n\n"},
		{"IndentWith empty fn", func(m *Builder) { m.IndentWith("    ", func(*Builder) {}) }, ""},
		{"IndentedCode multi-line", func(m *Builder) { m.IndentedCode("x := 1\ny := 2") }, "    x := 1\n    y := 2\n\n"},
		{"IndentedCode empty", func(m *Builder) { m.IndentedCode("") }, ""},
		{"IndentedCodeWith", func(m *Builder) {
			m.IndentedCodeWith(func(inner *Builder) { inner.PushString("fmt.Println()\n") })
		}, "    fmt.Println()\n\n"},
		{"IndentedCodeWith empty fn", func(m *Builder) { m.IndentedCodeWith(func(*Builder) {}) }, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Inline formatters ---

func TestInlineFormatters(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		// Bold
		{"Bold single", func(m *Builder) { m.Bold("important") }, "**important**"},
		{"Bold variadic", func(m *Builder) { m.Bold("a", "b", "c") }, "**abc**"},
		{"Bold empty", func(m *Builder) { m.Bold("") }, "****"},
		{"BoldWords", func(m *Builder) { m.BoldWords("hello", "world") }, "**hello world**"},
		// Italic
		{"Italic single", func(m *Builder) { m.Italic("em") }, "_em_"},
		{"Italic variadic", func(m *Builder) { m.Italic("a", "b") }, "_ab_"},
		{"Italic empty", func(m *Builder) { m.Italic("") }, "__"},
		{"ItalicWords", func(m *Builder) { m.ItalicWords("foo", "bar") }, "_foo bar_"},
		// Preformatted
		{"Preformatted single", func(m *Builder) { m.Preformatted("code") }, "`code`"},
		{"Preformatted variadic", func(m *Builder) { m.Preformatted("x", "y") }, "`xy`"},
		{"Preformatted empty", func(m *Builder) { m.Preformatted("") }, "``"},
		{"PreformattedWords", func(m *Builder) { m.PreformattedWords("go", "build") }, "`go build`"},
		// Strikethrough
		{"Strikethrough single", func(m *Builder) { m.Strikethrough("old") }, "~~old~~"},
		{"Strikethrough variadic", func(m *Builder) { m.Strikethrough("a", "b") }, "~~ab~~"},
		{"Strikethrough empty", func(m *Builder) { m.Strikethrough("") }, "~~~~"},
		{"StrikethroughWords", func(m *Builder) { m.StrikethroughWords("old", "text") }, "~~old text~~"},
		// Link
		{"Link", func(m *Builder) { m.Link("click", "https://example.com") }, "[click](https://example.com)"},
		{"Link empty", func(m *Builder) { m.Link("", "") }, "[]()"},
		// Text
		{"Text single", func(m *Builder) { m.Text("hello") }, "hello"},
		{"Text variadic", func(m *Builder) { m.Text("hello", " ", "world") }, "hello world"},
		{"Text empty", func(m *Builder) { m.Text("") }, ""},
		{"TextWords", func(m *Builder) { m.TextWords("hello", "world") }, "hello world"},
		// Chaining
		{"chained", func(m *Builder) {
			m.Bold("Note").Text(": see ").Link("docs", "https://example.com").ParagraphBreak()
		}, "**Note**: see [docs](https://example.com)\n\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- Push<op> chainable wrappers ---

func TestPushOps(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(*Builder)
		want string
	}{
		{"PushString chain", func(m *Builder) { m.PushString("a").PushString("b") }, "ab"},
		{"PushBytes chain", func(m *Builder) { m.PushBytes([]byte("hi")).PushBytes([]byte("!")) }, "hi!"},
		{"PushLine", func(m *Builder) { m.PushString("x").PushLine().PushString("y") }, "x\ny"},
		{"PushNLines", func(m *Builder) { m.PushString("x").PushNLines(2).PushString("y") }, "x\n\ny"},
		{"PushConcat", func(m *Builder) { m.PushConcat("a", "b", "c") }, "abc"},
		{"PushKV", func(m *Builder) { m.PushKV("Name", "Alice") }, "**Name**: Alice\n"},
		{"PushKVany string", func(m *Builder) { m.PushKVany("Count", 42) }, "**Count**: 42\n"},
		{"PushFromKV", func(m *Builder) { m.PushFromKV(irt.MakeKV("K", "V")) }, "**K**: V\n"},
		{"PushFromKVany", func(m *Builder) { m.PushFromKVany(irt.MakeKV[string, any]("N", 7)) }, "**N**: 7\n"},
		{"PushKV chained", func(m *Builder) {
			m.PushKV("A", "1").PushKV("B", "2")
		}, "**A**: 1\n**B**: 2\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := build(c.fn); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// --- fmt.Formatter and WriteTo ---

func TestFormat(t *testing.T) {
	t.Run("%s", func(t *testing.T) {
		var mb Builder
		mb.H1("Hi")
		if got := fmt.Sprintf("%s", &mb); got != "# Hi\n\n" {
			t.Errorf("got %q, want %q", got, "# Hi\n\n")
		}
	})
	t.Run("%v", func(t *testing.T) {
		var mb Builder
		mb.Paragraph("hello")
		if got := fmt.Sprintf("%v", &mb); got != "hello\n\n" {
			t.Errorf("got %q, want %q", got, "hello\n\n")
		}
	})
	t.Run("unknown verb", func(t *testing.T) {
		var mb Builder
		mb.Text("x")
		if got := fmt.Sprintf("%d", &mb); got != "%!d(mdwn.Builder)" {
			t.Errorf("got %q, want %q", got, "%!d(mdwn.Builder)")
		}
	})
	t.Run("WriteTo", func(t *testing.T) {
		var mb Builder
		mb.H1("Test")
		var out strings.Builder
		n, err := mb.WriteTo(&out)
		if err != nil {
			t.Fatal(err)
		}
		got := out.String()
		if got != "# Test\n\n" {
			t.Errorf("WriteTo: got %q", got)
		}
		if int(n) != len(got) {
			t.Errorf("WriteTo: reported n=%d but len=%d", n, len(got))
		}
	})
}

// --- Builder lifecycle ---

func TestBuilderLifecycle(t *testing.T) {
	t.Run("MakeBuilder pre-allocates capacity", func(t *testing.T) {
		mb := MakeBuilder(64)
		if mb == nil {
			t.Fatal("MakeBuilder returned nil")
		}
		if mb.Cap() < 64 {
			t.Errorf("Cap() = %d, want >= 64", mb.Cap())
		}
		if mb.Len() != 0 {
			t.Errorf("Len() = %d, want 0", mb.Len())
		}
		mb.H1("hello")
		if mb.Len() == 0 {
			t.Error("builder produced no output after H1")
		}
	})
	t.Run("MakeBuilder zero capacity", func(t *testing.T) {
		mb := MakeBuilder(0)
		if mb == nil {
			t.Fatal("MakeBuilder(0) returned nil")
		}
		mb.Text("x")
		if got := mb.String(); got != "x" {
			t.Errorf("got %q, want %q", got, "x")
		}
	})
	t.Run("Release zeros length", func(t *testing.T) {
		mb := MakeBuilder(32)
		mb.Paragraph("content")
		if mb.Len() == 0 {
			t.Fatal("expected non-empty builder before Release")
		}
		mb.Release()
		if mb.Len() != 0 {
			t.Errorf("after Release: Len() = %d, want 0", mb.Len())
		}
	})
	t.Run("Truncate", func(t *testing.T) {
		for _, c := range []struct {
			target  int
			wantLen int
			wantStr string
		}{
			{5, 5, "hello"},
			{0, 0, ""},
			{11, 11, "hello world"}, // equal to length — no-op
			{99, 11, "hello world"}, // beyond length — clamped to len
			{-1, 0, ""},             // negative — clamped to 0
		} {
			var mb Builder
			mb.Text("hello world")
			mb.Truncate(c.target)
			if mb.Len() != c.wantLen {
				t.Errorf("Truncate(%d): Len()=%d, want %d", c.target, mb.Len(), c.wantLen)
			}
			if mb.String() != c.wantStr {
				t.Errorf("Truncate(%d): got %q, want %q", c.target, mb.String(), c.wantStr)
			}
		}
	})
	t.Run("Truncate then write", func(t *testing.T) {
		var mb Builder
		mb.Text("hello world")
		mb.Truncate(5)
		mb.Text("!")
		if got := mb.String(); got != "hello!" {
			t.Errorf("got %q, want %q", got, "hello!")
		}
	})
}

// --- Table ---

func TestTable(t *testing.T) {
	t.Run("basic structure", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(
				Column{Name: "Name"},
				Column{Name: "Count", RightAlign: true},
			).Row("Alice", "42").Row("Bob", "7").Build()
		})
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 4 {
			t.Fatalf("expected at least 4 lines, got %d:\n%s", len(lines), got)
		}
		if !strings.HasPrefix(lines[0], "| Name") {
			t.Errorf("header = %q", lines[0])
		}
		if !strings.Contains(lines[1], "---") {
			t.Errorf("separator = %q", lines[1])
		}
		if !strings.Contains(lines[1], ":") {
			t.Errorf("right-align separator missing colon: %q", lines[1])
		}
		if !strings.Contains(lines[2], "Alice") {
			t.Errorf("row 0 = %q", lines[2])
		}
		if !strings.Contains(lines[3], "Bob") {
			t.Errorf("row 1 = %q", lines[3])
		}
	})
	t.Run("pipe escaping", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "Val"}).Row("a|b").Build()
		})
		if !strings.Contains(got, `a\|b`) {
			t.Errorf("pipe not escaped in %q", got)
		}
	})
	t.Run("column alignment", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(
				Column{Name: "L"},
				Column{Name: "R", RightAlign: true},
			).Row("left", "1234").Build()
		})
		dataRow := strings.Split(got, "\n")[2]
		if !strings.Contains(dataRow, "| left") {
			t.Errorf("left cell not left-aligned in %q", dataRow)
		}
		if !strings.Contains(dataRow, "1234 |") {
			t.Errorf("right cell not right-aligned in %q", dataRow)
		}
	})
	t.Run("MinWidth", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X", MinWidth: 10}).Row("hi").Build()
		})
		parts := strings.Split(strings.Split(got, "\n")[0], "|")
		if len(parts) < 2 {
			t.Fatalf("unexpected header %q", got)
		}
		if cellWidth := len(parts[1]) - 2; cellWidth < 10 {
			t.Errorf("column width %d < MinWidth 10", cellWidth)
		}
	})
	t.Run("MaxWidth truncates with default marker", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "T", MaxWidth: 8}).
				Row("short").
				Row("this is a very long value").Build()
		})
		longRow := strings.Split(got, "\n")[3]
		if strings.Contains(longRow, "this is a very long value") {
			t.Errorf("long value not truncated: %q", longRow)
		}
		if !strings.Contains(longRow, "...") {
			t.Errorf("truncated cell missing ellipsis: %q", longRow)
		}
	})
	t.Run("MaxWidth narrow truncation (no marker fits)", func(t *testing.T) {
		// MaxWidth=3 equals len("..."), so the else branch slices without appending marker.
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X", MaxWidth: 3}).Row("hello").Build()
		})
		if strings.Contains(strings.Split(got, "\n")[2], "hello") {
			t.Errorf("expected cell truncated, got %q", got)
		}
	})
	t.Run("custom TruncMarker", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "T", MaxWidth: 10, TruncMarker: "…"}).
				Row("this is definitely longer than ten characters").Build()
		})
		if !strings.Contains(got, "…") {
			t.Errorf("expected custom marker in %q", got)
		}
	})
	t.Run("Rows helper", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "K"}, Column{Name: "V"}).
				Rows([][]string{{"a", "1"}, {"b", "2"}}).Build()
		})
		if !strings.Contains(got, "| a") || !strings.Contains(got, "| b") {
			t.Errorf("expected rows a,b in %q", got)
		}
	})
	t.Run("Extend", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "K"}, Column{Name: "V"}).
				Extend(iter.Seq[[]string](func(yield func([]string) bool) {
					for _, row := range [][]string{{"x", "10"}, {"y", "20"}} {
						if !yield(row) {
							return
						}
					}
				})).Build()
		})
		if !strings.Contains(got, "| x") || !strings.Contains(got, "| y") {
			t.Errorf("expected rows x,y in %q", got)
		}
	})
	t.Run("ExtendRow", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "K"}, Column{Name: "V"}).
				ExtendRow(iter.Seq[string](func(yield func(string) bool) {
					for _, s := range []string{"alpha", "99"} {
						if !yield(s) {
							return
						}
					}
				})).Build()
		})
		if !strings.Contains(got, "alpha") || !strings.Contains(got, "99") {
			t.Errorf("expected row in output, got %q", got)
		}
	})
	t.Run("NewTableWithColumns", func(t *testing.T) {
		cols := []Column{{Name: "X"}, {Name: "Y", RightAlign: true}}
		got := build(func(m *Builder) {
			m.NewTableWithColumns(cols).Row("foo", "42").Build()
		})
		if !strings.Contains(got, "foo") || !strings.Contains(got, "42") {
			t.Errorf("unexpected output: %q", got)
		}
		if !strings.Contains(got, "X") || !strings.Contains(got, "Y") {
			t.Errorf("missing headers in: %q", got)
		}
	})
	t.Run("empty rows produces no output", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "A"}, Column{Name: "B"}).Build()
		})
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})
	t.Run("Row no cells is ignored", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).Row().Build()
		})
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})
	t.Run("ExtendRow empty seq is ignored", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).
				ExtendRow(func(yield func(string) bool) {}).Build()
		})
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})
	t.Run("ends with newline", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).Row("v").Build()
		})
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("expected trailing newline, got %q", got)
		}
	})
}

func TestKVTable(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.KVTable(
				irt.MakeKV("Name", "Count"),
				func(yield func(string, string) bool) {
					for _, pair := range [][2]string{{"Alice", "5"}, {"Bob", "3"}} {
						if !yield(pair[0], pair[1]) {
							return
						}
					}
				},
			)
		})
		if !strings.Contains(got, "Alice") || !strings.Contains(got, "Bob") {
			t.Errorf("expected rows in output, got %q", got)
		}
		if !strings.Contains(got, "Name") || !strings.Contains(got, "Count") {
			t.Errorf("expected headers in output, got %q", got)
		}
	})
	t.Run("empty seq produces no output", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.KVTable(irt.MakeKV("K", "V"), func(yield func(string, string) bool) {})
		})
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})
}

// --- Unicode column widths ---

func TestTableUnicodeMusicalSymbols(t *testing.T) {
	// Regression: column widths were computed from byte length, causing
	// multi-byte Unicode characters (♯ = 3 UTF-8 bytes, 1 rune) to produce
	// under-padded cells. Width must be rune count (visual width).
	//
	// "F♯ Minor" = 8 runes, 10 bytes. Column width must be 8, not 10.
	got := build(func(m *Builder) {
		m.NewTable(Column{Name: "Key"}).
			Row("E Minor").  // 7 runes, 7 bytes
			Row("F♯ Minor"). // 8 runes, 10 bytes — ♯ is 3 UTF-8 bytes
			Build()
	})
	want := "| Key      |\n| -------- |\n| E Minor  |\n| F♯ Minor |\n"
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
}

func TestTableUnicodeSmartQuotes(t *testing.T) {
	// Curly apostrophe ' (U+2019) is 3 UTF-8 bytes but 1 rune.
	got := build(func(m *Builder) {
		m.NewTable(Column{Name: "Title"}).
			Row("Short").        // 5 runes, 5 bytes
			Row("Saint\u2019s"). // 7 runes, 9 bytes
			Build()
	})
	want := "| Title   |\n| ------- |\n| Short   |\n| Saint\u2019s |\n"
	if got != want {
		t.Errorf("got:  %q\nwant: %q", got, want)
	}
}

func TestTableUnicodeColumnConsistency(t *testing.T) {
	// Every cell in a column must have the same visual width after padding.
	got := build(func(m *Builder) {
		m.NewTable(Column{Name: "K"}, Column{Name: "V"}).
			Row("plain", "A♭ Major"). // ♭ = 3 UTF-8 bytes
			Row("x", "B Major").
			Build()
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}
	var colWidths []int
	for _, line := range lines[2:] {
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		colWidths = append(colWidths, utf8.RuneCountInString(parts[2]))
	}
	for i := 1; i < len(colWidths); i++ {
		if colWidths[i] != colWidths[0] {
			t.Errorf("column visual width inconsistent: row 0=%d row %d=%d\n%s",
				colWidths[0], i, colWidths[i], got)
		}
	}
}

// --- runeByteOffset ---

func TestRuneByteOffset(t *testing.T) {
	b := []byte("F♯ Minor") // ♯ = 3 UTF-8 bytes; total 10 bytes, 8 runes
	for _, c := range []struct{ n, want int }{
		{0, 0},
		{1, 1},   // after "F" (1 byte)
		{2, 4},   // after "F♯" (1+3 bytes)
		{8, 10},  // after all 8 runes = end of slice
		{99, 10}, // n > rune count → len(b)
	} {
		if got := runeByteOffset(b, c.n); got != c.want {
			t.Errorf("runeByteOffset(%q, %d) = %d, want %d", b, c.n, got, c.want)
		}
	}
}

func TestHorizontalRule(t *testing.T) {
	got := build(func(m *Builder) { m.HorizontalRule() })
	if got != "-----\n\n" {
		t.Errorf("HorizontalRule() = %q, want %q", got, "-----\n\n")
	}
}

func TestImage(t *testing.T) {
	for _, c := range []struct {
		name    string
		altText string
		url     string
		want    string
	}{
		{"basic", "logo", "https://example.com/logo.png", "![logo](https://example.com/logo.png)"},
		{"empty alt", "", "https://example.com/img.png", "![](https://example.com/img.png)"},
		{"empty url", "alt text", "", "![alt text]()"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := build(func(m *Builder) { m.Image(c.altText, c.url) })
			if got != c.want {
				t.Errorf("Image() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTaskListItem(t *testing.T) {
	for _, c := range []struct {
		name string
		done bool
		item []string
		want string
	}{
		{"done single", true, []string{"buy milk"}, "- [x] buy milk\n"},
		{"not done single", false, []string{"buy milk"}, "- [ ] buy milk\n"},
		{"done multi-part", true, []string{"foo", "bar"}, "- [x] foobar\n"},
		{"not done empty", false, []string{}, "- [ ] \n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := build(func(m *Builder) { m.TaskListItem(c.done, c.item...) })
			if got != c.want {
				t.Errorf("TaskListItem() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTaskListItemWords(t *testing.T) {
	for _, c := range []struct {
		name string
		done bool
		want string
	}{
		{"done", true, "- [x] buy milk\n"},
		{"not done", false, "- [ ] buy milk\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := build(func(m *Builder) { m.TaskListItemWords(c.done, "buy", "milk") })
			if got != c.want {
				t.Errorf("TaskListItemWords() = %q, want %q", got, c.want)
			}
		})
	}
}

// rowWidth computes the visual width (rune count) of a pipe-delimited table
// row, not counting the trailing newline.
func rowWidth(line string) int { return utf8.RuneCountInString(line) }

func TestTableBuildMaxWidth(t *testing.T) {
	// Helper: build the table with BuildMaxWidth and return the output string.
	buildMaxWidth := func(t *testing.T, maxWidth int, cols []Column, rows [][]string) (string, error) {
		t.Helper()
		var mb Builder
		tb := mb.NewTableWithColumns(cols)
		for _, row := range rows {
			tb.Row(row...)
		}
		result, err := tb.BuildMaxWidth(maxWidth)
		if err != nil {
			return "", err
		}
		return result.String(), nil
	}

	t.Run("basic truncation", func(t *testing.T) {
		// 2 cols: "A" (fixed, natural width=3) + "B" (elastic)
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 20 → budget = 20 - 7 - 3 = 10
		// elastic content "This is quite long" (18 runes) > 10 → truncated to "This is..." (10)
		// row visual: | ab  | This is.. | = 1+(3+3)+(10+3) = 20
		cols := []Column{
			{Name: "A"},
			{Name: "B", Elastic: true},
		}
		rows := [][]string{{"ab", "This is quite long content here"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		dataLine := lines[2]
		// The data row must be exactly maxWidth runes wide.
		if w := rowWidth(dataLine); w != 20 {
			t.Errorf("data row width = %d, want 20; line=%q", w, dataLine)
		}
		// Elastic column must contain the truncation marker.
		if !strings.Contains(dataLine, "...") {
			t.Errorf("expected truncation marker in %q", dataLine)
		}
		// Full original content must NOT appear.
		if strings.Contains(got, "This is quite long content here") {
			t.Errorf("expected content to be truncated in %q", got)
		}
	})

	t.Run("no truncation needed", func(t *testing.T) {
		// elastic column content fits within budget — no truncation.
		// cols: "Key" (3 runes, width=3) + "Desc" (elastic)
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 30 → budget = 30 - 7 - 3 = 20
		// content "short" (5 runes) ≤ 20 — no truncation
		cols := []Column{
			{Name: "Key"},
			{Name: "Desc", Elastic: true},
		}
		rows := [][]string{{"abc", "short"}}
		got, err := buildMaxWidth(t, 30, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(got, "...") {
			t.Errorf("unexpected truncation marker in %q", got)
		}
		if !strings.Contains(got, "short") {
			t.Errorf("expected full content 'short' in %q", got)
		}
	})

	t.Run("other columns exhaust budget", func(t *testing.T) {
		// budget < 3 → elastic clamped to max(3, MinWidth) = 3; no error.
		// cols: "LongColName" (11 runes, width=11) + "E" (elastic)
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 10 → budget = 10 - 7 - 11 = -8 < 3 → width = 3
		cols := []Column{
			{Name: "LongColName"},
			{Name: "E", Elastic: true},
		}
		rows := [][]string{{"v", "hello"}}
		got, err := buildMaxWidth(t, 10, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Elastic column must still be present with width 3.
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		// The elastic column header cell has at least 3 chars of content width.
		parts := strings.Split(lines[0], "|")
		if len(parts) < 3 {
			t.Fatalf("unexpected header format: %q", lines[0])
		}
		// parts[2] is " E   " or similar — inner content width = rune count - 2 spaces
		cellContent := strings.TrimSpace(parts[2])
		if utf8.RuneCountInString(parts[2])-2 < 3 {
			t.Errorf("elastic column cell too narrow: %q", cellContent)
		}
	})

	t.Run("elastic column has MaxWidth ceiling", func(t *testing.T) {
		// budget > MaxWidth → elastic clamped to MaxWidth.
		// cols: "A" (width=3) + "Elastic" (Elastic, MaxWidth=8)
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 40 → budget = 40 - 7 - 3 = 30 > MaxWidth=8 → width = 8
		// table total = 7 + 3 + 8 = 18 < maxWidth
		cols := []Column{
			{Name: "A"},
			{Name: "Elastic", Elastic: true, MaxWidth: 8},
		}
		rows := [][]string{{"hi", "This is very long cell content indeed"}}
		got, err := buildMaxWidth(t, 40, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Elastic column must be capped at 8.
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		dataLine := lines[2]
		// Total row width should be 18, not 40.
		if w := rowWidth(dataLine); w != 18 {
			t.Errorf("data row width = %d, want 18 (table narrower than maxWidth); line=%q", w, dataLine)
		}
		// Truncated with default marker.
		if !strings.Contains(dataLine, "...") {
			t.Errorf("expected truncation marker in %q", dataLine)
		}
	})

	t.Run("MinWidth overrides budget", func(t *testing.T) {
		// col.MinWidth=15, budget=8 → widths[elasticIdx] = max(8, 3, 15) = 15
		// cols: "Long" (4) + "E" (Elastic, MinWidth=15)
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 22 → budget = 22 - 7 - 4 = 11 ... let me recalculate
		// "Long" header natural width = max(4, 0, 3) = 4, but row "longvalue" (9) > 4 → 9
		// maxWidth = 20 → budget = 20 - 7 - 9 = 4 < 15 → elastic width = max(4,3,15) = 15
		cols := []Column{
			{Name: "Long"},
			{Name: "E", Elastic: true, MinWidth: 15},
		}
		rows := [][]string{{"longvalue", "x"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Elastic column header cell should have visual width of at least 15.
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		parts := strings.Split(lines[0], "|")
		if len(parts) < 3 {
			t.Fatalf("unexpected header format: %q", lines[0])
		}
		// inner cell width = len(parts[2]) - 2 (surrounding spaces)
		innerWidth := utf8.RuneCountInString(parts[2]) - 2
		if innerWidth < 15 {
			t.Errorf("elastic column width = %d, want >= 15 (MinWidth)", innerWidth)
		}
	})

	t.Run("custom TruncMarker", func(t *testing.T) {
		// Custom marker "…" (single rune) on elastic column.
		// cols: "A" (width=3) + "B" (Elastic, MaxWidth=10, TruncMarker="…")
		// separatorOverhead = 7, maxWidth = 20
		// budget = 20 - 7 - 3 = 10 = MaxWidth → elastic width = 10
		// cell "123456789012345" (15 runes) > 10 → truncated: "123456789…" (9+1=10)
		cols := []Column{
			{Name: "A"},
			{Name: "B", Elastic: true, TruncMarker: "…"},
		}
		rows := [][]string{{"hi", "123456789012345"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "…") {
			t.Errorf("expected custom marker '…' in %q", got)
		}
		if strings.Contains(got, "123456789012345") {
			t.Errorf("expected truncated content, got full content in %q", got)
		}
	})

	t.Run("right-aligned elastic column", func(t *testing.T) {
		// Elastic column with RightAlign=true.
		// cols: "A" (width=3) + "Num" (Elastic, RightAlign=true)
		// separatorOverhead = 7, maxWidth = 20
		// budget = 20 - 7 - 3 = 10
		// cell "42" (2 runes) ≤ 10 → right-padded to width 10: "        42"
		cols := []Column{
			{Name: "A"},
			{Name: "Num", Elastic: true, RightAlign: true},
		}
		rows := [][]string{{"hi", "42"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		dataLine := lines[2]
		// Right-aligned: "42" should appear immediately before " |" at the end.
		if !strings.HasSuffix(dataLine, "42 |") {
			t.Errorf("expected right-aligned value '42 |' at end of %q", dataLine)
		}
		// Separator row must have "----:" colon for right alignment.
		sepLine := lines[1]
		if !strings.Contains(sepLine, ":") {
			t.Errorf("expected ':' in separator for right-aligned column: %q", sepLine)
		}
	})

	t.Run("error no elastic column", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A"}, Column{Name: "B"})
		tb.Row("x", "y")
		_, err := tb.BuildMaxWidth(30)
		if err == nil {
			t.Error("expected error for table with no elastic column, got nil")
		}
	})

	t.Run("error two elastic columns", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(
			Column{Name: "A", Elastic: true},
			Column{Name: "B", Elastic: true},
		)
		tb.Row("x", "y")
		_, err := tb.BuildMaxWidth(30)
		if err == nil {
			t.Error("expected error for table with two elastic columns, got nil")
		}
	})

	t.Run("error maxWidth zero", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A", Elastic: true})
		tb.Row("x")
		_, err := tb.BuildMaxWidth(0)
		if err == nil {
			t.Error("expected error for maxWidth=0, got nil")
		}
	})

	t.Run("error maxWidth negative", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A", Elastic: true})
		tb.Row("x")
		_, err := tb.BuildMaxWidth(-5)
		if err == nil {
			t.Error("expected error for maxWidth=-5, got nil")
		}
	})

	t.Run("single-column elastic table", func(t *testing.T) {
		// separatorOverhead = 1 + 1*3 = 4
		// cols: "Desc" (Elastic)
		// maxWidth = 14 → budget = 14 - 4 = 10
		// natural width of "Desc" header = max(4,0,3) = 4, but budget=10 wins
		// Actually: natural width is scanned first (max over header/rows), then
		// budget replaces it for elastic col. Row "hello" (5) → natural width=5.
		// budget = 14 - 4 - 0 (no other cols) = 10 → elastic width = max(10,3,0)=10
		cols := []Column{{Name: "Desc", Elastic: true}}
		rows := [][]string{{"hello"}}
		got, err := buildMaxWidth(t, 14, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		dataLine := lines[2]
		// Row width = 1 + (10+3) = 14
		if w := rowWidth(dataLine); w != 14 {
			t.Errorf("data row width = %d, want 14; line=%q", w, dataLine)
		}
	})

	t.Run("exact fit content", func(t *testing.T) {
		// Elastic column content length == budget exactly → no truncation marker.
		// cols: "A" (width=3) + "B" (Elastic)
		// separatorOverhead = 7, maxWidth = 20
		// budget = 20 - 7 - 3 = 10
		// cell exactly 10 runes: "1234567890"
		cols := []Column{
			{Name: "A"},
			{Name: "B", Elastic: true},
		}
		rows := [][]string{{"hi", "1234567890"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(got, "...") {
			t.Errorf("unexpected truncation for exact-fit content in %q", got)
		}
		if !strings.Contains(got, "1234567890") {
			t.Errorf("expected full content '1234567890' in %q", got)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		if w := rowWidth(lines[2]); w != 20 {
			t.Errorf("data row width = %d, want 20; line=%q", w, lines[2])
		}
	})

	t.Run("empty table returns builder without error", func(t *testing.T) {
		// Covers the len(t.rows)==0 early-return path (markdown.go:725-727).
		var mb Builder
		tb := mb.NewTable(Column{Name: "Col", Elastic: true})
		// No rows added.
		result, err := tb.BuildMaxWidth(40)
		if err != nil {
			t.Fatalf("unexpected error for empty table: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil Builder for empty table")
		}
	})

	t.Run("non-elastic column MaxWidth cap applied", func(t *testing.T) {
		// Covers markdown.go:749-751: a non-elastic column whose natural content
		// width exceeds its MaxWidth is capped before the budget is computed.
		// Col A: MaxWidth=5, content "verylongvalue" (13 runes) → capped to 5.
		// Col B: Elastic.
		// separatorOverhead = 1 + 2*3 = 7
		// maxWidth = 30 → budget = 30 - 7 - 5 = 18
		cols := []Column{
			{Name: "A", MaxWidth: 5},
			{Name: "B", Elastic: true},
		}
		rows := [][]string{{"verylongvalue", "content"}}
		got, err := buildMaxWidth(t, 30, cols, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
		}
		// The data row must be exactly maxWidth=30 wide.
		if w := rowWidth(lines[2]); w != 30 {
			t.Errorf("data row width = %d, want 30; line=%q", w, lines[2])
		}
		// Col A cell must be truncated to 5 runes (the MaxWidth cap).
		if !strings.Contains(lines[2], "ve...") {
			t.Errorf("expected col A truncated to 5 with marker, got line %q", lines[2])
		}
	})
}
