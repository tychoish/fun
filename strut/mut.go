package strut

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"os"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	// asciiCaseDiff is the difference between uppercase and lowercase ASCII letters ('a' - 'A').
	asciiCaseDiff = 'a' - 'A' // 32
)

var bufpool = sync.Pool{
	New: func() any { return new(Mutable) },
}

// Mutable provides a pooled, mutable byte slice type for efficient
// string manipulation. Methods mirror the bytes package API with
// zero-allocation optimizations for in-place operations and iterator-based
// splitting. Pointer receiver methods mutate in place; value receiver
// methods are read-only. The type implements io.Writer and fmt.Formatter.
//
// While it is safe to create Mutable values using `var name Mutable`
// and `name := Mutable{}`, the NewMutable() constructor takes advantage
// of a buffer pool, and buffers can be returned to the pool using
// Release(). Buffers larger than 64KB are not pooled, following the
// example of the implementation of the buffers in the stdlib fmt
// package.
//
// Mutake attempts, when possible to, to provide efficient in-place
// operations and avoids allocation when possible. Split and Fields
// methods return iter.Seq iterators for zero-allocation iteration.
type Mutable []byte

// NewMutable retrieves a Mutable from the pool. The returned Mutable
// should be released with Release() when no longer needed to enable
// reuse. The initial content is empty but may have non-zero capacity.
func NewMutable() *Mutable { return bufpool.Get().(*Mutable) }

// MakeMutable retrieves a Mutable from the pool and ensures it has
// at least the specified capacity. The returned Mutable has zero length
// and at least the specified capacity. Like NewMutable, the returned
// Mutable should be released with Release() when no longer needed.
func MakeMutable(capacity int) *Mutable {
	mut := bufpool.Get().(*Mutable)
	*mut = (*mut)[:0] // Reset length to 0
	if mut.Cap() < capacity {
		// Grow needs to account for current capacity being freed
		// Since length is now 0, Grow(n) will create capacity of n
		// We need total capacity, so grow by the full amount needed
		mut.Grow(capacity)
	}
	return mut
}

// Format implements fmt.Formatter, allowing Mutable to be used directly
// in formatted output. Writes the underlying bytes to the formatter's
// state.
func (mut Mutable) Format(state fmt.State, _ rune) { _, _ = state.Write(mut) }

// ref returns the Mutable value itself, used internally for capacity
// checks.
func (mut Mutable) ref() Mutable { return mut }

// Append appends the contents of 'next' to this mutable string,
// mutating in place. May allocate if capacity is insufficient.
// Returns the receiver for method chaining.
func (mut *Mutable) Append(next *Mutable) *Mutable { *mut = append(*mut, *next...); return mut }

// Release resets the Mutable and returns it to the pool for reuse.
// Any Mutable instance can be released, whether obtained from NewMutable(),
// MakeMutable(), or created directly. Buffers larger than 64KB are not
// returned to the pool to prevent excessive memory retention. After calling
// Release, the Mutable should not be used again.
func (mut *Mutable) Release() {
	if cap(mut.ref()) > 64*1024 {
		*mut = nil
		return
	}
	*mut = mut.ref()[:0]
	bufpool.Put(mut)
}

// Len returns the length of the mutable string.
func (mut Mutable) Len() int { return len(mut) }

// Cap returns the capacity of the underlying byte slice.
func (mut Mutable) Cap() int { return cap(mut) }

// Grow increases the capacity of the mutable string by 'n' bytes.
// May allocate a new underlying array if additional capacity is
// needed.
func (mut *Mutable) Grow(n int) {
	if n < 0 {
		panic("strut.Mutable.Grow: negative count")
	}
	m := mut.Len()
	if mut.Cap()-m < n {
		buf := make([]byte, m, m+n)
		copy(buf, *mut)
		*mut = buf
	}
}

// Reset resets the mutable string to be empty but retains the underlying storage.
func (mut *Mutable) Reset() { *mut = mut.ref()[:0] }

// Write implements io.Writer, appending the contents of 'p' to the
// mutable string. May allocate if capacity is insufficient.
func (mut *Mutable) Write(p []byte) (n int, err error) {
	*mut = append(*mut, p...)
	return len(p), nil
}

