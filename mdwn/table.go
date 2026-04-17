package mdwn

import (
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/strut"
)

// Column describes a single column in a markdown table.
type Column struct {
	Name        string
	RightAlign  bool
	MinWidth    int    // minimum column width; 0 means auto-size to content
	MaxWidth    int    // maximum column width; 0 means unlimited; truncates with TruncMarker
	TruncMarker string // suffix appended to truncated cells; defaults to "..."
	Elastic     bool   // absorbs remaining width in BuildMaxWidth; ignored by Build
}

func (c Column) truncMarker() string {
	if c.TruncMarker != "" {
		return c.TruncMarker
	}
	return "..."
}

// runeByteOffset returns the byte index of the start of the (n+1)th
// rune in b, i.e. the byte position immediately after n runes. Used to
// truncate cell content at a rune boundary rather than a byte boundary.
// Returns len(b) if b contains fewer than n runes.
func runeByteOffset(b []byte, n int) int {
	pos := 0
	for range n {
		if pos >= len(b) {
			return len(b)
		}
		_, size := utf8.DecodeRune(b[pos:])
		pos += size
	}

	return pos
}

// Table accumulates table rows and renders a column-aligned markdown
// table when Build is called. Cells are pipe-escaped at insertion time using
// pooled strut.Mutable buffers; Build releases them after rendering.
type Table struct {
	mb   *Builder
	cols []Column
	rows [][]*strut.Mutable
}

// ExtendRow appends a single data row from a sequence. Cells are
// pipe-escaped immediately. Iterates directly to avoid an intermediate
// []string allocation.
func (t *Table) ExtendRow(seq iter.Seq[string]) *Table {
	var row []*strut.Mutable
	for cell := range seq {
		row = append(row, strut.MutableFrom(strings.ReplaceAll(cell, "|", `\|`)))
	}
	if len(row) > 0 {
		t.rows = append(t.rows, row)
	}
	return t
}

// Row appends a single data row. Cells are pipe-escaped immediately.
func (t *Table) Row(cells ...string) *Table {
	if len(cells) == 0 {
		return t
	}
	// make avoids the irt.GenerateN iterator + irt.Collect overhead.
	row := make([]*strut.Mutable, len(cells))
	for i, cell := range cells {
		row[i] = strut.MutableFrom(strings.ReplaceAll(cell, "|", `\|`))
	}
	t.rows = append(t.rows, row)
	return t
}

// Rows appends multiple data rows to the table.
func (t *Table) Rows(rows [][]string) *Table { return t.Extend(irt.Slice(rows)) }

// Extend appends rows from a sequence to the table.
func (t *Table) Extend(seq iter.Seq[[]string]) *Table {
	for row := range seq {
		t.Row(row...)
	}
	return t
}

// naturalWidths computes per-column widths from header names and row content,
// applying MinWidth and the 3-rune markdown minimum. Columns listed in skip
// are excluded from MaxWidth capping (used by BuildMaxWidth for the elastic
// column). Callers that want all caps applied pass an empty skip set.
//
// Uses rune count (visual width) rather than byte length so that multi-byte
// Unicode characters (e.g. ♯ = 3 UTF-8 bytes, 1 rune) count as one column
// character and keep rows visually aligned.
func (t *Table) naturalWidths(skipMaxCap map[int]struct{}) []int {
	widths := make([]int, len(t.cols))
	for i, col := range t.cols {
		widths[i] = max(utf8.RuneCountInString(col.Name), col.MinWidth, 3)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) {
				if rw := utf8.RuneCount([]byte(*cell)); rw > widths[i] {
					widths[i] = rw
				}
			}
		}
	}
	for i, col := range t.cols {
		if _, skip := skipMaxCap[i]; skip {
			continue
		}
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = max(col.MaxWidth, col.MinWidth, 3)
		}
	}
	return widths
}

// Build renders the accumulated table into the parent Builder and returns it
// for further chaining. Column widths are auto-sized to content,
// lower-bounded by ColumnDef.MinWidth, at least 3 (minimum for a valid
// markdown separator), and capped by ColumnDef.MaxWidth when set.
// Cells exceeding MaxWidth are truncated with ColumnDef.TruncMarker ("...").
// The pooled Mutable cell buffers are released after rendering.
// Column.Elastic is silently ignored by Build.
func (t *Table) Build() *Builder {
	if len(t.rows) == 0 {
		return t.mb
	}
	defer t.releaseRows()
	return t.buildWithWidths(t.naturalWidths(nil))
}

