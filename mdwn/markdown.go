package mdwn

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/strut"
)

const (
	newLineByte = '\n'
	newLineStr  = "\n"
)

// Column describes a single column in a markdown table.
type Column struct {
	Name        string
	RightAlign  bool
	MinWidth    int    // minimum column width; 0 means auto-size to content
	MaxWidth    int    // maximum column width; 0 means unlimited; truncates with TruncMarker
	TruncMarker string // suffix appended to truncated cells; defaults to "..."
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

// Builder wraps strut.Mutable with methods for writing markdown
// elements. Many methods return the receiver for chaining. Call
// String() or WriteTo to get the result.
type Builder struct{ strut.Mutable }

// MakeBuilder returns a new Builder pre-allocated to the given byte capacity.
func MakeBuilder(capacity int) *Builder { return &Builder{Mutable: *strut.MakeMutable(capacity)} }

// Release returns the Builder's underlying buffer to the pool.  The
// Builder must not be used after Release is called.
func (m *Builder) Release() { m.Mutable.Release() }

// Truncate shortens the Builder's content to targetSize bytes, clamped
// to [0, Len()]. It does not release memory; use Release to return the
// buffer to the pool when the Builder is no longer needed.
func (m *Builder) Truncate(targetSize int) { m.Mutable = m.Mutable[:max(0, min(targetSize, m.Len()))] }

// H1 writes a level-1 heading ("# text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H1(text ...string) *Builder { return m.heading(1, text...) }

// H2 writes a level-2 heading ("## text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H2(text ...string) *Builder { return m.heading(2, text...) }

// H3 writes a level-3 heading ("### text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H3(text ...string) *Builder { return m.heading(3, text...) }

// H4 writes a level-4 heading ("#### text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H4(text ...string) *Builder { return m.heading(4, text...) }

// H5 writes a level-5 heading ("##### text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H5(text ...string) *Builder { return m.heading(5, text...) }

// H6 writes a level-6 heading ("###### text") followed by a blank line.
// Multiple parts are concatenated directly without a separator.
func (m *Builder) H6(text ...string) *Builder { return m.heading(6, text...) }

func (m *Builder) heading(level int, text ...string) *Builder {
	m.Concat(strings.Repeat("#", level), " ")
	m.Concat(text...)
	m.NLines(2)
	return m
}

// Paragraph writes text followed by a blank line. Multiple parts are
// concatenated directly without a separator.
func (m *Builder) Paragraph(text ...string) *Builder {
	m.Concat(text...)
	m.NLines(2)
	return m
}

// ParagraphWords writes parts joined with spaces followed by a blank line.
func (m *Builder) ParagraphWords(words ...string) *Builder {
	return m.Paragraph(joinSpace(words))
}

// H1Words writes a level-1 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H1Words(words ...string) *Builder { return m.heading(1, joinSpace(words)) }

// H2Words writes a level-2 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H2Words(words ...string) *Builder { return m.heading(2, joinSpace(words)) }

// H3Words writes a level-3 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H3Words(words ...string) *Builder { return m.heading(3, joinSpace(words)) }

// H4Words writes a level-4 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H4Words(words ...string) *Builder { return m.heading(4, joinSpace(words)) }

// H5Words writes a level-5 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H5Words(words ...string) *Builder { return m.heading(5, joinSpace(words)) }

// H6Words writes a level-6 heading with parts joined by a single space,
// followed by a blank line.
func (m *Builder) H6Words(words ...string) *Builder { return m.heading(6, joinSpace(words)) }

// ItalicParagraph writes _text_ followed by a blank line. Multiple parts are
// concatenated directly without a separator before the italic markers are applied.
func (m *Builder) ItalicParagraph(text ...string) *Builder {
	m.Italic(text...)
	m.NLines(2)
	return m
}

// KV writes a **key**: value line followed by a newline.
func (m *Builder) KV(key, val string) *Builder {
	m.Concat("**", key, "**: ", val)
	m.Line()
	return m
}

// FromKV writes a **key**: value line followed by a newline, reading key and
// value from an irt.KV[string, string].
func (m *Builder) FromKV(kv irt.KV[string, string]) *Builder {
	m.Concat("**", kv.Key, "**: ", kv.Value)
	m.Line()
	return m
}

// FromKVany writes a **key**: value line followed by a newline, reading key
// and value from an irt.KV[string, any] and formatting the value with %v.
func (m *Builder) FromKVany(kv irt.KV[string, any]) *Builder {
	m.Mprintf("**%s**: %v", kv.Key, kv.Value)
	m.Line()
	return m
}

// KVany writes a **key**: value line followed by a newline, formatting value with %v.
func (m *Builder) KVany(k string, v any) *Builder {
	m.Mprintf("**%s**: %v", k, v)
	m.Line()
	return m
}

// KVs writes a KV line for each irt.KV[string, string] argument.
func (m *Builder) KVs(kvs ...irt.KV[string, string]) *Builder {
	for _, kv := range kvs {
		m.FromKV(kv)
	}
	return m
}

// KVanys writes a KV line for each irt.KV[string, any] argument.
func (m *Builder) KVanys(kvs ...irt.KV[string, any]) *Builder {
	for _, kv := range kvs {
		m.FromKVany(kv)
	}
	return m
}

// ExtendKV writes a KV line for each pair yielded by seq.
func (m *Builder) ExtendKV(seq iter.Seq2[string, string]) *Builder {
	for k, v := range seq {
		m.KV(k, v)
	}
	return m
}

// ExtendKVany writes a KV line for each pair yielded by seq, formatting
// values with %v.
func (m *Builder) ExtendKVany(seq iter.Seq2[string, any]) *Builder {
	for k, v := range seq {
		m.KVany(k, v)
	}
	return m
}

// ExtendKVSeq writes a KV line for each irt.KV[string, string] yielded by seq.
func (m *Builder) ExtendKVSeq(seq iter.Seq[irt.KV[string, string]]) *Builder {
	for kv := range seq {
		m.FromKV(kv)
	}
	return m
}

// ExtendKVanySeq writes a KV line for each irt.KV[string, any] yielded by seq,
// formatting values with %v.
func (m *Builder) ExtendKVanySeq(seq iter.Seq[irt.KV[string, any]]) *Builder {
	for kv := range seq {
		m.FromKVany(kv)
	}
	return m
}

// BulletListItem writes a single "- item" line. Multiple parts are
// concatenated directly to form one item's text — they do not produce
// multiple list items. For multiple items use BulletList.
func (m *Builder) BulletListItem(item ...string) *Builder {
	m.PushString("- ")
	m.Concat(item...)
	m.Line()
	return m
}

// BulletListItemWords writes a single "- words..." line with parts joined
// by a single space.
func (m *Builder) BulletListItemWords(words ...string) *Builder {
	return m.BulletListItem(joinSpace(words))
}

// BulletList writes an unordered list followed by a blank line.
// Does nothing if no items are provided.
func (m *Builder) BulletList(items ...string) *Builder { return m.ExtendBulletList(irt.Slice(items)) }

// ExtendBulletList writes an unordered list from a sequence, followed by a
// blank line. Does nothing if the sequence is empty.
func (m *Builder) ExtendBulletList(seq iter.Seq[string]) *Builder {
	wrote := false
	for item := range irt.RemoveZeros(seq) {
		m.BulletListItem(item)
		wrote = true
	}
	m.WhenLine(wrote)
	return m
}

// OrderedListItem writes a single "1. item" line. Multiple parts are
// concatenated directly to form one item's text — they do not produce
// multiple list items. For multiple items use OrderedList. Markdown renderers
// auto-number ordered list items regardless of the literal number used.
func (m *Builder) OrderedListItem(item ...string) *Builder {
	m.Concat("1. ")
	m.Concat(item...)
	m.Line()
	return m
}

// OrderedListItemWords writes a single "1. words..." line with parts joined
// by a single space.
func (m *Builder) OrderedListItemWords(words ...string) *Builder {
	return m.OrderedListItem(joinSpace(words))
}

// OrderedList writes a numbered list followed by a blank line.
// Does nothing if no items are provided.
func (m *Builder) OrderedList(items ...string) *Builder {
	m.growToAccomidate(sumLengthOfStrings(items))
	for _, item := range items {
		m.OrderedListItem(item)
	}
	m.WhenLine(len(items) > 0)
	return m
}

// joinSpace joins parts with a single space. Returns "" for zero parts.
func joinSpace(parts []string) string { return strings.Join(parts, " ") }

func sumLengthOfStrings(strs []string) (total int) {
	for _, str := range strs {
		total += len(str)
	}
	return total
}

// growToAccomidate ensures at least l more bytes can be written without
// reallocation. bytes.Buffer.Grow(n) already accounts for existing free
// capacity, so passing l directly is correct and sufficient.
func (m *Builder) growToAccomidate(l int) { m.Grow(l) }

// ExtendOrderedList writes a numbered list from a sequence, followed by a
// blank line. Does nothing if the sequence is empty.
func (m *Builder) ExtendOrderedList(seq iter.Seq[string]) *Builder {
	var wrote bool
	for item := range irt.RemoveZeros(seq) {
		m.OrderedListItem(item)
		wrote = true
	}
	m.WhenLine(wrote)
	return m
}

// writePrefixedLines writes each line of b to m with prefix prepended, then
// writes a trailing blank line. b must already have trailing newlines trimmed.
// Callers are responsible for the growToAccomidate call before this.
func (m *Builder) writePrefixedLines(b []byte, prefix string) {
	for line := range bytes.SplitSeq(b, []byte{newLineByte}) {
		m.PushString(prefix)
		m.PushBytes(line)
		m.Line()
	}
	m.Line()
}

// BlockQuote prefixes each line of text with "> " and appends a blank line.
// Multiple parts are concatenated directly before quoting. Blank lines within
// the text are preserved so nested markdown elements render correctly.
// Trailing newlines are trimmed; text consisting solely of newlines (or empty
// string) produces no output.
func (m *Builder) BlockQuote(text ...string) *Builder {
	// strings.Join copies once for the multi-part case; []byte(...) copies
	// once more — two copies total, same as the old single-string path.
	b := bytes.TrimRight([]byte(strings.Join(text, "")), newLineStr)
	if len(b) == 0 {
		return m
	}
	m.growToAccomidate(len(b))
	m.writePrefixedLines(b, "> ")
	return m
}

// BlockQuoteWith builds inner content using the provided function and wraps
// the result in a block quote, enabling nested block-quote elements.
// Uses the underlying byte buffer directly to avoid an intermediate string copy.
func (m *Builder) BlockQuoteWith(fn func(*Builder)) *Builder {
	var inner Builder
	fn(&inner)

	b := bytes.TrimRight(inner.Bytes(), newLineStr)
	if len(b) == 0 {
		return m
	}
	m.growToAccomidate(len(b))
	m.writePrefixedLines(b, "> ")
	return m
}

// FencedCodeWith builds inner content using the provided function and wraps
// the result in a fenced code block with an optional language identifier.
// Returns m unchanged if fn produces no content.
func (m *Builder) FencedCodeWith(lang string, fn func(*Builder)) *Builder {
	var inner Builder
	fn(&inner)

	b := bytes.TrimRight(inner.Bytes(), newLineStr)
	if len(b) == 0 {
		return m
	}
	m.Concat("```", lang)
	m.Line()
	m.PushBytes(b)
	m.Line()
	m.PushString("```")
	m.NLines(2)
	return m
}

// FencedCode writes a fenced code block. lang is written as the language
// identifier on the opening fence (e.g. "go", "sh"); pass "" for no tag.
// If code does not already end with a newline, one is appended before the
// closing fence.
func (m *Builder) FencedCode(lang, code string) *Builder {
	m.Concat("```", lang)
	m.Line()
	m.PushString(code)
	m.WhenLine(len(code) == 0 || code[len(code)-1] != newLineByte)
	m.PushString("```")
	m.NLines(2)
	return m
}

// IndentWith builds inner content using the provided function and prefixes
// each line of the result with indent, followed by a blank line. Useful for
// code blocks beneath list items (4-space indent) or other indented sections.
// Returns m unchanged if fn produces no content.
func (m *Builder) IndentWith(indent string, fn func(*Builder)) *Builder {
	var inner Builder
	fn(&inner)

	b := bytes.TrimRight(inner.Bytes(), newLineStr)
	if len(b) == 0 {
		return m
	}
	m.growToAccomidate(len(b))
	m.writePrefixedLines(b, indent)
	return m
}

// IndentedCode writes code as a 4-space indented code block (CommonMark
// alternative to fenced code blocks). Trailing newlines are trimmed.
// Returns m unchanged if code is empty.
func (m *Builder) IndentedCode(code string) *Builder {
	b := bytes.TrimRight([]byte(code), newLineStr)
	if len(b) == 0 {
		return m
	}
	m.growToAccomidate(len(b))
	m.writePrefixedLines(b, "    ")
	return m
}

// IndentedCodeWith builds inner content using the provided function and writes
// it as a 4-space indented code block. Returns m unchanged if fn produces no content.
func (m *Builder) IndentedCodeWith(fn func(*Builder)) *Builder {
	return m.IndentWith("    ", fn)
}

// ParagraphBreak writes two newlines, creating a markdown paragraph separator.
func (m *Builder) ParagraphBreak() *Builder { m.NLines(2); return m }

// WriteTo drains the accumulated content to w without copying to an
// intermediate string.
func (m *Builder) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.Mutable)
	return int64(n), err
}

