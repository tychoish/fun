package mdwn

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/irt"
)

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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
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
			assert.Equal(t, build(c.fn), c.want)
		})
	}
}

// --- fmt.Formatter and WriteTo ---

func TestFormat(t *testing.T) {
	t.Run("%s", func(t *testing.T) {
		var mb Builder
		mb.H1("Hi")
		assert.Equal(t, fmt.Sprintf("%s", &mb), "# Hi\n\n")
	})
	t.Run("%v", func(t *testing.T) {
		var mb Builder
		mb.Paragraph("hello")
		assert.Equal(t, fmt.Sprintf("%v", &mb), "hello\n\n")
	})
	t.Run("unknown verb", func(t *testing.T) {
		var mb Builder
		mb.Text("x")
		assert.Equal(t, fmt.Sprintf("%d", &mb), "%!d(mdwn.Builder)")
	})
	t.Run("WriteTo", func(t *testing.T) {
		var mb Builder
		mb.H1("Test")
		var out strings.Builder
		n, err := mb.WriteTo(&out)
		assert.NotError(t, err)
		got := out.String()
		assert.Equal(t, got, "# Test\n\n")
		assert.Equal(t, int(n), len(got))
	})
}

// --- Builder lifecycle ---

func TestBuilderLifecycle(t *testing.T) {
	t.Run("MakeBuilder pre-allocates capacity", func(t *testing.T) {
		mb := MakeBuilder(64)
		assert.NotNil(t, mb)
		assert.True(t, mb.Cap() >= 64)
		assert.Equal(t, mb.Len(), 0)
		mb.H1("hello")
		assert.True(t, mb.Len() > 0)
	})
	t.Run("MakeBuilder zero capacity", func(t *testing.T) {
		mb := MakeBuilder(0)
		assert.NotNil(t, mb)
		mb.Text("x")
		assert.Equal(t, mb.String(), "x")
	})
	t.Run("Release zeros length", func(t *testing.T) {
		mb := MakeBuilder(32)
		mb.Paragraph("content")
		assert.True(t, mb.Len() > 0)
		mb.Release()
		assert.Equal(t, mb.Len(), 0)
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
			assert.Equal(t, mb.Len(), c.wantLen)
			assert.Equal(t, mb.String(), c.wantStr)
		}
	})
	t.Run("Truncate then write", func(t *testing.T) {
		var mb Builder
		mb.Text("hello world")
		mb.Truncate(5)
		mb.Text("!")
		assert.Equal(t, mb.String(), "hello!")
	})
}

// --- HorizontalRule ---

func TestHorizontalRule(t *testing.T) {
	got := build(func(m *Builder) { m.HorizontalRule() })
	assert.Equal(t, got, "-----\n\n")
}

// --- Image ---

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
			assert.Equal(t, got, c.want)
		})
	}
}

// --- Task list items ---

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
			assert.Equal(t, got, c.want)
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
			assert.Equal(t, got, c.want)
		})
	}
}