// WriteByte appends a single byte 'c' to the mutable string.
// May allocate if capacity is insufficient.
func (mut *Mutable) WriteByte(c byte) error {
	*mut = append(*mut, c)
	return nil
}

// WriteRune appends the UTF-8 encoding of rune 'r' to the mutable
// string. May allocate if capacity is insufficient.
func (mut *Mutable) WriteRune(r rune) (n int, err error) {
	if r < 0x80 {
		*mut = append(*mut, byte(r))
		return 1, nil
	}
	b := make([]byte, 4)
	n = utf8.EncodeRune(b, r)
	*mut = append(*mut, b[:n]...)
	return n, nil
}

// WriteString appends string 's' to the mutable string.
// May allocate if capacity is insufficient.
func (mut *Mutable) WriteString(s string) (n int, err error) {
	*mut = append(*mut, s...)
	return len(s), nil
}

// Reader provides access to an io.Reader for reading from the
// mutable string. Allocates a new bytes.Reader.
func (mut Mutable) Reader() io.Reader { return bytes.NewReader(mut) }

// String returns the mutable string as a regular string.
// Allocates and copies the underlying bytes.
func (mut Mutable) String() string { return string(mut) }

// Bytes returns the underlying byte slice. No allocation.
func (mut Mutable) Bytes() []byte { return []byte(mut) }

// Copy copies the contents of this mutable string to 'dst', resizing
// 'dst' as needed. This overwrites the content of 'dst', reusing the
// underlying storage if it has sufficient capacity.
func (mut Mutable) Copy(dst *Mutable) error {
	*dst = append((*dst)[:0], mut...)
	return nil
}

// CopyTo copies the contents of this mutable string to 'dst'.
// Returns an error if 'dst' is too small to hold the contents.
func (mut Mutable) CopyTo(dst []byte) error {
	if len(dst) < len(mut) {
		return io.ErrShortBuffer
	}
	copy(dst, mut)
	return nil
}

// Contains reports whether 'subslice' is within the mutable string.
func (mut Mutable) Contains(subslice []byte) bool { return bytes.Contains(mut, subslice) }

// ContainsString reports whether string 's' is within the mutable
// string.
func (mut Mutable) ContainsString(s string) bool { return bytes.Contains(mut, []byte(s)) }

// ContainsAny reports whether any UTF-8 encoded code points in
// 'chars' are within the mutable string.
func (mut Mutable) ContainsAny(chars string) bool { return bytes.ContainsAny(mut, chars) }

// ContainsRune reports whether rune 'r' is contained in the mutable
// string.
func (mut Mutable) ContainsRune(r rune) bool { return bytes.ContainsRune(mut, r) }

// Count counts the number of non-overlapping instances of 'sep' in
// the mutable string.
func (mut Mutable) Count(sep []byte) int { return bytes.Count(mut, sep) }

// CountString counts the number of non-overlapping instances of 'sep'
// in the mutable string.
func (mut Mutable) CountString(sep string) int { return bytes.Count(mut, []byte(sep)) }

// Equal reports whether the mutable string and 'b' are the same
// length and contain the same bytes.
func (mut Mutable) Equal(b []byte) bool { return bytes.Equal(mut, b) }

// EqualString reports whether the mutable string and 's' are the
// same length and contain the same bytes.
func (mut Mutable) EqualString(s string) bool { return bytes.Equal(mut, []byte(s)) }

// EqualFold reports whether the mutable string and 's' are equal
// under Unicode case-folding.
func (mut Mutable) EqualFold(s []byte) bool { return bytes.EqualFold(mut, s) }

// EqualFoldString reports whether the mutable string and 's' are
// equal under Unicode case-folding.
func (mut Mutable) EqualFoldString(s string) bool { return bytes.EqualFold(mut, []byte(s)) }

// HasPrefix tests whether the mutable string begins with 'prefix'.
func (mut Mutable) HasPrefix(prefix []byte) bool { return bytes.HasPrefix(mut, prefix) }

// HasPrefixString tests whether the mutable string begins with
// 'prefix'.
func (mut Mutable) HasPrefixString(prefix string) bool { return bytes.HasPrefix(mut, []byte(prefix)) }

// HasSuffix tests whether the mutable string ends with 'suffix'.
func (mut Mutable) HasSuffix(suffix []byte) bool { return bytes.HasSuffix(mut, suffix) }