// Format implements [fmt.Formatter]. For the %s and %v verbs it writes the
// accumulated content directly to f without allocating an intermediate string.
// This makes Builder usable in fmt.Fprintf and similar calls without String().
func (m *Builder) Format(f fmt.State, verb rune) {
	switch verb {
	case 's', 'v':
		f.Write(m.Mutable)
	default:
		fmt.Fprintf(f, "%%!%c(mdwn.Builder)", verb)
	}
}

// Push<op> wrappers — chainable forms of the underlying strut.Mutable methods
// that do not themselves return *Builder. These allow full method chaining
// without breaking the chain to call low-level buffer operations.

// PushString appends s to the builder and returns the receiver for chaining.
func (m *Builder) PushString(s string) *Builder { m.Mutable.PushString(s); return m }

// PushBytes appends b to the builder and returns the receiver for chaining.
func (m *Builder) PushBytes(b []byte) *Builder { m.Mutable.PushBytes(b); return m }

// PushLine appends a newline to the builder and returns the receiver for chaining.
func (m *Builder) PushLine() *Builder { m.Mutable.Line(); return m }

// PushNLines appends n newlines to the builder and returns the receiver for chaining.
func (m *Builder) PushNLines(n int) *Builder { m.Mutable.NLines(n); return m }

// PushConcat appends all parts concatenated to the builder and returns the
// receiver for chaining.
func (m *Builder) PushConcat(parts ...string) *Builder { m.Mutable.Concat(parts...); return m }

