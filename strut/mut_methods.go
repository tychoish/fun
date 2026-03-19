package strut

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
)

// cat is a private helper for Concat/WhenConcat.
func (mut *Mutable) cat(strs []string) { apply(mut.PushString, strs) }

func (mut *Mutable) wb(b byte)  { *mut = append(*mut, b) }
func (mut *Mutable) wrr(r rune) { _, _ = mut.WriteRune(r) }

// ---- Line / Tab ----

// Line writes a single newline character to the mutable.
func (mut *Mutable) Line() { *mut = append(*mut, '\n') }

// Tab writes a single tab character to the mutable.
func (mut *Mutable) Tab() { *mut = append(*mut, '\t') }

// NLines writes n newline characters to the mutable.
// If n is negative, it is a no-op.
func (mut *Mutable) NLines(n int) { ntimes(n, mut.Line) }

// NTabs writes n tab characters to the mutable.
// If n is negative, it is a no-op.
func (mut *Mutable) NTabs(n int) { ntimes(n, mut.Tab) }

// ---- Write helpers ----

// WriteMutable writes the Mutable m to the mutable.
func (mut *Mutable) WriteMutable(m Mutable) { mut.PushBytes(m) }

// WriteLine writes string ln followed by a newline to the mutable.
func (mut *Mutable) WriteLine(ln string) { mut.PushString(ln); mut.Line() }

// WriteLines writes each string in lns followed by a newline to the mutable.
func (mut *Mutable) WriteLines(lns ...string) { apply(mut.WriteLine, lns) }

// WriteBytesLine writes the byte slice ln followed by a newline to the mutable.
func (mut *Mutable) WriteBytesLine(ln []byte) { mut.PushBytes(ln); mut.Line() }

// WriteBytesLines writes each byte slice in lns followed by a newline to the mutable.
func (mut *Mutable) WriteBytesLines(lns ...[]byte) { apply(mut.WriteBytesLine, lns) }

// WriteMutableLine writes m followed by a newline to the mutable.
func (mut *Mutable) WriteMutableLine(m Mutable) { mut.WriteMutable(m); mut.Line() }

// WriteMutableLines writes each Mutable in ms followed by a newline to the mutable.
func (mut *Mutable) WriteMutableLines(ms ...Mutable) { apply(mut.WriteMutableLine, ms) }

// ---- Concat / Join ----

// Concat writes all strings in strs consecutively to the mutable.
func (mut *Mutable) Concat(strs ...string) { mut.cat(strs) }

// JoinStrings writes all strings from s separated by sep to the mutable.
func (mut *Mutable) JoinStrings(s []string, sep string) {
	mut.ExtendStringsJoin(slices.Values(s), sep)
}

// ---- Extend additions ----

// ExtendStringsLines writes each string from seq on its own line to the mutable.
func (mut *Mutable) ExtendStringsLines(seq iter.Seq[string]) { flush(seq, mut.WriteLine) }

// ExtendBytesLines writes each byte slice from seq on its own line to the mutable.
func (mut *Mutable) ExtendBytesLines(seq iter.Seq[[]byte]) { flush(seq, mut.WriteBytesLine) }

// ExtendLines writes each Mutable from seq on its own line to the mutable.
func (mut *Mutable) ExtendLines(seq iter.Seq[Mutable]) { flush(seq, mut.WriteMutableLine) }

// RepeatByte writes byte 'char' to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatByte(char byte, n int) { nwith(n, mut.wb, char) }

// RepeatRune writes rune 'r' to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatRune(r rune, n int) { nwith(n, mut.wrr, r) }

// RepeatLine writes string 'ln' followed by a newline to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatLine(ln string, n int) { nwith(n, mut.WriteLine, ln) }

// ---- PushPrint* ----

// PushPrint formats args using default formatting and writes to the mutable.
// Analogous to fmt.Fprint.
func (mut *Mutable) PushPrint(args ...any) *Mutable { fmt.Fprint(mut, args...); return mut }

// PushPrintf formats according to tpl and writes to the mutable.
// Analogous to fmt.Fprintf.
func (mut *Mutable) PushPrintf(tpl string, args ...any) *Mutable {
	fmt.Fprintf(mut, tpl, args...)
	return mut
}

// PushPrintln formats args using default formatting, appends a newline, and writes to the mutable.
// Analogous to fmt.Fprintln.
func (mut *Mutable) PushPrintln(args ...any) *Mutable { fmt.Fprintln(mut, args...); return mut }

// ---- Numeric / strconv ----

// PushInt writes the decimal string representation of num to the mutable.
func (mut *Mutable) PushInt(num int) { *mut = strconv.AppendInt(*mut, int64(num), 10) }

// PushBool writes "true" or "false" to the mutable.
func (mut *Mutable) PushBool(v bool) { *mut = strconv.AppendBool(*mut, v) }