// HasSuffixString tests whether the mutable string ends with
// 'suffix'.
func (mut Mutable) HasSuffixString(suffix string) bool { return bytes.HasSuffix(mut, []byte(suffix)) }

// Index returns the index of the first instance of 'sep' in the
// mutable string, or -1 if 'sep' is not present.
func (mut Mutable) Index(sep []byte) int { return bytes.Index(mut, sep) }

// IndexString returns the index of the first instance of 'sep' in the
// mutable string, or -1 if 'sep' is not present.
func (mut Mutable) IndexString(sep string) int { return bytes.Index(mut, []byte(sep)) }

// IndexAny returns the index of the first instance of any code point
// from 'chars' in the mutable string.
func (mut Mutable) IndexAny(chars string) int { return bytes.IndexAny(mut, chars) }

// IndexByte returns the index of the first instance of 'c' in the
// mutable string, or -1 if 'c' is not present.
func (mut Mutable) IndexByte(c byte) int { return bytes.IndexByte(mut, c) }

// IndexRune returns the index of the first instance of rune 'r', or
// -1 if the rune is not present.
func (mut Mutable) IndexRune(r rune) int { return bytes.IndexRune(mut, r) }

// LastIndex returns the index of the last instance of 'sep' in the
// mutable string, or -1 if 'sep' is not present.
func (mut Mutable) LastIndex(sep []byte) int { return bytes.LastIndex(mut, sep) }

// LastIndexString returns the index of the last instance of 'sep' in
// the mutable string, or -1 if 'sep' is not present.
func (mut Mutable) LastIndexString(sep string) int { return bytes.LastIndex(mut, []byte(sep)) }

// LastIndexAny returns the index of the last instance of any code
// point from 'chars' in the mutable string.
func (mut Mutable) LastIndexAny(chars string) int { return bytes.LastIndexAny(mut, chars) }

// LastIndexByte returns the index of the last instance of 'c' in the
// mutable string, or -1 if 'c' is not present.
func (mut Mutable) LastIndexByte(c byte) int { return bytes.LastIndexByte(mut, c) }

// Compare returns an integer comparing two byte slices
// lexicographically.
func (mut Mutable) Compare(b []byte) int { return bytes.Compare(mut, b) }

// CompareString returns an integer comparing the mutable string with
// 's' lexicographically.
func (mut Mutable) CompareString(s string) int { return bytes.Compare(mut, []byte(s)) }

// Clone returns a copy of the mutable string.
// Allocates a new Mutable with the same content.
func (mut Mutable) Clone() *Mutable {
	clone := make(Mutable, len(mut))
	copy(clone, mut)
	return &clone
}

// Cut slices the mutable string around the first instance of 'sep',
// returning the text before and after 'sep'.
func (mut Mutable) Cut(sep []byte) (before, after []byte, found bool) {
	return bytes.Cut(mut, sep)
}

// CutString slices the mutable string around the first instance of
// 'sep', returning the text before and after 'sep'.
func (mut Mutable) CutString(sep string) (before, after []byte, found bool) {
	return bytes.Cut(mut, []byte(sep))
}

// CutPrefix returns the mutable string without the provided leading
// 'prefix' and reports whether it found the prefix.
func (mut Mutable) CutPrefix(prefix []byte) (after []byte, found bool) {
	return bytes.CutPrefix(mut, prefix)
}

// CutPrefixString returns the mutable string without the provided
// leading 'prefix' and reports whether it found the prefix.
func (mut Mutable) CutPrefixString(prefix string) (after []byte, found bool) {
	return bytes.CutPrefix(mut, []byte(prefix))
}

// CutSuffix returns the mutable string without the provided ending
// 'suffix' and reports whether it found the suffix.
func (mut Mutable) CutSuffix(suffix []byte) (before []byte, found bool) {
	return bytes.CutSuffix(mut, suffix)
}

// CutSuffixString returns the mutable string without the provided
// ending 'suffix' and reports whether it found the suffix.
func (mut Mutable) CutSuffixString(suffix string) (before []byte, found bool) {
	return bytes.CutSuffix(mut, []byte(suffix))
}

