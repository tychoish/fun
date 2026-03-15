package strut

import (
	"bytes"
	"fmt"
	"iter"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Buffer provides the same interface as Builder but wraps
// 'bytes.Buffer'.
type Buffer struct{ bytes.Buffer }

var bufferPool = sync.Pool{New: func() any { return new(Buffer) }}

// NewBuffer constructs a new 'strut.Buffer' using the provided 'buf'
// as the basis of the buffer.
func NewBuffer(buf []byte) *Buffer { var b Buffer; b.Write(buf); return &b }

// MakeBuffer retrieves a Buffer from the pool and ensures it has at
// least the specified capacity. The returned Buffer has zero length.
// Call Release() when done to return it to the pool for reuse.
func MakeBuffer(capacity int) *Buffer {
	b := bufferPool.Get().(*Buffer)
	b.Grow(capacity)
	return b
}

// Release resets the Buffer and returns it to the pool for reuse.
// Buffers larger than 64KB are not pooled to prevent excessive memory
// retention. After calling Release, the Buffer should not be used again.
func (b *Buffer) Release() {
	if b.Cap() > 64*1024 {
		return
	}
	b.Reset()
	bufferPool.Put(b)
}

func (b *Buffer) ws(s string)       { b.WriteString(s) }
func (b *Buffer) wb(in byte)        { b.WriteByte(in) }
func (b *Buffer) wrr(r rune)        { b.WriteRune(r) }
func (b *Buffer) cat(strs []string) { apply(b.ws, strs) }

// AppendPrint formats its arguments using default formatting and writes to
// the buffer. Analogous to fmt.Print, fmt.Fprint, and fmt.Sprint.
func (b *Buffer) AppendPrint(args ...any) *Buffer { fmt.Fprint(b, args...); return b }

// AppendPrintf formats according to a format specifier and writes to the
// buffer. The 'tpl' parameter is the format string, and 'args' are the
// values to format. Analgous to fmt.Printf, fmt.Sprintf, and
// fmt.Fprintf.
func (b *Buffer) AppendPrintf(tpl string, args ...any) *Buffer {
	fmt.Fprintf(b, tpl, args...)
	return b
}

// AppendPrintln formats its arguments using default formatting, adds a
// newline, and writes to the buffer. Analogous to fmt.Println,
// fmt.Sprintln, fmt.Fprintln.
func (b *Buffer) AppendPrintln(args ...any) *Buffer { fmt.Fprintln(b, args...); return b }

// Line writes a single newline character to the buffer.
func (b *Buffer) Line() { b.WriteByte('\n') }

// Tab writes a single tab character to the buffer.
func (b *Buffer) Tab() { b.WriteByte('\t') }

// NLines writes 'n' newline characters to the buffer.
// If 'n' is negative, the operaton is a no-op.
func (b *Buffer) NLines(n int) { ntimes(n, b.Line) }

// NTabs writes 'n' tabs characters to the buffer.
// If 'n' is negative, the operaton is a no-op.
func (b *Buffer) NTabs(n int) { ntimes(n, b.Tab) }

// WriteLine writes the string 'ln' followed by a newline character to
// the buffer.
func (b *Buffer) WriteLine(ln string) { b.WriteString(ln); b.Line() }

// WriteLines writes each string in 'lns' followed by a newline
// character to the buffer. Each string is written on its own line.
func (b *Buffer) WriteLines(lns ...string) { apply(b.WriteLine, lns) }

// WriteBytesLine writes the string 'ln' followed by a newline character to
// the buffer.
func (b *Buffer) WriteBytesLine(ln []byte) { b.Write(ln); b.Line() }

// WriteBytesLines writes each string in 'lns' followed by a newline
// character to the buffer. Each string is written on its own line.
func (b *Buffer) WriteBytesLines(lns ...[]byte) { apply(b.WriteBytesLine, lns) }

// WriteMutable writes the Mutable byte slice 'in' to the buffer.
func (b *Buffer) WriteMutable(in Mutable) { b.Write(in) }

// WriteMutableLine writes the Mutable byte slice 'in' followed by a newline
// character to the buffer.
func (b *Buffer) WriteMutableLine(in Mutable) { b.Write(in); b.Line() }

// WriteMutableLines writes each Mutable byte slice in 'in' followed by a
// newline character to the buffer. Each element is written on its own line.
func (b *Buffer) WriteMutableLines(in ...Mutable) { apply(b.WriteMutableLine, in) }

// Append writes the byte slice 'buf' to the buffer.
// This is a convenience wrapper around Write.
func (b *Buffer) Append(buf []byte) { b.Write(buf) }

// Join writes all strings from the slice 's', separated by 'sep', to the
// buffer. This is analogous to strings.Join but writes directly to
// the buffer.
func (b *Buffer) Join(s []string, sep string) { b.ExtendJoin(slices.Values(s), sep) }

// Concat writes all provided strings consecutively to the buffer
// without any separator.
func (b *Buffer) Concat(strs ...string) { b.cat(strs) }

// Repeat writes the string 'ln' to the buffer 'n' times.
// The 'n' parameter must be non-negative.
func (b *Buffer) Repeat(ln string, n int) { nwith(n, b.ws, ln) }

// RepeatByte writes the byte 'char' to the buffer 'n' times.
// The 'n' parameter must be non-negative.
func (b *Buffer) RepeatByte(char byte, n int) { nwith(n, b.wb, char) }

// RepeatRune writes the rune 'r' to the buffer 'n' times.
// The 'n' parameter must be non-negative.
func (b *Buffer) RepeatRune(r rune, n int) { nwith(n, b.wrr, r) }

// RepeatLine writes the string 'ln' followed by a newline to the
// buffer 'n' times. The 'n' parameter must be non-negative. Each
// repetition is on its own line.
func (b *Buffer) RepeatLine(ln string, n int) { nwith(n, b.WriteLine, ln) }

// WhenAppendPrint calls AppendPrint with 'args' if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenAppendPrint(cond bool, args ...any) *Buffer {
	if cond {
		b.AppendPrint(args...)
	}
	return b
}

