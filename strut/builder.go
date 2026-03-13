// Package strut provides high-level, ergonomic str(ing) ut(lites).
package strut

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
)

// Builder is a wrapper around strings.Builder, that provides additional
// higher-level methods for building strings.
type Builder struct{ strings.Builder }

func (b *Builder) ws(s string)       { b.WriteString(s) }
func (b *Builder) wb(in byte)        { b.WriteByte(in) }
func (b *Builder) wrr(r rune)        { b.WriteRune(r) }
func (b *Builder) cat(strs []string) { apply(b.ws, strs) }

// Wprint formats its arguments using default formatting and writes to
// the builder. Analogous to fmt.Print, fmt.Fprint, and fmt.Sprint.
func (b *Builder) Wprint(args ...any) { fmt.Fprint(b, args...) }

// Wprintf formats according to a format specifier and writes to the
// builder. The 'tpl' parameter is the format string, and 'args' are the
// values to format. Analgous to fmt.Printf, fmt.Sprintf, and
// fmt.Fprintf.
func (b *Builder) Wprintf(tpl string, args ...any) { fmt.Fprintf(b, tpl, args...) }

// Wprintln formats its arguments using default formatting, adds a
// newline, and writes to the builder. Analogous to fmt.Println,
// fmt.Sprintln, fmt.Fprintln.
func (b *Builder) Wprintln(args ...any) { fmt.Fprintln(b, args...) }

// Line writes a single newline character to the builder.
func (b *Builder) Line() { b.WriteByte('\n') }

// Tab writes a single tab character to the builder.
func (b *Builder) Tab() { b.WriteByte('\t') }

// NLines writes 'n' newline characters to the builder.
// If 'n' is negative, the operaton is a no-op.
func (b *Builder) NLines(n int) { ntimes(n, b.Line) }

// NTabs writes 'n' tabs characters to the builder.
// If 'n' is negative, the operaton is a no-op.
func (b *Builder) NTabs(n int) { ntimes(n, b.Tab) }

// WriteLine writes the string 'ln' followed by a newline character to
// the builder.
func (b *Builder) WriteLine(ln string) { b.WriteString(ln); b.Line() }

// WriteLines writes each string in 'lns' followed by a newline
// character to the builder. Each string is written on its own line.
func (b *Builder) WriteLines(lns ...string) { apply(b.WriteLine, lns) }

// WriteBytesLine writes the string 'ln' followed by a newline character to
// the builder.
func (b *Builder) WriteBytesLine(ln []byte) { b.Append(ln); b.Line() }

// WriteBytesLines writes each string in 'lns' followed by a newline
// character to the builder. Each string is written on its own line.
func (b *Builder) WriteBytesLines(lns ...[]byte) { apply(b.WriteBytesLine, lns) }

// Append writes the byte slice 'buf' to the builder.
// This is a convenience wrapper around Write.
func (b *Builder) Append(buf []byte) { b.Write(buf) }

// Join writes all strings from the slice 's', separated by 'sep', to the
// builder. This is analogous to strings.Join but writes directly to
// the builder.
func (b *Builder) Join(s []string, sep string) { b.ExtendJoin(slices.Values(s), sep) }

// Concat writes all provided strings consecutively to the builder
// without any separator.
func (b *Builder) Concat(strs ...string) { b.cat(strs) }

// Repeat writes the string 'ln' to the builder 'n' times.
// The 'n' parameter must be non-negative.
func (b *Builder) Repeat(ln string, n int) { nwith(n, b.ws, ln) }

// RepeatByte writes the byte 'char' to the builder 'n' times.
// The 'n' parameter must be non-negative.
func (b *Builder) RepeatByte(char byte, n int) { nwith(n, b.wb, char) }

// RepeatRune writes the rune 'r' to the builder 'n' times.
// The 'n' parameter must be non-negative.
func (b *Builder) RepeatRune(r rune, n int) { nwith(n, b.wrr, r) }

// RepeatLine writes the string 'ln' followed by a newline to the
// builder 'n' times. The 'n' parameter must be non-negative. Each
// repetition is on its own line.
func (b *Builder) RepeatLine(ln string, n int) { nwith(n, b.WriteLine, ln) }

// WhenWprint calls Wprint with 'args' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWprint(cond bool, args ...any) { ifargs(cond, b.Wprint, args) }