// BuildMaxWidth renders the accumulated table constraining total row width to
// maxWidth. Exactly one column must have Elastic set to true; that column
// absorbs the remaining width after all other columns are sized. If the
// elastic column also has MaxWidth set, it acts as a ceiling on how much
// space the column absorbs. Returns an error if maxWidth <= 0, no column has
// Elastic set, or more than one column has Elastic set.
// The pooled Mutable cell buffers are released after rendering.
func (t *Table) BuildMaxWidth(maxWidth int) (*Builder, error) {
	// Defer unconditionally so that pooled cell Mutables are released even when
	// validation fails after rows have already been added via Row/ExtendRow.
	defer t.releaseRows()

	if maxWidth <= 0 {
		return nil, ers.New("BuildMaxWidth: maxWidth must be greater than zero")
	}

	// Find the single elastic column.
	elasticIdx := -1
	for i, col := range t.cols {
		if col.Elastic {
			if elasticIdx >= 0 {
				return nil, ers.New("BuildMaxWidth: at most one column may have Elastic set")
			}
			elasticIdx = i
		}
	}
	if elasticIdx < 0 {
		return nil, ers.New("BuildMaxWidth: at least one column must have Elastic set")
	}

	if len(t.rows) == 0 {
		return t.mb, nil
	}

	// Compute natural widths, skipping MaxWidth cap on the elastic column.
	widths := t.naturalWidths(map[int]struct{}{elasticIdx: {}})

	// Compute the budget for the elastic column.
	// Separator overhead: 1 (leading "|") + len(cols)*3 (" x |" per column)
	separatorOverhead := 1 + len(t.cols)*3
	nonElasticSum := 0
	for i, w := range widths {
		if i != elasticIdx {
			nonElasticSum += w
		}
	}
	budget := maxWidth - separatorOverhead - nonElasticSum

	// Elastic column final width: floor at max(budget, 3, MinWidth).
	elasticCol := t.cols[elasticIdx]
	w := max(budget, 3, elasticCol.MinWidth)
	// Apply MaxWidth ceiling if set.
	if elasticCol.MaxWidth > 0 && w > elasticCol.MaxWidth {
		w = elasticCol.MaxWidth
	}
	widths[elasticIdx] = w

	return t.buildWithWidths(widths), nil
}

// releaseRows releases all pooled Mutable cell buffers and clears t.rows.
func (t *Table) releaseRows() {
	for i := range t.rows {
		for j := range t.rows[i] {
			t.rows[i][j].Release()
			t.rows[i][j] = nil
		}
		t.rows[i] = nil
	}
	t.rows = nil
}

// buildWithWidths renders the table using the provided per-column widths and
// returns the parent Builder for chaining. Callers are responsible for
// computing widths and releasing rows.
func (t *Table) buildWithWidths(widths []int) *Builder {
	// Pre-grow the output buffer: each row is ~(sum of widths + 3 per col + newline).
	rowWidth := 1 // leading "|"
	for _, w := range widths {
		rowWidth += w + 3 // " cell |"
	}
	t.mb.Grow(rowWidth * (len(t.rows) + 2)) // +2 for header and separator rows

	// Header row.
	t.mb.PushString("|")
	for i, col := range t.cols {
		t.mb.Concat(" ", col.Name, strings.Repeat(" ", widths[i]-utf8.RuneCountInString(col.Name)))
		t.mb.PushString(" |")
	}
	t.mb.Line()

	// Separator row: right-aligned columns use "----:" syntax.
	t.mb.PushString("|")
	for i, col := range t.cols {
		t.mb.PushString(" ")
		if col.RightAlign {
			t.mb.Concat(strings.Repeat("-", widths[i]-1), ":")
		} else {
			t.mb.PushString(strings.Repeat("-", widths[i]))
		}
		t.mb.PushString(" |")
	}
	t.mb.Line()

	// Data rows.
	for _, row := range t.rows {
		t.mb.PushString("|")
		for i, col := range t.cols {
			// Get the raw escaped bytes for this cell (nil = empty).
			var cellBytes []byte
			if i < len(row) && row[i] != nil {
				cellBytes = []byte(*row[i])
			}
			cellLen := utf8.RuneCount(cellBytes)

			// Truncate if cell exceeds capped column width.
			needsTrunc := (col.MaxWidth > 0 || col.Elastic) && cellLen > widths[i]
			if needsTrunc {
				marker := col.truncMarker()
				markerLen := utf8.RuneCountInString(marker)
				if widths[i] > markerLen {
					cutAt := runeByteOffset(cellBytes, widths[i]-markerLen)
					cellBytes = append(cellBytes[:cutAt:cutAt], marker...)
				} else {
					cellBytes = cellBytes[:runeByteOffset(cellBytes, widths[i])]
				}
				cellLen = widths[i]
			}

			pad := widths[i] - cellLen
			t.mb.PushString(" ")
			if col.RightAlign && pad > 0 {
				t.mb.PushString(strings.Repeat(" ", pad))
			}
			t.mb.PushBytes(cellBytes)
			if !col.RightAlign && pad > 0 {
				t.mb.PushString(strings.Repeat(" ", pad))
			}
			t.mb.PushString(" |")
		}
		t.mb.Line()
	}

	return t.mb
}

// KVTable builds a two-column key/value table from a two-value sequence and
// calls Build. The header parameter names the key and value columns. Values
// are formatted with fmt.Sprint.
func (mb *Builder) KVTable(header irt.KV[string, string], seq iter.Seq2[string, string]) *Builder {
	tb := mb.NewTable(
		Column{Name: header.Key},
		Column{Name: header.Value, RightAlign: true},
	)
	irt.Apply2(seq, tb.kvRow)
	return tb.Build()
}

func (tb *Table) kvRow(k, v string) { tb.Row(k, v) }