// PushInt64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) PushInt64(n int64, base int) { *mut = strconv.AppendInt(*mut, n, base) }

// PushUint64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) PushUint64(n uint64, base int) { *mut = strconv.AppendUint(*mut, n, base) }

// PushFloat writes the string representation of f to the mutable.
// The tpl parameter is the format ('b', 'e', 'E', 'f', 'g', 'G', 'x', 'X'),
// prec controls precision, and size is the number of bits (32 or 64).
func (mut *Mutable) PushFloat(f float64, tpl byte, prec, size int) {
	*mut = strconv.AppendFloat(*mut, f, tpl, prec, size)
}

// PushComplex writes the string representation of n to the mutable.
// The tpl parameter is the format ('b', 'e', 'E', 'f', 'g', 'G', 'x', 'X'),
// prec controls precision, and size is the total number of bits (64 or 128).
func (mut *Mutable) PushComplex(n complex128, tpl byte, prec, size int) {
	mut.PushString(strconv.FormatComplex(n, tpl, prec, size))
}

// ---- PushQuote* ----

// PushQuote writes a double-quoted Go string literal for str to the mutable.
func (mut *Mutable) PushQuote(str string) { *mut = strconv.AppendQuote(*mut, str) }

// PushQuoteASCII writes a double-quoted Go string literal for str, escaping non-ASCII.
func (mut *Mutable) PushQuoteASCII(str string) { *mut = strconv.AppendQuoteToASCII(*mut, str) }

// PushQuoteGrapic writes a double-quoted Go string literal for str, escaping non-graphic chars.
func (mut *Mutable) PushQuoteGrapic(str string) { *mut = strconv.AppendQuoteToGraphic(*mut, str) }

// PushQuoteRune writes a single-quoted Go character literal for r to the mutable.
func (mut *Mutable) PushQuoteRune(r rune) { *mut = strconv.AppendQuoteRune(*mut, r) }

// PushQuoteRuneASCII writes a single-quoted Go character literal for r, escaping non-ASCII.
func (mut *Mutable) PushQuoteRuneASCII(r rune) {
	*mut = strconv.AppendQuoteRuneToASCII(*mut, r)
}

// PushQuoteRuneGrapic writes a single-quoted Go character literal for r, escaping non-graphic chars.
func (mut *Mutable) PushQuoteRuneGrapic(r rune) {
	*mut = strconv.AppendQuoteRuneToGraphic(*mut, r)
}

// ---- With* (string transformations written to mutable) ----

// WithTrimSpace writes str with all leading and trailing whitespace removed.
func (mut *Mutable) WithTrimSpace(str string) { mut.PushString(strings.TrimSpace(str)) }

// WithTrimRight writes str with trailing characters in cut removed.
func (mut *Mutable) WithTrimRight(str, cut string) { mut.PushString(strings.TrimRight(str, cut)) }

// WithTrimLeft writes str with leading characters in cut removed.
func (mut *Mutable) WithTrimLeft(str, cut string) { mut.PushString(strings.TrimLeft(str, cut)) }

// WithTrimPrefix writes s with the leading prefix removed.
func (mut *Mutable) WithTrimPrefix(s, prefix string) {
	mut.PushString(strings.TrimPrefix(s, prefix))
}

// WithTrimSuffix writes s with the trailing suffix removed.
func (mut *Mutable) WithTrimSuffix(s, suffix string) {
	mut.PushString(strings.TrimSuffix(s, suffix))
}

// WithReplaceAll writes s with all non-overlapping instances of old replaced by new.
func (mut *Mutable) WithReplaceAll(s, old, new string) { //nolint:predeclared
	mut.PushString(strings.ReplaceAll(s, old, new))
}

// WithReplace writes s with the first n non-overlapping instances of old replaced by new.
// If n is negative, all instances are replaced.
func (mut *Mutable) WithReplace(s, old, new string, n int) { //nolint:predeclared
	mut.PushString(strings.Replace(s, old, new, n))
}

// ---- PushTrim* (byte slice transformations written to mutable) ----

// PushTrimSpace writes str with all leading and trailing whitespace removed.
// bytes.TrimSpace returns a subslice of str (no allocation).
func (mut *Mutable) PushTrimSpace(str []byte) { mut.PushBytes(bytes.TrimSpace(str)) }

// PushTrimRight writes str with trailing characters in cut removed.
// bytes.TrimRight returns a subslice of str (no allocation).
func (mut *Mutable) PushTrimRight(str []byte, cut string) {
	mut.PushBytes(bytes.TrimRight(str, cut))
}

// PushTrimLeft writes str with leading characters in cut removed.
// bytes.TrimLeft returns a subslice of str (no allocation).
func (mut *Mutable) PushTrimLeft(str []byte, cut string) {
	mut.PushBytes(bytes.TrimLeft(str, cut))
}