// WhenWprintf calls Wprintf with 'tpl' and 'args' if 'cond' is true and is
// a no-op otherwise.
func (b *Builder) WhenWprintf(cond bool, tpl string, args ...any) { iffmt(cond, b.Wprintf, tpl, args) }

// WhenWprintln calls Wprintln with 'args' if 'cond' is true and is a
// no-op otherwise.
func (b *Builder) WhenWprintln(cond bool, args ...any) { ifargs(cond, b.Wprintln, args) }

// WhenLine writes a newline if 'cond' is true and is a no-op otherwise.
func (b *Builder) WhenLine(cond bool) { ifop(cond, b.Line) }

// WhenTab writes a tab character if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenTab(cond bool) { ifop(cond, b.Tab) }

// WhenNLines writes 'n' newlines if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenNLines(cond bool, n int) { ifwith(cond, b.NLines, n) }

// WhenNTabs writes 'n' tabs if 'cond' is true and is a no-op otherwise.
func (b *Builder) WhenNTabs(cond bool, n int) { ifwith(cond, b.NTabs, n) }

// WhenWrite writes the byte slice 'buf' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWrite(cond bool, buf []byte) { ifwith(cond, b.Append, buf) }

// WhenWriteString writes the string 's' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWriteString(cond bool, s string) { ifwith(cond, b.ws, s) }

// WhenWriteByte writes the byte 'bt' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWriteByte(cond bool, bt byte) { ifwith(cond, b.wb, bt) }

// WhenWriteRune writes the rune 'r' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWriteRune(cond bool, r rune) { ifwith(cond, b.wrr, r) }

// WhenWriteLine writes the string 'ln' followed by a newline if 'cond' is
// true and is a no-op otherwise.
func (b *Builder) WhenWriteLine(cond bool, ln string) { ifwith(cond, b.WriteLine, ln) }

// WhenWriteLines writes each string in 'lns' on its own line if 'cond' is
// true and is a no-op otherwise.
func (b *Builder) WhenWriteLines(cond bool, lns ...string) { ifargs(cond, b.WriteLines, lns) }

// WhenConcat concatenates all strings in 'strs' if 'cond' is true and is
// a no-op otherwise.
func (b *Builder) WhenConcat(cond bool, strs ...string) { ifwith(cond, b.cat, strs) }

// WhenJoin joins all strings from 'sl' with 'sep' if 'cond' is true and is
// a no-op otherwise.
func (b *Builder) WhenJoin(cond bool, sl []string, sep string) { iftuple(cond, b.Join, sl, sep) }

// AppendQuote writes a double-quoted Go string literal representing 'str' to
// the builder. The output includes surrounding quotes and uses Go
// escape sequences.
func (b *Builder) AppendQuote(str string) { b.Write(strconv.AppendQuote(nil, str)) }

// AppendQuoteASCII writes a double-quoted Go string literal representing
// 'str' to the builder. Non-ASCII characters are escaped using \u or
// \U sequences.
func (b *Builder) AppendQuoteASCII(str string) { b.Write(strconv.AppendQuoteToASCII(nil, str)) }

// AppendQuoteGrapic writes a double-quoted Go string literal representing
// 'str' to the builder. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Builder) AppendQuoteGrapic(str string) { b.Write(strconv.AppendQuoteToGraphic(nil, str)) }

// AppendQuoteRune writes a single-quoted Go character literal representing
// 'r' to the builder. The output includes surrounding single quotes
// and uses Go escape sequences.
func (b *Builder) AppendQuoteRune(r rune) { b.Write(strconv.AppendQuoteRune(nil, r)) }

// AppendQuoteRuneASCII writes a single-quoted Go character literal
// representing 'r' to the builder. Non-ASCII characters are escaped
// using \u or \U sequences.
func (b *Builder) AppendQuoteRuneASCII(r rune) { b.Write(strconv.AppendQuoteRuneToASCII(nil, r)) }

// AppendQuoteRuneGrapic writes a single-quoted Go character literal
// representing 'r' to the builder. Non-graphic characters as defined
// by unicode.IsGraphic are escaped.
func (b *Builder) AppendQuoteRuneGrapic(r rune) { b.Write(strconv.AppendQuoteRuneToGraphic(nil, r)) }

