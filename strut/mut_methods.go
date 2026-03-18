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

// ExtendLines writes each string from seq on its own line to the mutable.
func (mut *Mutable) ExtendLines(seq iter.Seq[string]) { flush(seq, mut.WriteLine) }

// ExtendBytesLines writes each byte slice from seq on its own line to the mutable.
func (mut *Mutable) ExtendBytesLines(seq iter.Seq[[]byte]) { flush(seq, mut.WriteBytesLine) }

// ExtendMutableLines writes each Mutable from seq on its own line to the mutable.
func (mut *Mutable) ExtendMutableLines(seq iter.Seq[Mutable]) { flush(seq, mut.WriteMutableLine) }

// ExtendMutable appends all Mutable values from seq to the mutable.
// Equivalent to Extend.
func (mut *Mutable) ExtendMutable(seq iter.Seq[Mutable]) *Mutable { return mut.Extend(seq) }

// ExtendMutableJoin appends Mutable values from seq separated by sep.
// Equivalent to ExtendJoin.
func (mut *Mutable) ExtendMutableJoin(seq iter.Seq[Mutable], sep Mutable) *Mutable {
	return mut.ExtendJoin(seq, sep)
}

// RepeatByte writes byte 'char' to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatByte(char byte, n int) { nwith(n, mut.wb, char) }

// RepeatRune writes rune 'r' to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatRune(r rune, n int) { nwith(n, mut.wrr, r) }

// RepeatLine writes string 'ln' followed by a newline to the mutable 'n' times.
// If 'n' is non-positive, the operation is a no-op.
func (mut *Mutable) RepeatLine(ln string, n int) { nwith(n, mut.WriteLine, ln) }

// ---- AppendPrint* ----

// AppendPrint formats args using default formatting and writes to the mutable.
// Analogous to fmt.Fprint.
func (mut *Mutable) AppendPrint(args ...any) *Mutable { fmt.Fprint(mut, args...); return mut }

// AppendPrintf formats according to tpl and writes to the mutable.
// Analogous to fmt.Fprintf.
func (mut *Mutable) AppendPrintf(tpl string, args ...any) *Mutable {
	fmt.Fprintf(mut, tpl, args...)
	return mut
}

// AppendPrintln formats args using default formatting, appends a newline, and writes to the mutable.
// Analogous to fmt.Fprintln.
func (mut *Mutable) AppendPrintln(args ...any) *Mutable { fmt.Fprintln(mut, args...); return mut }

// ---- Numeric / strconv ----

// Int writes the decimal string representation of num to the mutable.
func (mut *Mutable) Int(num int) { *mut = strconv.AppendInt(*mut, int64(num), 10) }

// AppendBool writes "true" or "false" to the mutable.
func (mut *Mutable) AppendBool(v bool) { *mut = strconv.AppendBool(*mut, v) }

// AppendInt64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) AppendInt64(n int64, base int) { *mut = strconv.AppendInt(*mut, n, base) }

// AppendUint64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) AppendUint64(n uint64, base int) { *mut = strconv.AppendUint(*mut, n, base) }

// AppendFloat writes the string representation of f to the mutable.
// The tpl parameter is the format ('b', 'e', 'E', 'f', 'g', 'G', 'x', 'X'),
// prec controls precision, and size is the number of bits (32 or 64).
func (mut *Mutable) AppendFloat(f float64, tpl byte, prec, size int) {
	*mut = strconv.AppendFloat(*mut, f, tpl, prec, size)
}

// FormatBool writes "true" or "false" to the mutable.
func (mut *Mutable) FormatBool(v bool) { mut.AppendBool(v) }

// FormatInt64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) FormatInt64(n int64, base int) { mut.AppendInt64(n, base) }

// FormatUint64 writes the string representation of n in the given base to the mutable.
func (mut *Mutable) FormatUint64(n uint64, base int) { mut.AppendUint64(n, base) }