// Fields splits the mutable string around each instance of one or
// more consecutive white space characters, as defined by unicode.IsSpace,
// returning an iterator that yields each field. Zero allocation for the
// iterator itself.
func (mut Mutable) Fields() iter.Seq[Mutable] {
	return mut.FieldsFunc(unicode.IsSpace)
}

// FieldsFunc splits the mutable string at each run of code points
// 'c' satisfying 'f(c)', returning an iterator. Zero allocation for
// the iterator itself.
func (mut Mutable) FieldsFunc(f func(rune) bool) iter.Seq[Mutable] {
	return func(yield func(Mutable) bool) {
		s := []byte(mut)
		start := -1

		for i := 0; i < len(s); {
			r, size := utf8.DecodeRune(s[i:])
			isField := f(r)
			switch {
			case isField && start >= 0:
				m := Mutable(s[start:i])
				if !yield(m) {
					return
				}
				start = -1
			case !isField && start < 0:
				start = i
			}
			i += size
		}
		if start >= 0 {
			m := Mutable(s[start:])
			yield(m)
		}
	}
}

// splitHelper is a helper function that implements the common logic for
// Split and SplitAfter. The includeSep parameter determines whether the
// separator is included in the yielded slices.
func splitHelper(mut Mutable, sep []byte, includeSep bool) iter.Seq[Mutable] {
	return func(yield func(Mutable) bool) {
		if len(sep) == 0 {
			// Empty separator: split into individual bytes
			for i := range mut {
				m := Mutable(mut[i : i+1])
				if !yield(m) {
					return
				}
			}
			return
		}

		s := []byte(mut)
		for {
			idx := bytes.Index(s, sep)
			if idx < 0 {
				m := Mutable(s)
				yield(m)
				return
			}
			endIdx := idx
			if includeSep {
				endIdx += len(sep)
			}
			m := Mutable(s[:endIdx])
			if !yield(m) {
				return
			}
			s = s[idx+len(sep):]
		}
	}
}

// Split returns an iterator that yields subslices separated by 'sep'.
// Zero allocation for the iterator itself; each yielded slice is a
// subslice of the original data (no copying).
func (mut Mutable) Split(sep Mutable) iter.Seq[Mutable] {
	return splitHelper(mut, sep, false)
}

// SplitString returns an iterator that yields subslices separated by
// 'sep'. Zero allocation for the iterator itself; each yielded slice
// is a subslice of the original data (no copying).
func (mut Mutable) SplitString(sep string) iter.Seq[Mutable] {
	return mut.Split([]byte(sep))
}

// splitNHelper is a helper function that implements the common logic for
// SplitN and SplitAfterN. The includeSep parameter determines whether the
// separator is included in the yielded slices.
func splitNHelper(mut Mutable, sep []byte, n int, includeSep bool, unlimitedSplit func(Mutable) iter.Seq[Mutable]) iter.Seq[Mutable] {
	return func(yield func(Mutable) bool) {
		if n == 0 || len(mut) == 0 {
			return
		}
		if n < 0 {
			// No limit: use regular Split or SplitAfter
			for part := range unlimitedSplit(sep) {
				if !yield(part) {
					return
				}
			}
			return
		}

		if len(sep) == 0 {
			// Empty separator: split into individual bytes
			for i := 0; i < len(mut) && i < n-1; i++ {
				m := Mutable(mut[i : i+1])
				if !yield(m) {
					return
				}
			}

			if n == 1 {
				m := Mutable(mut)
				yield(m)
			} else {
				m := Mutable(mut[n-1:])
				yield(m)
			}

			return
		}

		s := []byte(mut)
		var count int
		for count < n-1 {
			idx := bytes.Index(s, sep)
			if idx < 0 {
				break
			}
			endIdx := idx
			if includeSep {
				endIdx += len(sep)
			}
			m := Mutable(s[:endIdx])
			if !yield(m) {
				return
			}
			s = s[idx+len(sep):]
			count++
		}
		// Yield remainder
		if len(s) > 0 || count < n {
			m := Mutable(s)
			yield(m)
		}
	}
}

// SplitN returns an iterator that yields at most 'n' subslices
// separated by 'sep'. If n <= 0, yields all subslices. Zero
// allocation for the iterator itself.
func (mut Mutable) SplitN(sep []byte, n int) iter.Seq[Mutable] {
	return splitNHelper(mut, sep, n, false, mut.Split)
}