// PushKV writes a **key**: value line followed by a newline and returns the receiver for chaining.
func (m *Builder) PushKV(key, val string) *Builder { return m.KV(key, val) }

// PushKVany writes a **key**: value line followed by a newline, formatting value with %v,
// and returns the receiver for chaining.
func (m *Builder) PushKVany(key string, val any) *Builder { return m.KVany(key, val) }

// PushFromKV writes a **key**: value line from an irt.KV[string, string] followed by a newline
// and returns the receiver for chaining.
func (m *Builder) PushFromKV(kv irt.KV[string, string]) *Builder { return m.FromKV(kv) }

// PushFromKVany writes a **key**: value line from an irt.KV[string, any] followed by a newline,
// formatting the value with %v, and returns the receiver for chaining.
func (m *Builder) PushFromKVany(kv irt.KV[string, any]) *Builder { return m.FromKVany(kv) }

// Text writes parts concatenated to the builder and returns the receiver for
// chaining. Use this to intersperse plain text with inline formatting methods:
//
//	mb.Bold("Note").Text(": see ").Link("docs", url).ParagraphBreak()
func (m *Builder) Text(parts ...string) *Builder { m.Concat(parts...); return m }

// TextWords writes parts joined with spaces to the builder and returns the
// receiver for chaining.
func (m *Builder) TextWords(parts ...string) *Builder { m.JoinStrings(parts, " "); return m }