// WhenAppendPrintf calls AppendPrintf with 'tpl' and 'args' if 'cond' is true and is
// a no-op otherwise.
func (b *Buffer) WhenAppendPrintf(cond bool, tpl string, args ...any) *Buffer {
	if cond {
		b.AppendPrintf(tpl, args...)
	}
	return b
}

// WhenAppendPrintln calls AppendPrintln with 'args' if 'cond' is true and is a
// no-op otherwise.
func (b *Buffer) WhenAppendPrintln(cond bool, args ...any) *Buffer {
	if cond {
		b.AppendPrintln(args...)
	}
	return b
}

// WhenLine writes a newline if 'cond' is true and is a no-op otherwise.
func (b *Buffer) WhenLine(cond bool) { ifop(cond, b.Line) }

// WhenTab writes a tab character if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenTab(cond bool) { ifop(cond, b.Tab) }

// WhenNLines writes 'n' newlines if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenNLines(cond bool, n int) { ifwith(cond, b.NLines, n) }

// WhenNTabs writes 'n' tabs if 'cond' is true and is a no-op otherwise.
func (b *Buffer) WhenNTabs(cond bool, n int) { ifwith(cond, b.NTabs, n) }

// WhenWrite writes the byte slice 'buf' if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenWrite(cond bool, buf []byte) { ifwith(cond, b.Append, buf) }

// WhenWriteString writes the string 's' if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenWriteString(cond bool, s string) { ifwith(cond, b.ws, s) }

// WhenWriteByte writes the byte 'bt' if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenWriteByte(cond bool, bt byte) { ifwith(cond, b.wb, bt) }

// WhenWriteRune writes the rune 'r' if 'cond' is true and is a no-op
// otherwise.
func (b *Buffer) WhenWriteRune(cond bool, r rune) { ifwith(cond, b.wrr, r) }

// WhenWriteLine writes the string 'ln' followed by a newline if 'cond' is
// true and is a no-op otherwise.
func (b *Buffer) WhenWriteLine(cond bool, ln string) { ifwith(cond, b.WriteLine, ln) }

// WhenWriteLines writes each string in 'lns' on its own line if 'cond' is
// true and is a no-op otherwise.
func (b *Buffer) WhenWriteLines(cond bool, lns ...string) { ifargs(cond, b.WriteLines, lns) }

// WhenConcat concatenates all strings in 'strs' if 'cond' is true and is
// a no-op otherwise.
func (b *Buffer) WhenConcat(cond bool, strs ...string) { ifwith(cond, b.cat, strs) }

// WhenWriteMutable writes the Mutable byte slice 'm' if 'cond' is true and is
// a no-op otherwise.
func (b *Buffer) WhenWriteMutable(cond bool, m Mutable) { ifwith(cond, b.WriteMutable, m) }

