// Package strut provides high-level, ergonomic str(ing) ut(lites).
package strut

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Builder is a wrapper around strings.Builder, that provides additional
// higher-level methods for building strings.
type Builder struct{ strings.Builder }

// MakeBuilder constructs a new Builder with at least the specified capacity
// preallocated, avoiding early reallocation for known-size outputs.
func MakeBuilder(capacity int) *Builder { var b Builder; b.Grow(capacity); return &b }

func (b *Builder) wb(in byte)        { b.WriteByte(in) }
func (b *Builder) wrr(r rune)        { b.WriteRune(r) }
func (b *Builder) cat(strs []string) { apply(b.PushString, strs) }

// AppendPrint formats its arguments using default formatting and writes to
// the builder. Analogous to fmt.Print, fmt.Fprint, and fmt.Sprint.
func (b *Builder) AppendPrint(args ...any) *Builder { fmt.Fprint(b, args...); return b }

// AppendPrintf formats according to a format specifier and writes to the
// builder. The 'tpl' parameter is the format string, and 'args' are the
// values to format. Analgous to fmt.Printf, fmt.Sprintf, and
// fmt.Fprintf.
func (b *Builder) AppendPrintf(tpl string, args ...any) *Builder {
	fmt.Fprintf(b, tpl, args...)
	return b
}

// AppendPrintln formats its arguments using default formatting, adds a
// newline, and writes to the builder. Analogous to fmt.Println,
// fmt.Sprintln, fmt.Fprintln.
func (b *Builder) AppendPrintln(args ...any) *Builder { fmt.Fprintln(b, args...); return b }

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

// WriteMutableLine writes the Mutable byte slice 'in' followed by a newline
// character to the builder.
func (b *Builder) WriteMutableLine(in Mutable) { b.Write(in); b.Line() }

// WriteMutableLines writes each Mutable byte slice in 'in' followed by a
// newline character to the builder. Each element is written on its own line.
func (b *Builder) WriteMutableLines(in ...Mutable) { apply(b.WriteMutableLine, in) }

// WriteLine writes the string 'ln' followed by a newline character to
// the builder.
func (b *Builder) WriteLine(ln string) { b.WriteString(ln); b.Line() }

// WriteLines writes each string in 'lns' followed by a newline
// character to the builder. Each string is written on its own line.
func (b *Builder) WriteLines(lns ...string) { apply(b.WriteLine, lns) }

// WriteBytesLine writes the byte slice 'ln' followed by a newline character
// to the builder. The byte slice is copied during the write operation.
func (b *Builder) WriteBytesLine(ln []byte) { b.PushBytes(ln); b.Line() }

// WriteBytesLines writes each byte slice in 'lns' followed by a newline
// character to the builder. Each byte slice is written on its own line.
// Each byte slice is copied during the write operation.
func (b *Builder) WriteBytesLines(lns ...[]byte) { apply(b.WriteBytesLine, lns) }

// WhenWriteMutable writes the Mutable byte slice 'm' if 'cond' is true and is
// a no-op otherwise.
func (b *Builder) WhenWriteMutable(cond bool, m Mutable) { ifwith(cond, b.WriteMutable, m) }

// WhenWriteMutableLine writes the Mutable byte slice 'm' followed by a newline
// if 'cond' is true and is a no-op otherwise.
func (b *Builder) WhenWriteMutableLine(cond bool, m Mutable) { ifwith(cond, b.WriteMutableLine, m) }

// WhenWriteMutableLines writes each Mutable byte slice in 'ms' on its own line
// if 'cond' is true and is a no-op otherwise.
func (b *Builder) WhenWriteMutableLines(cond bool, ms ...Mutable) {
	ifargs(cond, b.WriteMutableLines, ms)
}

// Push writes the byte slice 'buf' to the builder. The byte slice is
// copied during the write operation; the caller retains ownership of 'buf'
// and may modify it after this call returns. This is a convenience wrapper
// around Write.
func (b *Builder) PushBytes(buf []byte) { b.Write(buf) }

// PushString appends string 's' to the builder.
func (b *Builder) PushString(s string) { b.WriteString(s) }