// SplitNString returns an iterator that yields at most 'n' subslices
// separated by 'sep'. If n <= 0, yields all subslices. Zero
// allocation for the iterator itself.
func (mut Mutable) SplitNString(sep string, n int) iter.Seq[Mutable] {
	return mut.SplitN([]byte(sep), n)
}

// SplitAfter returns an iterator that yields subslices after each
// instance of 'sep'. Each yielded slice includes the separator. Zero
// allocation for the iterator itself.
func (mut Mutable) SplitAfter(sep Mutable) iter.Seq[Mutable] {
	return splitHelper(mut, sep, true)
}

// SplitAfterString returns an iterator that yields subslices after
// each instance of 'sep'. Each yielded slice includes the separator.
// Zero allocation for the iterator itself.
func (mut Mutable) SplitAfterString(sep string) iter.Seq[Mutable] {
	return mut.SplitAfter([]byte(sep))
}

// SplitAfterN returns an iterator that yields at most 'n' subslices
// after each instance of 'sep'. Each yielded slice includes the
// separator. Zero allocation for the iterator itself.
func (mut Mutable) SplitAfterN(sep []byte, n int) iter.Seq[Mutable] {
	return splitNHelper(mut, sep, n, true, mut.SplitAfter)
}

// SplitAfterNString returns an iterator that yields at most 'n'
// subslices after each instance of 'sep'. Each yielded slice includes
// the separator. Zero allocation for the iterator itself.
func (mut Mutable) SplitAfterNString(sep string, n int) iter.Seq[Mutable] {
	return mut.SplitAfterN([]byte(sep), n)
}

// Replace replaces the first 'n' non-overlapping instances of 'old'
// with 'new', mutating in place. Uses zero-allocation in-place
// replacement when sufficient capacity exists and replacement is
// same-length or smaller; otherwise allocates new storage.
func (mut *Mutable) Replace(old, new []byte, n int) *Mutable { //nolint:predeclared
	if n == 0 || len(old) == 0 {
		return mut
	}

	// Count actual replacements (up to n, or all if n < 0)
	var count int
	s := *mut
	for len(s) > 0 {
		if n >= 0 && count >= n {
			break
		}
		idx := bytes.Index(s, old)
		if idx < 0 {
			break
		}
		count++
		s = s[idx+len(old):]
	}

	if count == 0 {
		return mut
	}

	// Calculate new size
	oldLen := mut.Len()
	newLen := oldLen + count*(len(new)-len(old))

	// Fast path: same length or growing (if fits in capacity)
	// Shrinking replacements require a copy, so just use bytes.Replace
	if len(new) >= len(old) && newLen <= mut.Cap() && newLen >= 0 {
		*mut = replaceInPlace(*mut, old, new, n, newLen)
		return mut
	}

	// Slow path: allocate new storage (includes all shrinking cases)
	*mut = bytes.Replace(*mut, old, new, n)
	return mut
}

// ReplaceString replaces the first 'n' non-overlapping instances of
// 'old' with 'new', mutating in place. Uses zero-allocation in-place
// replacement when sufficient capacity exists; otherwise allocates.
func (mut *Mutable) ReplaceString(old, new string, n int) *Mutable { //nolint:predeclared
	return mut.Replace([]byte(old), []byte(new), n)
}

// ReplaceAll replaces all non-overlapping instances of 'old' with
// 'new', mutating in place. Uses zero-allocation in-place replacement
// when sufficient capacity exists; otherwise allocates new storage.
func (mut *Mutable) ReplaceAll(old, new []byte) *Mutable { //nolint:predeclared
	return mut.Replace(old, new, -1)
}

// ReplaceAllString replaces all non-overlapping instances of 'old'
// with 'new', mutating in place. Uses zero-allocation in-place
// replacement when sufficient capacity exists; otherwise allocates.
func (mut *Mutable) ReplaceAllString(old, new string) *Mutable { //nolint:predeclared
	return mut.Replace([]byte(old), []byte(new), -1)
}