// WhenWriteMutableLine writes the Mutable byte slice 'm' followed by a newline
// if 'cond' is true and is a no-op otherwise.
func (b *Buffer) WhenWriteMutableLine(cond bool, m Mutable) { ifwith(cond, b.WriteMutableLine, m) }

// WhenWriteMutableLines writes each Mutable byte slice in 'ms' on its own line
// if 'cond' is true and is a no-op otherwise.
func (b *Buffer) WhenWriteMutableLines(cond bool, ms ...Mutable) {
	ifargs(cond, b.WriteMutableLines, ms)
}

// WhenJoin joins all strings from 'sl' with 'sep' if 'cond' is true and is
// a no-op otherwise.
func (b *Buffer) WhenJoin(cond bool, sl []string, sep string) { iftuple(cond, b.Join, sl, sep) }

// Quote writes a double-quoted Go string literal representing 'str' to
// the buffer. The output includes surrounding quotes and uses Go
// escape sequences.
func (b *Buffer) Quote(str string) { b.ws(strconv.Quote(str)) }

// QuoteASCII writes a double-quoted Go string literal representing
// 'str' to the buffer. Non-ASCII characters are escaped using \u or
// \U sequences.
func (b *Buffer) QuoteASCII(str string) { b.ws(strconv.QuoteToASCII(str)) }

// QuoteGrapic writes a double-quoted Go string literal representing
// 'str' to the buffer. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Buffer) QuoteGrapic(str string) { b.ws(strconv.QuoteToGraphic(str)) }

// QuoteRune writes a single-quoted Go character literal representing
// 'r' to the buffer. The output includes surrounding single quotes
// and uses Go escape sequences.
func (b *Buffer) QuoteRune(r rune) { b.ws(strconv.QuoteRune(r)) }

// QuoteRuneASCII writes a single-quoted Go character literal
// representing 'r' to the buffer. Non-ASCII characters are escaped
// using \u or \U sequences.
func (b *Buffer) QuoteRuneASCII(r rune) { b.ws(strconv.QuoteRuneToASCII(r)) }

// QuoteRuneGrapic writes a single-quoted Go character literal
// representing 'r' to the buffer. Non-graphic characters as defined
// by unicode.IsGraphic are escaped.
func (b *Buffer) QuoteRuneGrapic(r rune) { b.ws(strconv.QuoteRuneToGraphic(r)) }

// AppendQuote writes a double-quoted Go string literal representing 'str' to
// the buffer. The output includes surrounding quotes and uses Go
// escape sequences.
func (b *Buffer) AppendQuote(str string) { b.Write(strconv.AppendQuote(nil, str)) }

// AppendQuoteASCII writes a double-quoted Go string literal representing
// 'str' to the buffer. Non-ASCII characters are escaped using \u or
// \U sequences.
func (b *Buffer) AppendQuoteASCII(str string) { b.Write(strconv.AppendQuoteToASCII(nil, str)) }

// AppendQuoteGrapic writes a double-quoted Go string literal representing
// 'str' to the buffer. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Buffer) AppendQuoteGrapic(str string) { b.Write(strconv.AppendQuoteToGraphic(nil, str)) }

// AppendQuoteRune writes a single-quoted Go character literal representing
// 'r' to the buffer. The output includes surrounding single quotes
// and uses Go escape sequences.
func (b *Buffer) AppendQuoteRune(r rune) { b.Write(strconv.AppendQuoteRune(nil, r)) }

// AppendQuoteRuneASCII writes a single-quoted Go character literal
// representing 'r' to the buffer. Non-ASCII characters are escaped
// using \u or \U sequences.
func (b *Buffer) AppendQuoteRuneASCII(r rune) { b.Write(strconv.AppendQuoteRuneToASCII(nil, r)) }

// AppendQuoteRuneGrapic writes a single-quoted Go character literal
// representing 'r' to the buffer. Non-graphic characters as defined
// by unicode.IsGraphic are escaped.
func (b *Buffer) AppendQuoteRuneGrapic(r rune) { b.Write(strconv.AppendQuoteRuneToGraphic(nil, r)) }

// Int writes the decimal string representation of 'num' to the buffer.
func (b *Buffer) Int(num int) { b.Write(strconv.AppendInt(nil, int64(num), 10)) }

// AppendBool writes "true" or "false" according to the value of 'v' to
// the buffer.
func (b *Buffer) AppendBool(v bool) { b.Write(strconv.AppendBool(nil, v)) }