// FormatFloat writes the string representation of f to the mutable.
// The tpl parameter is the format ('b', 'e', 'E', 'f', 'g', 'G', 'x', 'X'),
// prec controls precision, and size is the number of bits (32 or 64).
func (mut *Mutable) FormatFloat(f float64, tpl byte, prec, size int) { mut.AppendFloat(f, tpl, prec, size) }

// FormatComplex writes the string representation of n to the mutable.
// The tpl parameter is the format ('b', 'e', 'E', 'f', 'g', 'G', 'x', 'X'),
// prec controls precision, and size is the total number of bits (64 or 128).
func (mut *Mutable) FormatComplex(n complex128, tpl byte, prec, size int) {
	mut.PushString(strconv.FormatComplex(n, tpl, prec, size))
}

// ---- AppendQuote* ----

// AppendQuote writes a double-quoted Go string literal for str to the mutable.
func (mut *Mutable) AppendQuote(str string) { *mut = strconv.AppendQuote(*mut, str) }

// AppendQuoteASCII writes a double-quoted Go string literal for str, escaping non-ASCII.
func (mut *Mutable) AppendQuoteASCII(str string) { *mut = strconv.AppendQuoteToASCII(*mut, str) }

// AppendQuoteGrapic writes a double-quoted Go string literal for str, escaping non-graphic chars.
func (mut *Mutable) AppendQuoteGrapic(str string) { *mut = strconv.AppendQuoteToGraphic(*mut, str) }

// AppendQuoteRune writes a single-quoted Go character literal for r to the mutable.
func (mut *Mutable) AppendQuoteRune(r rune) { *mut = strconv.AppendQuoteRune(*mut, r) }

// AppendQuoteRuneASCII writes a single-quoted Go character literal for r, escaping non-ASCII.
func (mut *Mutable) AppendQuoteRuneASCII(r rune) {
	*mut = strconv.AppendQuoteRuneToASCII(*mut, r)
}

// AppendQuoteRuneGrapic writes a single-quoted Go character literal for r, escaping non-graphic chars.
func (mut *Mutable) AppendQuoteRuneGrapic(r rune) {
	*mut = strconv.AppendQuoteRuneToGraphic(*mut, r)
}

// Quote writes a double-quoted Go string literal representing 'str' to the mutable.
func (mut *Mutable) Quote(str string) { mut.AppendQuote(str) }

// QuoteASCII writes a double-quoted Go string literal for 'str', escaping non-ASCII characters.
func (mut *Mutable) QuoteASCII(str string) { mut.AppendQuoteASCII(str) }

// QuoteGrapic writes a double-quoted Go string literal for 'str', escaping non-graphic characters.
func (mut *Mutable) QuoteGrapic(str string) { mut.AppendQuoteGrapic(str) }

// QuoteRune writes a single-quoted Go character literal representing 'r' to the mutable.
func (mut *Mutable) QuoteRune(r rune) { mut.AppendQuoteRune(r) }

// QuoteRuneASCII writes a single-quoted Go character literal for 'r', escaping non-ASCII characters.
func (mut *Mutable) QuoteRuneASCII(r rune) { mut.AppendQuoteRuneASCII(r) }

// QuoteRuneGrapic writes a single-quoted Go character literal for 'r', escaping non-graphic characters.
func (mut *Mutable) QuoteRuneGrapic(r rune) { mut.AppendQuoteRuneGrapic(r) }

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

// ---- AppendTrim* (byte slice transformations written to mutable) ----

// AppendTrimSpace writes str with all leading and trailing whitespace removed.
// bytes.TrimSpace returns a subslice of str (no allocation).
func (mut *Mutable) AppendTrimSpace(str []byte) { mut.PushBytes(bytes.TrimSpace(str)) }

// AppendTrimSpaceString writes str with all leading and trailing whitespace removed.
func (mut *Mutable) AppendTrimSpaceString(str string) { mut.PushString(strings.TrimSpace(str)) }