// Quote writes a double-quoted Go string literal representing 'str' to
// the builder. The output includes surrounding quotes and uses Go
// escape sequences.
func (b *Builder) Quote(str string) { b.ws(strconv.Quote(str)) }

// QuoteASCII writes a double-quoted Go string literal representing
// 'str' to the builder. Non-ASCII characters are escaped using \u or
// \U sequences.
func (b *Builder) QuoteASCII(str string) { b.ws(strconv.QuoteToASCII(str)) }

// QuoteGrapic writes a double-quoted Go string literal representing
// 'str' to the builder. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Builder) QuoteGrapic(str string) { b.ws(strconv.QuoteToGraphic(str)) }

// QuoteRune writes a single-quoted Go character literal representing
// 'r' to the builder. The output includes surrounding single quotes
// and uses Go escape sequences.
func (b *Builder) QuoteRune(r rune) { b.ws(strconv.QuoteRune(r)) }

// QuoteRuneASCII writes a single-quoted Go character literal
// representing 'r' to the builder. Non-ASCII characters are escaped
// using \u or \U sequences.
func (b *Builder) QuoteRuneASCII(r rune) { b.ws(strconv.QuoteRuneToASCII(r)) }

// QuoteRuneGrapic writes a single-quoted Go character literal
// representing 'r' to the builder. Non-graphic characters as defined
// by unicode.IsGraphic are escaped.
func (b *Builder) QuoteRuneGrapic(r rune) { b.ws(strconv.QuoteRuneToGraphic(r)) }

// Int writes the decimal string representation of 'num' to the builder.
func (b *Builder) Int(num int) { b.ws(strconv.Itoa(num)) }

// AppendBool writes "true" or "false" according to the value of 'v' to
// the builder.
func (b *Builder) AppendBool(v bool) { b.Write(strconv.AppendBool(nil, v)) }

// AppendInt64 writes the string representation of 'n' in the given 'base'
// to the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) AppendInt64(n int64, base int) { b.Write(strconv.AppendInt(nil, n, base)) }

// AppendUint64 writes the string representation of 'n' in the given
// 'base' to the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) AppendUint64(n uint64, base int) { b.Write(strconv.AppendUint(nil, n, base)) }

// AppendFloat writes the string representation of the floating-point
// number 'f' to the builder. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the number of bits (32 or 64).
func (b *Builder) AppendFloat(f float64, tpl byte, prec, size int) {
	b.Write(strconv.AppendFloat(nil, f, tpl, prec, size))
}

// FormatBool writes "true" or "false" according to the value of 'v' to
// the builder.
func (b *Builder) FormatBool(v bool) { b.ws(strconv.FormatBool(v)) }

// FormatInt64 writes the string representation of 'n' in the given 'base'
// to the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) FormatInt64(n int64, base int) { b.ws(strconv.FormatInt(n, base)) }

// FormatUint64 writes the string representation of 'n' in the given
// 'base' to the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) FormatUint64(n uint64, base int) { b.ws(strconv.FormatUint(n, base)) }

// FormatFloat writes the string representation of the floating-point
// number 'f' to the builder. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the number of bits (32 or 64).
func (b *Builder) FormatFloat(f float64, tpl byte, prec, size int) {
	b.ws(strconv.FormatFloat(f, tpl, prec, size))
}

// FormatComplex writes the string representation of the complex
// number 'n' to the builder. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the total number of bits (64 or 128).
func (b *Builder) FormatComplex(n complex128, tpl byte, prec, size int) {
	b.ws(strconv.FormatComplex(n, tpl, prec, size))
}

// WithTrimSpace writes 'str' with all leading and trailing whitespace
// removed to the builder.
func (b *Builder) WithTrimSpace(str string) { b.ws(strings.TrimSpace(str)) }

// WithTrimRight writes 'str' with all trailing characters contained in
// 'cut' removed to the builder.
func (b *Builder) WithTrimRight(str string, cut string) { b.ws(strings.TrimRight(str, cut)) }

// WithTrimLeft writes 'str' with all leading characters contained in
// 'cut' removed to the builder.
func (b *Builder) WithTrimLeft(str string, cut string) { b.ws(strings.TrimLeft(str, cut)) }

// WithTrimPrefix writes 's' with the leading 'prefix' string removed to
// the builder. If 's' doesn't start with 'prefix', 's' is written
// unchanged.
func (b *Builder) WithTrimPrefix(s string, prefix string) { b.ws(strings.TrimPrefix(s, prefix)) }