// PushTrimPrefix writes s with the leading prefix removed.
// bytes.TrimPrefix returns a subslice of s (no allocation).
func (mut *Mutable) PushTrimPrefix(s, prefix []byte) {
	mut.PushBytes(bytes.TrimPrefix(s, prefix))
}

// PushTrimSuffix writes s with the trailing suffix removed.
// bytes.TrimSuffix returns a subslice of s (no allocation).
func (mut *Mutable) PushTrimSuffix(s, suffix []byte) {
	mut.PushBytes(bytes.TrimSuffix(s, suffix))
}

// PushReplaceAll writes s with all non-overlapping instances of old replaced by new.
func (mut *Mutable) PushReplaceAll(s, old, new []byte) { //nolint:predeclared
	mut.PushBytes(bytes.ReplaceAll(s, old, new))
}

// PushReplace writes s with the first n non-overlapping instances of old replaced by new.
// If n is negative, all instances are replaced.
func (mut *Mutable) PushReplace(s, old, new []byte, n int) { //nolint:predeclared
	mut.PushBytes(bytes.Replace(s, old, new, n))
}

// ---- When* ----

// WhenPushPrint calls PushPrint with args if cond is true.
func (mut *Mutable) WhenPushPrint(cond bool, args ...any) *Mutable {
	if cond {
		mut.PushPrint(args...)
	}
	return mut
}

// WhenPushPrintf calls PushPrintf with tpl and args if cond is true.
func (mut *Mutable) WhenPushPrintf(cond bool, tpl string, args ...any) *Mutable {
	if cond {
		mut.PushPrintf(tpl, args...)
	}
	return mut
}

// WhenPushPrintln calls PushPrintln with args if cond is true.
func (mut *Mutable) WhenPushPrintln(cond bool, args ...any) *Mutable {
	if cond {
		mut.PushPrintln(args...)
	}
	return mut
}

// WhenLine writes a newline if cond is true.
func (mut *Mutable) WhenLine(cond bool) { ifop(cond, mut.Line) }

// WhenTab writes a tab if cond is true.
func (mut *Mutable) WhenTab(cond bool) { ifop(cond, mut.Tab) }

// WhenNLines writes n newlines if cond is true.
func (mut *Mutable) WhenNLines(cond bool, n int) { ifwith(cond, mut.NLines, n) }

// WhenNTabs writes n tabs if cond is true.
func (mut *Mutable) WhenNTabs(cond bool, n int) { ifwith(cond, mut.NTabs, n) }

// WhenWrite writes buf if cond is true.
func (mut *Mutable) WhenWrite(cond bool, buf []byte) { ifwith(cond, mut.PushBytes, buf) }

// WhenWriteString writes s if cond is true.
func (mut *Mutable) WhenWriteString(cond bool, s string) { ifwith(cond, mut.PushString, s) }

// WhenWriteByte writes bt if cond is true.
func (mut *Mutable) WhenWriteByte(cond bool, bt byte) {
	if cond {
		_ = mut.WriteByte(bt)
	}
}

// WhenWriteRune writes r if cond is true.
func (mut *Mutable) WhenWriteRune(cond bool, r rune) {
	if cond {
		_, _ = mut.WriteRune(r)
	}
}

// WhenWriteLine writes ln followed by a newline if cond is true.
func (mut *Mutable) WhenWriteLine(cond bool, ln string) { ifwith(cond, mut.WriteLine, ln) }

// WhenWriteLines writes each string in lns on its own line if cond is true.
func (mut *Mutable) WhenWriteLines(cond bool, lns ...string) { ifargs(cond, mut.WriteLines, lns) }

// WhenConcat writes all strings in strs consecutively if cond is true.
func (mut *Mutable) WhenConcat(cond bool, strs ...string) { ifwith(cond, mut.cat, strs) }

// WhenWriteMutable writes m if cond is true.
func (mut *Mutable) WhenWriteMutable(cond bool, m Mutable) { ifwith(cond, mut.WriteMutable, m) }

// WhenWriteMutableLine writes m followed by a newline if cond is true.
func (mut *Mutable) WhenWriteMutableLine(cond bool, m Mutable) {
	ifwith(cond, mut.WriteMutableLine, m)
}

// WhenWriteMutableLines writes each Mutable in ms on its own line if cond is true.
func (mut *Mutable) WhenWriteMutableLines(cond bool, ms ...Mutable) {
	ifargs(cond, mut.WriteMutableLines, ms)
}

// WhenJoin writes strings from sl joined by sep if cond is true.
func (mut *Mutable) WhenJoin(cond bool, sl []string, sep string) {
	iftuple(cond, mut.JoinStrings, sl, sep)
}