// AppendTrimRight writes str with trailing characters in cut removed.
// bytes.TrimRight returns a subslice of str (no allocation).
func (mut *Mutable) AppendTrimRight(str []byte, cut string) {
	mut.PushBytes(bytes.TrimRight(str, cut))
}

// AppendTrimRightString writes str with trailing characters in cut removed.
func (mut *Mutable) AppendTrimRightString(str, cut string) {
	mut.PushString(strings.TrimRight(str, cut))
}

// AppendTrimLeft writes str with leading characters in cut removed.
// bytes.TrimLeft returns a subslice of str (no allocation).
func (mut *Mutable) AppendTrimLeft(str []byte, cut string) {
	mut.PushBytes(bytes.TrimLeft(str, cut))
}

// AppendTrimLeftString writes str with leading characters in cut removed.
func (mut *Mutable) AppendTrimLeftString(str, cut string) {
	mut.PushString(strings.TrimLeft(str, cut))
}

// AppendTrimPrefix writes s with the leading prefix removed.
// bytes.TrimPrefix returns a subslice of s (no allocation).
func (mut *Mutable) AppendTrimPrefix(s, prefix []byte) {
	mut.PushBytes(bytes.TrimPrefix(s, prefix))
}

// AppendTrimPrefixString writes s with the leading prefix removed.
func (mut *Mutable) AppendTrimPrefixString(s, prefix string) {
	mut.PushString(strings.TrimPrefix(s, prefix))
}

// AppendTrimSuffix writes s with the trailing suffix removed.
// bytes.TrimSuffix returns a subslice of s (no allocation).
func (mut *Mutable) AppendTrimSuffix(s, suffix []byte) {
	mut.PushBytes(bytes.TrimSuffix(s, suffix))
}

// AppendTrimSuffixString writes s with the trailing suffix removed.
func (mut *Mutable) AppendTrimSuffixString(s, suffix string) {
	mut.PushString(strings.TrimSuffix(s, suffix))
}

// AppendReplaceAll writes s with all non-overlapping instances of old replaced by new.
func (mut *Mutable) AppendReplaceAll(s, old, new []byte) { //nolint:predeclared
	mut.PushBytes(bytes.ReplaceAll(s, old, new))
}

// AppendReplaceAllString writes s with all non-overlapping instances of old replaced by new.
func (mut *Mutable) AppendReplaceAllString(s, old, new string) { //nolint:predeclared
	mut.PushString(strings.ReplaceAll(s, old, new))
}

// AppendReplace writes s with the first n non-overlapping instances of old replaced by new.
// If n is negative, all instances are replaced.
func (mut *Mutable) AppendReplace(s, old, new []byte, n int) { //nolint:predeclared
	mut.PushBytes(bytes.Replace(s, old, new, n))
}

// AppendReplaceString writes s with the first n non-overlapping instances of old replaced by new.
// If n is negative, all instances are replaced.
func (mut *Mutable) AppendReplaceString(s, old, new string, n int) { //nolint:predeclared
	mut.PushString(strings.Replace(s, old, new, n))
}

// ---- When* ----

// WhenAppendPrint calls AppendPrint with args if cond is true.
func (mut *Mutable) WhenAppendPrint(cond bool, args ...any) *Mutable {
	if cond {
		mut.AppendPrint(args...)
	}
	return mut
}

// WhenAppendPrintf calls AppendPrintf with tpl and args if cond is true.
func (mut *Mutable) WhenAppendPrintf(cond bool, tpl string, args ...any) *Mutable {
	if cond {
		mut.AppendPrintf(tpl, args...)
	}
	return mut
}

// WhenAppendPrintln calls AppendPrintln with args if cond is true.
func (mut *Mutable) WhenAppendPrintln(cond bool, args ...any) *Mutable {
	if cond {
		mut.AppendPrintln(args...)
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