// WithTrimSuffix writes 's' with the trailing 'suffix' string removed to
// the builder. If 's' doesn't end with 'suffix', 's' is written unchanged.
func (b *Builder) WithTrimSuffix(s string, suffix string) { b.ws(strings.TrimSuffix(s, suffix)) }

// WithReplaceAll writes 's' with all non-overlapping instances of 'old'
// replaced by 'new' to the builder.
func (b *Builder) WithReplaceAll(s, old, new string) { b.ws(strings.ReplaceAll(s, old, new)) } //nolint:predeclared

// WithReplace writes 's' with the first 'n' non-overlapping instances of
// 'old' replaced by 'new' to the builder. If 'n' is negative, all
// instances are replaced.
func (b *Builder) WithReplace(s, old, new string, n int) { b.ws(strings.Replace(s, old, new, n)) } //nolint:predeclared

// AppendTrimSpace writes 'str' with all leading and trailing whitespace
// removed to the builder.
func (b *Builder) AppendTrimSpace(str []byte) { b.Write(bytes.TrimSpace(str)) }

// AppendTrimRight writes 'str' with all trailing characters contained in
// 'cut' removed to the builder.
func (b *Builder) AppendTrimRight(str []byte, cut string) { b.Write(bytes.TrimRight(str, cut)) }

// AppendTrimLeft writes 'str' with all leading characters contained in
// 'cut' removed to the builder.
func (b *Builder) AppendTrimLeft(str []byte, cut string) { b.Write(bytes.TrimLeft(str, cut)) }

// AppendTrimPrefix writes 's' with the leading 'prefix' string removed to
// the builder. If 's' doesn't start with 'prefix', 's' is written
// unchanged.
func (b *Builder) AppendTrimPrefix(s []byte, prefix []byte) { b.Write(bytes.TrimPrefix(s, prefix)) }

// AppendTrimSuffix writes 's' with the trailing 'suffix' string removed to
// the builder. If 's' doesn't end with 'suffix', 's' is written unchanged.
func (b *Builder) AppendTrimSuffix(s []byte, suffix []byte) { b.Write(bytes.TrimSuffix(s, suffix)) }

// AppendReplaceAll writes 's' with all non-overlapping instances of 'old'
// replaced by 'new' to the builder.
func (b *Builder) AppendReplaceAll(s, old, new []byte) { b.Write(bytes.ReplaceAll(s, old, new)) } //nolint:predeclared

// AppendReplace writes 's' with the first 'n' non-overlapping instances of
// 'old' replaced by 'new' to the builder. If 'n' is negative, all
// instances are replaced.
func (b *Builder) AppendReplace(s, old, new []byte, n int) { b.Write(bytes.Replace(s, old, new, n)) } //nolint:predeclared

// Extend writes all strings from the iterator 'seq' consecutively to
// the builder.
func (b *Builder) Extend(seq iter.Seq[string]) { flush(seq, b.ws) }

// ExtendBytes writes all strings from the iterator 'seq' consecutively to
// the builder.
func (b *Builder) ExtendBytes(seq iter.Seq[[]byte]) { flush(seq, b.Append) }

// ExtendLines writes each string from the iterator 'seq' on its own
// line to the builder. Each string is followed by a newline
// character.
func (b *Builder) ExtendLines(seq iter.Seq[string]) { flush(seq, b.WriteLine) }

// ExtendBytesLines writes all strings from the iterator 'seq' consecutively to
// the builder, interspersing a newline character.
func (b *Builder) ExtendBytesLines(seq iter.Seq[[]byte]) { flush(seq, b.WriteBytesLine) }

// ExtendJoin writes all strings from the iterator 'seq' to the builder,
// separated by 'sep'. The first string is not preceded by a separator.
func (b *Builder) ExtendJoin(seq iter.Seq[string], sep string) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.WriteString(sep)
		}
		ct++
		b.WriteString(elem)
	}
}

// ExtendBytesJoin writes all strings from the iterator 'seq' to the builder,
// separated by 'sep'. The first string is not preceded by a separator.
func (b *Builder) ExtendBytesJoin(seq iter.Seq[[]byte], sep []byte) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.Write(sep)
		}
		ct++
		b.Write(elem)
	}
}