// WriteMutable writes the Mutable byte slice 'in' to the builder.
func (b *Builder) WriteMutable(in Mutable) { b.Write(in) }

// Join writes all strings from the slice 's', separated by 'sep', to the
// builder. This is analogous to strings.Join but writes directly to
// the builder.
func (b *Builder) Join(s []string, sep string) { b.ExtendJoin(slices.Values(s), sep) }

// Concat writes all provided strings consecutively to the builder
// without any separator.
func (b *Builder) Concat(strs ...string) { b.cat(strs) }

// Repeat writes the string 'ln' to the builder 'n' times.
// The 'n' parameter must be non-negative.
func (b *Builder) Repeat(ln string, n int) { nwith(n, b.PushString, ln) }

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

// WhenAppendPrint calls AppendPrint with 'args' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenAppendPrint(cond bool, args ...any) *Builder {
	if cond {
		b.AppendPrint(args...)
	}
	return b
}

// WhenAppendPrintf calls AppendPrintf with 'tpl' and 'args' if 'cond' is true and is
// a no-op otherwise.
func (b *Builder) WhenAppendPrintf(cond bool, tpl string, args ...any) *Builder {
	if cond {
		b.AppendPrintf(tpl, args...)
	}
	return b
}

// WhenAppendPrintln calls AppendPrintln with 'args' if 'cond' is true and is a
// no-op otherwise.
func (b *Builder) WhenAppendPrintln(cond bool, args ...any) *Builder {
	if cond {
		b.AppendPrintln(args...)
	}
	return b
}

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
func (b *Builder) WhenWrite(cond bool, buf []byte) { ifwith(cond, b.PushBytes, buf) }

// WhenWriteString writes the string 's' if 'cond' is true and is a no-op
// otherwise.
func (b *Builder) WhenWriteString(cond bool, s string) { ifwith(cond, b.PushString, s) }

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

// PushQuote writes a double-quoted Go string literal representing 'str' to
// the builder. The output includes surrounding quotes and uses Go escape
// sequences.
func (b *Builder) PushQuote(str string) { b.Write(strconv.AppendQuote(nil, str)) }

// PushQuoteASCII writes a double-quoted Go string literal representing
// 'str' to the builder. Non-ASCII characters are escaped using \u or \U
// sequences.
func (b *Builder) PushQuoteASCII(str string) { b.Write(strconv.AppendQuoteToASCII(nil, str)) }

// PushQuoteGrapic writes a double-quoted Go string literal representing
// 'str' to the builder. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Builder) PushQuoteGrapic(str string) { b.Write(strconv.AppendQuoteToGraphic(nil, str)) }

// PushQuoteRune writes a single-quoted Go character literal representing
// 'r' to the builder. The output includes surrounding single quotes and uses
// Go escape sequences.
func (b *Builder) PushQuoteRune(r rune) { b.Write(strconv.AppendQuoteRune(nil, r)) }

// PushQuoteRuneASCII writes a single-quoted Go character literal
// representing 'r' to the builder. Non-ASCII characters are escaped using \u
// or \U sequences.
func (b *Builder) PushQuoteRuneASCII(r rune) { b.Write(strconv.AppendQuoteRuneToASCII(nil, r)) }

// PushQuoteRuneGrapic writes a single-quoted Go character literal
// representing 'r' to the builder. Non-graphic characters as defined by
// unicode.IsGraphic are escaped.
func (b *Builder) PushQuoteRuneGrapic(r rune) { b.Write(strconv.AppendQuoteRuneToGraphic(nil, r)) }

// Int writes the  string representation of the integer 'num' to the builder.
func (b *Builder) PushInt(num int) { b.Write(strconv.AppendInt(nil, int64(num), 10)) }

// PushBool writes "true" or "false" according to the value of 'v' to the builder.
// PushInt writes the decimal string representation of 'num' to the builder.
func (b *Builder) PushBool(v bool) { b.Write(strconv.AppendBool(nil, v)) }

// PushInt64 writes the string representation of 'n' in the given 'base' to
// the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) PushInt64(n int64, base int) { b.Write(strconv.AppendInt(nil, n, base)) }

// PushUint64 writes the string representation of 'n' in the given 'base'
// to the builder. The 'base' must be between 2 and 36 inclusive.
func (b *Builder) PushUint64(n uint64, base int) { b.Write(strconv.AppendUint(nil, n, base)) }