// replaceInPlace performs in-place replacement for same-length and
// growing replacements when the result fits in existing capacity.
// Shrinking replacements are handled by bytes.Replace to avoid the
// allocation cost of copying the original data.
func replaceInPlace(s []byte, old, new []byte, n int, newLen int) []byte { //nolint:predeclared
	if len(old) == len(new) {
		// Same length: simple in-place replacement
		var replaced int
		var start int
		for replaced < n || n < 0 {
			idx := bytes.Index(s[start:], old)
			if idx < 0 {
				break
			}
			idx += start
			copy(s[idx:], new)
			start = idx + len(new)
			replaced++
		}
		return s
	}

	// Growing: need to expand slice and copy backward
	// Note: Shrinking case is handled by bytes.Replace (see Replace method)
	origLen := len(s)
	s = s[:newLen]

	// Find all match positions first
	matches := make([]int, 0, 8)
	var start int
	for len(matches) < n || n < 0 {
		idx := bytes.Index(s[start:origLen], old)
		if idx < 0 {
			break
		}
		matches = append(matches, start+idx)
		start += idx + len(old)
	}

	// Copy backward to avoid overlapping
	writePos := newLen
	readPos := origLen
	for i := len(matches) - 1; i >= 0; i-- {
		matchPos := matches[i]
		// Copy everything after this match
		copyLen := readPos - (matchPos + len(old))
		writePos -= copyLen
		readPos -= copyLen
		copy(s[writePos:], s[readPos:readPos+copyLen])

		// Copy the replacement
		writePos -= len(new)
		copy(s[writePos:], new)

		// Skip the old pattern
		readPos -= len(old)
	}

	// Copy any remaining prefix
	if readPos > 0 {
		copy(s, s[:readPos])
	}

	return s
}

// caseHelper is a helper function that implements the common logic for
// ToLower, ToUpper, and ToTitle. It applies ASCII case conversion in-place
// and falls back to a Unicode function for non-ASCII content.
func caseHelper(mut *Mutable, rangeStart, rangeEnd byte, offset int, unicodeFunc func([]byte) []byte) *Mutable {
	// Fast path: in-place for ASCII
	var hasNonASCII bool
	for i, b := range *mut {
		if b >= rangeStart && b <= rangeEnd {
			(*mut)[i] = byte(int(b) + offset)
		} else if b > 127 {
			hasNonASCII = true
			// Don't break - continue converting ASCII we've seen
		}
	}

	// Slow path: use Unicode function for non-ASCII content
	if hasNonASCII {
		*mut = unicodeFunc(*mut)
	}
	return mut
}

// ToLower converts all Unicode letters to their lower case, mutating
// in place. Uses zero-allocation ASCII fast path when possible; only
// allocates new storage for non-ASCII Unicode content.
func (mut *Mutable) ToLower() *Mutable {
	return caseHelper(mut, 'A', 'Z', asciiCaseDiff, bytes.ToLower)
}

// ToUpper converts all Unicode letters to their upper case, mutating
// in place. Uses zero-allocation ASCII fast path when possible; only
// allocates new storage for non-ASCII Unicode content.
func (mut *Mutable) ToUpper() *Mutable {
	return caseHelper(mut, 'a', 'z', -asciiCaseDiff, bytes.ToUpper)
}

// ToTitle converts all Unicode letters to their title case, mutating
// in place. Uses zero-allocation ASCII fast path when possible; only
// allocates new storage for non-ASCII Unicode content.
func (mut *Mutable) ToTitle() *Mutable {
	return caseHelper(mut, 'a', 'z', -asciiCaseDiff, bytes.ToTitle)
}

// Trim slices off all leading and trailing UTF-8-encoded code points
// contained in 'cutset', mutating in place. Returns a subslice, no
// allocation.
func (mut *Mutable) Trim(cutset string) *Mutable {
	*mut = bytes.Trim(*mut, cutset)
	return mut
}

// TrimLeft slices off all leading UTF-8-encoded code points contained
// in 'cutset', mutating in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimLeft(cutset string) *Mutable {
	*mut = bytes.TrimLeft(*mut, cutset)
	return mut
}

// TrimRight slices off all trailing UTF-8-encoded code points
// contained in 'cutset', mutating in place. Returns a subslice, no
// allocation.
func (mut *Mutable) TrimRight(cutset string) *Mutable {
	*mut = bytes.TrimRight(*mut, cutset)
	return mut
}