// AppendInt64 writes the string representation of 'n' in the given 'base'
// to the buffer. The 'base' must be between 2 and 36 inclusive.
func (b *Buffer) AppendInt64(n int64, base int) { b.Write(strconv.AppendInt(nil, n, base)) }

// AppendUint64 writes the string representation of 'n' in the given
// 'base' to the buffer. The 'base' must be between 2 and 36 inclusive.
func (b *Buffer) AppendUint64(n uint64, base int) { b.Write(strconv.AppendUint(nil, n, base)) }

// AppendFloat writes the string representation of the floating-point
// number 'f' to the buffer. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the number of bits (32 or 64).
func (b *Buffer) AppendFloat(f float64, tpl byte, prec, size int) {
	b.Write(strconv.AppendFloat(nil, f, tpl, prec, size))
}

// FormatBool writes "true" or "false" according to the value of 'v' to
// the buffer.
func (b *Buffer) FormatBool(v bool) { b.ws(strconv.FormatBool(v)) }

// FormatInt64 writes the string representation of 'n' in the given 'base'
// to the buffer. The 'base' must be between 2 and 36 inclusive.
func (b *Buffer) FormatInt64(n int64, base int) { b.ws(strconv.FormatInt(n, base)) }

// FormatUint64 writes the string representation of 'n' in the given
// 'base' to the buffer. The 'base' must be between 2 and 36 inclusive.
func (b *Buffer) FormatUint64(n uint64, base int) { b.ws(strconv.FormatUint(n, base)) }

// FormatFloat writes the string representation of the floating-point
// number 'f' to the buffer. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the number of bits (32 or 64).
func (b *Buffer) FormatFloat(f float64, tpl byte, prec, size int) {
	b.ws(strconv.FormatFloat(f, tpl, prec, size))
}

// FormatComplex writes the string representation of the complex
// number 'n' to the buffer. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the total number of bits (64 or 128).
func (b *Buffer) FormatComplex(n complex128, tpl byte, prec, size int) {
	b.ws(strconv.FormatComplex(n, tpl, prec, size))
}

// WithTrimSpace writes 'str' with all leading and trailing whitespace
// removed to the buffer.
func (b *Buffer) WithTrimSpace(str string) { b.ws(strings.TrimSpace(str)) }

// WithTrimRight writes 'str' with all trailing characters contained in
// 'cut' removed to the buffer.
func (b *Buffer) WithTrimRight(str string, cut string) { b.ws(strings.TrimRight(str, cut)) }

// WithTrimLeft writes 'str' with all leading characters contained in
// 'cut' removed to the buffer.
func (b *Buffer) WithTrimLeft(str string, cut string) { b.ws(strings.TrimLeft(str, cut)) }

// WithTrimPrefix writes 's' with the leading 'prefix' string removed to
// the buffer. If 's' doesn't start with 'prefix', 's' is written
// unchanged.
func (b *Buffer) WithTrimPrefix(s string, prefix string) { b.ws(strings.TrimPrefix(s, prefix)) }

// WithTrimSuffix writes 's' with the trailing 'suffix' string removed to
// the buffer. If 's' doesn't end with 'suffix', 's' is written unchanged.
func (b *Buffer) WithTrimSuffix(s string, suffix string) { b.ws(strings.TrimSuffix(s, suffix)) }

// WithReplaceAll writes 's' with all non-overlapping instances of 'old'
// replaced by 'new' to the buffer.
func (b *Buffer) WithReplaceAll(s, old, new string) { b.ws(strings.ReplaceAll(s, old, new)) } //nolint:predeclared

// WithReplace writes 's' with the first 'n' non-overlapping instances of
// 'old' replaced by 'new' to the buffer. If 'n' is negative, all
// instances are replaced.
func (b *Buffer) WithReplace(s, old, new string, n int) { b.ws(strings.Replace(s, old, new, n)) } //nolint:predeclared

// AppendTrimSpace writes 'str' with all leading and trailing whitespace
// removed to the buffer.
func (b *Buffer) AppendTrimSpace(str []byte) { b.Write(bytes.TrimSpace(str)) }

// AppendTrimRight writes 'str' with all trailing characters contained in
// 'cut' removed to the buffer.
func (b *Buffer) AppendTrimRight(str []byte, cut string) { b.Write(bytes.TrimRight(str, cut)) }