// PushFloat writes the string representation of the floating-point number
// 'f' to the builder. The 'tpl' parameter is the format ('b', 'e', 'E', 'f',
// 'g', 'G', 'x', 'X'), 'prec' controls precision, and 'size' is the number of
// bits (32 or 64).
func (b *Builder) PushFloat(f float64, tpl byte, prec, size int) {
	b.Write(strconv.AppendFloat(nil, f, tpl, prec, size))
}

// PushComplex writes the string representation of the complex
// number 'n' to the builder. The 'tpl' parameter is the format ('b',
// 'e', 'E', 'f', 'g', 'G', 'x', 'X'), 'prec' controls precision, and
// 'size' is the total number of bits (64 or 128).
func (b *Builder) PushComplex(n complex128, tpl byte, prec, size int) {
	b.PushString(strconv.FormatComplex(n, tpl, prec, size))
}

// WithTrimSpace writes 'str' with all leading and trailing whitespace
// removed to the builder.
func (b *Builder) WithTrimSpace(str string) { b.PushString(strings.TrimSpace(str)) }

// WithTrimRight writes 'str' with all trailing characters contained in
// 'cut' removed to the builder.
func (b *Builder) WithTrimRight(str string, cut string) { b.PushString(strings.TrimRight(str, cut)) }

// WithTrimLeft writes 'str' with all leading characters contained in
// 'cut' removed to the builder.
func (b *Builder) WithTrimLeft(str string, cut string) { b.PushString(strings.TrimLeft(str, cut)) }

// WithTrimPrefix writes 's' with the leading 'prefix' string removed to
// the builder. If 's' doesn't start with 'prefix', 's' is written
// unchanged.
func (b *Builder) WithTrimPrefix(s string, prefix string) {
	b.PushString(strings.TrimPrefix(s, prefix))
}

// WithTrimSuffix writes 's' with the trailing 'suffix' string removed to
// the builder. If 's' doesn't end with 'suffix', 's' is written unchanged.
func (b *Builder) WithTrimSuffix(s string, suffix string) {
	b.PushString(strings.TrimSuffix(s, suffix))
}

// WithReplaceAll writes 's' with all non-overlapping instances of 'old'
// replaced by 'new' to the builder.
func (b *Builder) WithReplaceAll(s, old, new string) { b.PushString(strings.ReplaceAll(s, old, new)) } //nolint:predeclared

// WithReplace writes 's' with the first 'n' non-overlapping instances of
// 'old' replaced by 'new' to the builder. If 'n' is negative, all
// instances are replaced.
func (b *Builder) WithReplace(s, old, new string, n int) { //nolint:predeclared
	b.PushString(strings.Replace(s, old, new, n))
}

// AppendTrimSpace writes the byte slice 'str' with all leading and trailing
// whitespace removed to the builder. The input 'str' is not modified; a
// transformed copy is written. This is the byte slice equivalent of
// WithTrimSpace.
func (b *Builder) AppendTrimSpace(str []byte) { b.Write(bytes.TrimSpace(str)) }

// AppendTrimRight writes the byte slice 'str' with all trailing characters
// contained in 'cut' removed to the builder. The input 'str' is not modified;
// a transformed copy is written. This is the byte slice equivalent of
// WithTrimRight.
func (b *Builder) AppendTrimRight(str []byte, cut string) { b.Write(bytes.TrimRight(str, cut)) }

// AppendTrimLeft writes the byte slice 'str' with all leading characters
// contained in 'cut' removed to the builder. The input 'str' is not modified;
// a transformed copy is written. This is the byte slice equivalent of
// WithTrimLeft.
func (b *Builder) AppendTrimLeft(str []byte, cut string) { b.Write(bytes.TrimLeft(str, cut)) }

// AppendTrimPrefix writes the byte slice 's' with the leading 'prefix'
// removed to the builder. If 's' doesn't start with 'prefix', 's' is written
// unchanged. The input 's' is not modified; a transformed copy is written.
// This is the byte slice equivalent of WithTrimPrefix.
func (b *Builder) AppendTrimPrefix(s []byte, prefix []byte) { b.Write(bytes.TrimPrefix(s, prefix)) }