// Bold writes **parts** to the builder. Multiple parts are concatenated
// directly without a separator. Use BoldWords to join parts with spaces.
func (m *Builder) Bold(parts ...string) *Builder {
	m.PushString("**")
	m.Concat(parts...)
	m.PushString("**")
	return m
}

// BoldWords writes **parts** to the builder with parts joined by a single space.
func (m *Builder) BoldWords(parts ...string) *Builder {
	m.PushString("**")
	m.JoinStrings(parts, " ")
	m.PushString("**")
	return m
}

// Italic writes _parts_ to the builder. Multiple parts are concatenated
// directly without a separator. Use ItalicWords to join parts with spaces.
func (m *Builder) Italic(parts ...string) *Builder {
	m.PushString("_")
	m.Concat(parts...)
	m.PushString("_")
	return m
}

// ItalicWords writes _parts_ to the builder with parts joined by a single space.
func (m *Builder) ItalicWords(parts ...string) *Builder {
	m.PushString("_")
	m.JoinStrings(parts, " ")
	m.PushString("_")
	return m
}

// Preformatted writes `parts` to the builder as an inline code span. Multiple
// parts are concatenated directly without a separator. Use PreformattedWords
// to join parts with spaces.
func (m *Builder) Preformatted(parts ...string) *Builder {
	m.PushString("`")
	m.Concat(parts...)
	m.PushString("`")
	return m
}