// TrimSpace slices off all leading and trailing white space, mutating
// in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimSpace() *Mutable {
	*mut = bytes.TrimSpace(*mut)
	return mut
}

// TrimPrefix removes the provided leading 'prefix', mutating in
// place. Returns a subslice, no allocation.
func (mut *Mutable) TrimPrefix(prefix []byte) *Mutable {
	*mut = bytes.TrimPrefix(*mut, prefix)
	return mut
}

// TrimPrefixString removes the provided leading 'prefix' string,
// mutating in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimPrefixString(prefix string) *Mutable {
	*mut = bytes.TrimPrefix(*mut, []byte(prefix))
	return mut
}

// TrimSuffix removes the provided trailing 'suffix', mutating in
// place. Returns a subslice, no allocation.
func (mut *Mutable) TrimSuffix(suffix []byte) *Mutable {
	*mut = bytes.TrimSuffix(*mut, suffix)
	return mut
}

// TrimSuffixString removes the provided trailing 'suffix' string,
// mutating in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimSuffixString(suffix string) *Mutable {
	*mut = bytes.TrimSuffix(*mut, []byte(suffix))
	return mut
}

// TrimFunc slices off all leading and trailing code points 'c'
// satisfying 'f(c)', mutating in place. Returns a subslice, no
// allocation.
func (mut *Mutable) TrimFunc(f func(rune) bool) *Mutable {
	*mut = bytes.TrimFunc(*mut, f)
	return mut
}

// TrimLeftFunc slices off all leading code points 'c' satisfying
// 'f(c)', mutating in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimLeftFunc(f func(rune) bool) *Mutable {
	*mut = bytes.TrimLeftFunc(*mut, f)
	return mut
}

// TrimRightFunc slices off all trailing code points 'c' satisfying
// 'f(c)', mutating in place. Returns a subslice, no allocation.
func (mut *Mutable) TrimRightFunc(f func(rune) bool) *Mutable {
	*mut = bytes.TrimRightFunc(*mut, f)
	return mut
}

// Repeat repeats the mutable string 'count' times, mutating in place.
// Uses zero-allocation in-place copy when sufficient capacity exists;
// otherwise allocates new storage for the repeated content.
func (mut *Mutable) Repeat(count int) *Mutable {
	if count <= 0 {
		*mut = (*mut)[:0]
		return mut
	}
	if count == 1 {
		return mut
	}

	origLen := mut.Len()
	newLen := origLen * count

	// Fast path: reuse capacity if available
	if newLen <= mut.Cap() {
		// Save original content
		orig := make([]byte, origLen)
		copy(orig, *mut)

		// Expand to target size
		*mut = (*mut)[:newLen]

		// Copy pattern repeatedly
		for i := 1; i < count; i++ {
			copy((*mut)[i*origLen:(i+1)*origLen], orig)
		}
		return mut
	}

	// Slow path: allocate new storage
	*mut = bytes.Repeat((*mut)[:origLen], count)
	return mut
}

// Map modifies all characters according to the 'mapping' function,
// mutating in place. May allocate new storage if character widths
// change.
func (mut *Mutable) Map(mapping func(rune) rune) *Mutable {
	*mut = bytes.Map(mapping, *mut)
	return mut
}

// IsASCII reports whether all bytes in the mutable string are valid
// ASCII (values 0-127).
func (mut Mutable) IsASCII() bool {
	for _, b := range mut {
		if b > 127 {
			return false
		}
	}
	return true
}

// IsUnicode reports whether the mutable string contains only valid
// UTF-8 encoded Unicode characters.
func (mut Mutable) IsUnicode() bool {
	return utf8.Valid(mut)
}

// IsNullTerminated reports whether the mutable string ends with a
// null byte (0x00), indicating C-style null termination.
func (mut Mutable) IsNullTerminated() bool {
	if len(mut) == 0 {
		return false
	}
	return mut[len(mut)-1] == 0
}

// Print writes the contents of the mutable string to standard output.
func (mut Mutable) Print() { _, _ = os.Stdout.Write(mut) }

// Println writes the context of the Mutable to standard output, adding
// a new line at the end.
func (mut Mutable) Println() { mut.Print(); _, _ = os.Stdout.Write([]byte{'\n'}) }