// AppendTrimSuffix writes the byte slice 's' with the trailing 'suffix'
// removed to the builder. If 's' doesn't end with 'suffix', 's' is written
// unchanged. The input 's' is not modified; a transformed copy is written.
// This is the byte slice equivalent of WithTrimSuffix.
func (b *Builder) AppendTrimSuffix(s []byte, suffix []byte) { b.Write(bytes.TrimSuffix(s, suffix)) }

// AppendReplaceAll writes the byte slice 's' with all non-overlapping
// instances of 'old' replaced by 'new' to the builder. The input 's' is not
// modified; a transformed copy is written. This is the byte slice equivalent
// of WithReplaceAll.
func (b *Builder) AppendReplaceAll(s, old, new []byte) { b.Write(bytes.ReplaceAll(s, old, new)) } //nolint:predeclared

// AppendReplace writes the byte slice 's' with the first 'n' non-overlapping
// instances of 'old' replaced by 'new' to the builder. If 'n' is negative,
// all instances are replaced. The input 's' is not modified; a transformed
// copy is written. This is the byte slice equivalent of WithReplace.
func (b *Builder) AppendReplace(s, old, new []byte, n int) { b.Write(bytes.Replace(s, old, new, n)) } //nolint:predeclared

// Extend writes all strings from the iterator 'seq' consecutively to
// the builder.
func (b *Builder) Extend(seq iter.Seq[string]) { flush(seq, b.PushString) }

// ExtendLines writes each string from the iterator 'seq' on its own line to
// the builder. Each string is followed by a newline character.
func (b *Builder) ExtendLines(seq iter.Seq[string]) { flush(seq, b.WriteLine) }

// ExtendMutable writes all Mutable byte slices from the iterator 'seq'
// consecutively to the builder.
func (b *Builder) ExtendMutable(seq iter.Seq[Mutable]) { flush(seq, b.WriteMutable) }

// ExtendMutableLines writes each Mutable byte slice from the iterator 'seq'
// on its own line to the builder. Each element is followed by a newline character.
func (b *Builder) ExtendMutableLines(seq iter.Seq[Mutable]) { flush(seq, b.WriteMutableLine) }

// ExtendBytes writes all byte slices from the iterator 'seq' consecutively
// to the builder. Each byte slice is copied during the write operation. This
// is the byte slice equivalent of Extend.
func (b *Builder) ExtendBytes(seq iter.Seq[[]byte]) { flush(seq, b.PushBytes) }

// ExtendBytesLines writes all byte slices from the iterator 'seq' to the
// builder, each followed by a newline character. Each byte slice is written
// on its own line. Each byte slice is copied during the write operation.
// This is the byte slice equivalent of ExtendLines.
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

// ExtendBytesJoin writes all byte slices from the iterator 'seq' to the
// builder, separated by 'sep'. The first byte slice is not preceded by a
// separator. Each byte slice is copied during the write operation. This is
// the byte slice equivalent of ExtendJoin.
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

// ExtendMutableJoin writes all Mutable byte slices from the iterator 'seq' to
// the builder, separated by 'sep'. The first element is not preceded by a separator.
func (b *Builder) ExtendMutableJoin(seq iter.Seq[Mutable], sep Mutable) {
	var ct int
	for elem := range seq {
		if ct != 0 {
			b.Write(sep)
		}
		ct++
		b.Write(elem)
	}
}

// Bytes returns the builder's accumulated content as a byte slice.
func (b *Builder) Bytes() []byte { return []byte(b.Builder.String()) }

// Format implements fmt.Formatter, writing the builder's contents directly
// to the formatter state without allocating an intermediate string.
func (b *Builder) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, b.String()) }

// Print writes the builder's contents to standard output.
func (b *Builder) Print() { _, _ = io.WriteString(os.Stdout, b.String()) }

// Println writes the builder's contents to standard output followed by a newline.
func (b *Builder) Println() { b.Print(); _, _ = os.Stdout.Write(newline) }

// Mutable returns the builder's contents as a Mutable byte slice.
func (b *Builder) Mutable() Mutable { return Mutable(b.Bytes()) }
