package mdwn

import (
	"iter"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/irt"
)

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
		assert.True(t, len(lines) >= 4)
		assert.True(t, strings.HasPrefix(lines[0], "| Name"))
		assert.True(t, strings.Contains(lines[1], "---"))
		assert.True(t, strings.Contains(lines[1], ":"))
		assert.True(t, strings.Contains(lines[2], "Alice"))
		assert.True(t, strings.Contains(lines[3], "Bob"))
	})
	t.Run("pipe escaping", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "Val"}).Row("a|b").Build()
		})
		assert.True(t, strings.Contains(got, `a\|b`))
	})
	t.Run("column alignment", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(
				Column{Name: "L"},
				Column{Name: "R", RightAlign: true},
			).Row("left", "1234").Build()
		})
		dataRow := strings.Split(got, "\n")[2]
		assert.True(t, strings.Contains(dataRow, "| left"))
		assert.True(t, strings.Contains(dataRow, "1234 |"))
	})
	t.Run("MinWidth", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X", MinWidth: 10}).Row("hi").Build()
		})
		parts := strings.Split(strings.Split(got, "\n")[0], "|")
		assert.True(t, len(parts) >= 2)
		cellWidth := len(parts[1]) - 2
		assert.True(t, cellWidth >= 10)
	})
	t.Run("MaxWidth truncates with default marker", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "T", MaxWidth: 8}).
				Row("short").
				Row("this is a very long value").Build()
		})
		longRow := strings.Split(got, "\n")[3]
		assert.True(t, !strings.Contains(longRow, "this is a very long value"))
		assert.True(t, strings.Contains(longRow, "..."))
	})
	t.Run("MaxWidth narrow truncation (no marker fits)", func(t *testing.T) {
		// MaxWidth=3 equals len("..."), so the else branch slices without appending marker.
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X", MaxWidth: 3}).Row("hello").Build()
		})
		assert.True(t, !strings.Contains(strings.Split(got, "\n")[2], "hello"))
	})
	t.Run("custom TruncMarker", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "T", MaxWidth: 10, TruncMarker: "…"}).
				Row("this is definitely longer than ten characters").Build()
		})
		assert.True(t, strings.Contains(got, "…"))
	})
	t.Run("Rows helper", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "K"}, Column{Name: "V"}).
				Rows([][]string{{"a", "1"}, {"b", "2"}}).Build()
		})
		assert.True(t, strings.Contains(got, "| a"))
		assert.True(t, strings.Contains(got, "| b"))
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
		assert.True(t, strings.Contains(got, "| x"))
		assert.True(t, strings.Contains(got, "| y"))
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
		assert.True(t, strings.Contains(got, "alpha"))
		assert.True(t, strings.Contains(got, "99"))
	})
	t.Run("NewTableWithColumns", func(t *testing.T) {
		cols := []Column{{Name: "X"}, {Name: "Y", RightAlign: true}}
		got := build(func(m *Builder) {
			m.NewTableWithColumns(cols).Row("foo", "42").Build()
		})
		assert.True(t, strings.Contains(got, "foo"))
		assert.True(t, strings.Contains(got, "42"))
		assert.True(t, strings.Contains(got, "X"))
		assert.True(t, strings.Contains(got, "Y"))
	})
	t.Run("empty rows produces no output", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "A"}, Column{Name: "B"}).Build()
		})
		assert.Equal(t, got, "")
	})
	t.Run("Row no cells is ignored", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).Row().Build()
		})
		assert.Equal(t, got, "")
	})
	t.Run("ExtendRow empty seq is ignored", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).
				ExtendRow(func(yield func(string) bool) {}).Build()
		})
		assert.Equal(t, got, "")
	})
	t.Run("ends with newline", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.NewTable(Column{Name: "X"}).Row("v").Build()
		})
		assert.True(t, strings.HasSuffix(got, "\n"))
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
		assert.True(t, strings.Contains(got, "Alice"))
		assert.True(t, strings.Contains(got, "Bob"))
		assert.True(t, strings.Contains(got, "Name"))
		assert.True(t, strings.Contains(got, "Count"))
	})
	t.Run("empty seq produces no output", func(t *testing.T) {
		got := build(func(m *Builder) {
			m.KVTable(irt.MakeKV("K", "V"), func(yield func(string, string) bool) {})
		})
		assert.Equal(t, got, "")
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
	assert.Equal(t, got, want)
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
	assert.Equal(t, got, want)
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
	assert.True(t, len(lines) >= 4)
	var colWidths []int
	for _, line := range lines[2:] {
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		colWidths = append(colWidths, utf8.RuneCountInString(parts[2]))
	}
	for i := 1; i < len(colWidths); i++ {
		assert.Equal(t, colWidths[i], colWidths[0])
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
		assert.Equal(t, runeByteOffset(b, c.n), c.want)
	}
}

// --- BuildMaxWidth ---

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
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		dataLine := lines[2]
		// The data row must be exactly maxWidth runes wide.
		assert.Equal(t, rowWidth(dataLine), 20)
		// Elastic column must contain the truncation marker.
		assert.True(t, strings.Contains(dataLine, "..."))
		// Full original content must NOT appear.
		assert.True(t, !strings.Contains(got, "This is quite long content here"))
	})

	t.Run("no truncation needed", func(t *testing.T) {
		// elastic column content fits within budget — no truncation.
		cols := []Column{
			{Name: "Key"},
			{Name: "Desc", Elastic: true},
		}
		rows := [][]string{{"abc", "short"}}
		got, err := buildMaxWidth(t, 30, cols, rows)
		assert.NotError(t, err)
		assert.True(t, !strings.Contains(got, "..."))
		assert.True(t, strings.Contains(got, "short"))
	})

	t.Run("other columns exhaust budget", func(t *testing.T) {
		// budget < 3 → elastic clamped to max(3, MinWidth) = 3; no error.
		cols := []Column{
			{Name: "LongColName"},
			{Name: "E", Elastic: true},
		}
		rows := [][]string{{"v", "hello"}}
		got, err := buildMaxWidth(t, 10, cols, rows)
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		// The elastic column header cell has at least 3 chars of content width.
		parts := strings.Split(lines[0], "|")
		assert.True(t, len(parts) >= 3)
		// inner content width = rune count - 2 spaces
		assert.True(t, utf8.RuneCountInString(parts[2])-2 >= 3)
	})

	t.Run("elastic column has MaxWidth ceiling", func(t *testing.T) {
		// budget > MaxWidth → elastic clamped to MaxWidth.
		cols := []Column{
			{Name: "A"},
			{Name: "Elastic", Elastic: true, MaxWidth: 8},
		}
		rows := [][]string{{"hi", "This is very long cell content indeed"}}
		got, err := buildMaxWidth(t, 40, cols, rows)
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		dataLine := lines[2]
		// Total row width should be 18, not 40.
		assert.Equal(t, rowWidth(dataLine), 18)
		// Truncated with default marker.
		assert.True(t, strings.Contains(dataLine, "..."))
	})

	t.Run("MinWidth overrides budget", func(t *testing.T) {
		cols := []Column{
			{Name: "Long"},
			{Name: "E", Elastic: true, MinWidth: 15},
		}
		rows := [][]string{{"longvalue", "x"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		parts := strings.Split(lines[0], "|")
		assert.True(t, len(parts) >= 3)
		// inner cell width = len(parts[2]) - 2 (surrounding spaces)
		innerWidth := utf8.RuneCountInString(parts[2]) - 2
		assert.True(t, innerWidth >= 15)
	})

	t.Run("custom TruncMarker", func(t *testing.T) {
		// Custom marker "…" (single rune) on elastic column.
		cols := []Column{
			{Name: "A"},
			{Name: "B", Elastic: true, TruncMarker: "…"},
		}
		rows := [][]string{{"hi", "123456789012345"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		assert.NotError(t, err)
		assert.True(t, strings.Contains(got, "…"))
		assert.True(t, !strings.Contains(got, "123456789012345"))
	})

	t.Run("right-aligned elastic column", func(t *testing.T) {
		// Elastic column with RightAlign=true.
		cols := []Column{
			{Name: "A"},
			{Name: "Num", Elastic: true, RightAlign: true},
		}
		rows := [][]string{{"hi", "42"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		dataLine := lines[2]
		// Right-aligned: "42" should appear immediately before " |" at the end.
		assert.True(t, strings.HasSuffix(dataLine, "42 |"))
		// Separator row must have "----:" colon for right alignment.
		sepLine := lines[1]
		assert.True(t, strings.Contains(sepLine, ":"))
	})

	t.Run("error no elastic column", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A"}, Column{Name: "B"})
		tb.Row("x", "y")
		_, err := tb.BuildMaxWidth(30)
		assert.Error(t, err)
	})

	t.Run("error two elastic columns", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(
			Column{Name: "A", Elastic: true},
			Column{Name: "B", Elastic: true},
		)
		tb.Row("x", "y")
		_, err := tb.BuildMaxWidth(30)
		assert.Error(t, err)
	})

	t.Run("error maxWidth zero", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A", Elastic: true})
		tb.Row("x")
		_, err := tb.BuildMaxWidth(0)
		assert.Error(t, err)
	})

	t.Run("error maxWidth negative", func(t *testing.T) {
		var mb Builder
		tb := mb.NewTable(Column{Name: "A", Elastic: true})
		tb.Row("x")
		_, err := tb.BuildMaxWidth(-5)
		assert.Error(t, err)
	})

	t.Run("single-column elastic table", func(t *testing.T) {
		cols := []Column{{Name: "Desc", Elastic: true}}
		rows := [][]string{{"hello"}}
		got, err := buildMaxWidth(t, 14, cols, rows)
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		dataLine := lines[2]
		// Row width = 1 + (10+3) = 14
		assert.Equal(t, rowWidth(dataLine), 14)
	})

	t.Run("exact fit content", func(t *testing.T) {
		// Elastic column content length == budget exactly → no truncation marker.
		cols := []Column{
			{Name: "A"},
			{Name: "B", Elastic: true},
		}
		rows := [][]string{{"hi", "1234567890"}}
		got, err := buildMaxWidth(t, 20, cols, rows)
		assert.NotError(t, err)
		assert.True(t, !strings.Contains(got, "..."))
		assert.True(t, strings.Contains(got, "1234567890"))
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		assert.Equal(t, rowWidth(lines[2]), 20)
	})

	t.Run("empty table returns builder without error", func(t *testing.T) {
		// Covers the len(t.rows)==0 early-return path.
		var mb Builder
		tb := mb.NewTable(Column{Name: "Col", Elastic: true})
		// No rows added.
		result, err := tb.BuildMaxWidth(40)
		assert.NotError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("non-elastic column MaxWidth cap applied", func(t *testing.T) {
		// Covers a non-elastic column whose natural content width exceeds its
		// MaxWidth being capped before the budget is computed.
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
		assert.NotError(t, err)
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		assert.True(t, len(lines) >= 3)
		// The data row must be exactly maxWidth=30 wide.
		assert.Equal(t, rowWidth(lines[2]), 30)
		// Col A cell must be truncated to 5 runes (the MaxWidth cap).
		assert.True(t, strings.Contains(lines[2], "ve..."))
	})
}