// PreformattedWords writes `parts` to the builder as an inline code span with
// parts joined by a single space.
func (m *Builder) PreformattedWords(parts ...string) *Builder {
	m.PushString("`")
	m.JoinStrings(parts, " ")
	m.PushString("`")
	return m
}

// Strikethrough writes ~~parts~~ to the builder. Multiple parts are
// concatenated directly without a separator. Use StrikethroughWords to join
// parts with spaces.
func (m *Builder) Strikethrough(parts ...string) *Builder {
	m.PushString("~~")
	m.Concat(parts...)
	m.PushString("~~")
	return m
}

// StrikethroughWords writes ~~parts~~ to the builder with parts joined by a
// single space.
func (m *Builder) StrikethroughWords(parts ...string) *Builder {
	m.PushString("~~")
	m.JoinStrings(parts, " ")
	m.PushString("~~")
	return m
}

// Link writes [text](url) to the builder.
func (m *Builder) Link(text, url string) *Builder { m.Concat("[", text, "](", url, ")"); return m }

// NewTable creates a Table attached to this Builder. Call Row on the
// returned builder to accumulate rows, then Build to render the table and
// resume chaining on Builder.
func (m *Builder) NewTable(cols ...Column) *Table { return m.NewTableWithColumns(cols) }

// NewTableWithColumns creates a Table from a slice of column definition.
func (m *Builder) NewTableWithColumns(cols []Column) *Table { return &Table{mb: m, cols: cols} }

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
		m := strut.NewMutable()
		m.WithReplaceAll(cell, "|", `\|`)
		row = append(row, m)
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
		row[i] = strut.NewMutable()
		row[i].WithReplaceAll(cell, "|", `\|`)
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

// Build renders the accumulated table into the parent Builder and returns it
// for further chaining. Column widths are auto-sized to content,
// lower-bounded by ColumnDef.MinWidth, at least 3 (minimum for a valid
// markdown separator), and capped by ColumnDef.MaxWidth when set.
// Cells exceeding MaxWidth are truncated with ColumnDef.TruncMarker ("...").
// The pooled Mutable cell buffers are released after rendering.
func (t *Table) Build() *Builder {
	if len(t.rows) == 0 {
		return t.mb
	}
	defer func() {
		for i := range t.rows {
			for j := range t.rows[i] {
				t.rows[i][j].Release()
				t.rows[i][j] = nil
			}
			t.rows[i] = nil
		}
		t.rows = nil
	}()

	// Compute per-column widths using rune count (visual width), not byte
	// length. Multi-byte Unicode characters (e.g. ♯ = 3 UTF-8 bytes, 1 rune)
	// must count as one column character to keep rows visually aligned.
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
	// Apply MaxWidth cap.
	for i, col := range t.cols {
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = max(col.MaxWidth, col.MinWidth, 3)
		}
	}

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
			needsTrunc := col.MaxWidth > 0 && cellLen > widths[i]
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
	tb := mb.NewTable(Column{Name: header.Key}, Column{Name: header.Value, RightAlign: true})
	irt.Apply2(seq, tb.kvRow)
	return tb.Build()
}

func (tb *Table) kvRow(k, v string) { tb.Row(k, v) }