// AppendTrimLeft writes 'str' with all leading characters contained in
// 'cut' removed to the buffer.
func (b *Buffer) AppendTrimLeft(str []byte, cut string) { b.Write(bytes.TrimLeft(str, cut)) }

// AppendTrimPrefix writes 's' with the leading 'prefix' string removed to
// the buffer. If 's' doesn't start with 'prefix', 's' is written
// unchanged.
func (b *Buffer) AppendTrimPrefix(s []byte, prefix []byte) { b.Write(bytes.TrimPrefix(s, prefix)) }

// AppendTrimSuffix writes 's' with the trailing 'suffix' string removed to
// the buffer. If 's' doesn't end with 'suffix', 's' is written unchanged.
func (b *Buffer) AppendTrimSuffix(s []byte, suffix []byte) { b.Write(bytes.TrimSuffix(s, suffix)) }

// AppendReplaceAll writes 's' with all non-overlapping instances of 'old'
// replaced by 'new' to the buffer.
func (b *Buffer) AppendReplaceAll(s, old, new []byte) { b.Write(bytes.ReplaceAll(s, old, new)) } //nolint:predeclared

// AppendReplace writes 's' with the first 'n' non-overlapping instances of
// 'old' replaced by 'new' to the buffer. If 'n' is negative, all
// instances are replaced.
func (b *Buffer) AppendReplace(s, old, new []byte, n int) { b.Write(bytes.Replace(s, old, new, n)) } //nolint:predeclared

// Extend writes all strings from the iterator 'seq' consecutively to
// the buffer.
func (b *Buffer) Extend(seq iter.Seq[string]) { flush(seq, b.ws) }

// ExtendLines writes each string from the iterator 'seq' on its own
// line to the buffer. Each string is followed by a newline
// character.
func (b *Buffer) ExtendLines(seq iter.Seq[string]) { flush(seq, b.WriteLine) }

// ExtendBytes writes all strings from the iterator 'seq' consecutively to
// the buffer.
func (b *Buffer) ExtendBytes(seq iter.Seq[[]byte]) { flush(seq, b.Append) }

// ExtendBytesLines writes all strings from the iterator 'seq' consecutively to
// the buffer, interspersing a newline character.
func (b *Buffer) ExtendBytesLines(seq iter.Seq[[]byte]) { flush(seq, b.WriteBytesLine) }

// ExtendMutable writes all Mutable byte slices from the iterator 'seq'
// consecutively to the buffer.
func (b *Buffer) ExtendMutable(seq iter.Seq[Mutable]) { flush(seq, b.WriteMutable) }

// ExtendMutableLines writes each Mutable byte slice from the iterator 'seq'
// on its own line to the buffer. Each element is followed by a newline character.
func (b *Buffer) ExtendMutableLines(seq iter.Seq[Mutable]) { flush(seq, b.WriteMutableLine) }

// ExtendJoin writes all strings from the iterator 'seq' to the buffer,
// separated by 'sep'. The first string is not preceded by a separator.
func (b *Buffer) ExtendJoin(seq iter.Seq[string], sep string) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.WriteString(sep)
		}
		ct++
		b.WriteString(elem)
	}
}

// ExtendBytesJoin writes all strings from the iterator 'seq' to the buffer,
// separated by 'sep'. The first string is not preceded by a separator.
func (b *Buffer) ExtendBytesJoin(seq iter.Seq[[]byte], sep []byte) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.Write(sep)
		}
		ct++
		b.Write(elem)
	}
}

// ExtendMutableJoin writes all Mutable byte slices from the iterator 'seq' to
// the buffer, separated by 'sep'. The first element is not preceded by a separator.
func (b *Buffer) ExtendMutableJoin(seq iter.Seq[Mutable], sep Mutable) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.Write(sep)
		}
		ct++
		b.Write(elem)
	}
}

// Format implements fmt.Formatter, writing the buffer's contents directly
// to the formatter state without allocating an intermediate string.
func (b *Buffer) Format(state fmt.State, _ rune) { _, _ = state.Write(b.Bytes()) }

// Print writes the buffer's contents to standard output.
func (b *Buffer) Print() { _, _ = os.Stdout.Write(b.Bytes()) }

// Println writes the buffer's contents to standard output followed by a newline.
func (b *Buffer) Println() { b.Print(); _, _ = os.Stdout.Write(newline) }

// Mutable returns the buffer's contents as a Mutable byte slice.
func (b *Buffer) Mutable() Mutable { return Mutable(b.Bytes()) }
