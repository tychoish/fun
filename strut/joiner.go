package strut

import (
	"bytes"
	"io"
	"iter"
	"slices"
	"strings"
)

// Joiner provides a number of methods to join/concatenate string
// sequences/collections of string and string-like operations in an
// ergonomic and memory-efficient manner. Use
// `strut.JOIN.With<separator>() to produce a joiner object, the methods
// of which can be used (repeatedly) as needed.
//
// For simple concatenation `JOIN.WithConcat` joins objects with no
// separator.
//
// Zero-value inputs are not filtered or removed and so when joined will
// produce a sequence of separators.
type Joiner struct{ sep string }

// JoinConstructors is a namespacing type to support the JOIN
// namespace. While you _can_ construct these objects yourself.
type JoinConstructors struct{}

// JOIN provides a namespsace for string/string-linke join/concatenation
// operations constructor provided by the `Joiner` type.
var JOIN JoinConstructors

// WithSpace returns a Joiner that separates elements with a single space (" ").
func (JoinConstructors) WithSpace() Joiner { return Joiner{sep: " "} }

// WithDoubleSpace returns a Joiner that separates elements with two spaces ("  ").
func (JoinConstructors) WithDoubleSpace() Joiner { return Joiner{sep: "  "} }

// WithTab returns a Joiner that separates elements with a horizontal tab ("\t").
func (JoinConstructors) WithTab() Joiner { return Joiner{sep: "\t"} }

// WithLine returns a Joiner that separates elements with a single newline ("\n").
func (JoinConstructors) WithLine() Joiner { return Joiner{sep: "\n"} }

// WithDoubleLine returns a Joiner that separates elements with two newlines ("\n\n"),
// producing a blank line between each element.
func (JoinConstructors) WithDoubleLine() Joiner { return Joiner{sep: "\n\n"} }

// WithConcat returns a Joiner with no separator, producing plain concatenation.
func (JoinConstructors) WithConcat() Joiner { return Joiner{sep: ""} }

// WithUnderscore returns a Joiner that separates elements with an underscore ("_").
func (JoinConstructors) WithUnderscore() Joiner { return Joiner{sep: "_"} }

// WithDash returns a Joiner that separates elements with a single dash ("-").
func (JoinConstructors) WithDash() Joiner { return Joiner{sep: "-"} }

// WithDoubleDash returns a Joiner that separates elements with two dashes ("--").
func (JoinConstructors) WithDoubleDash() Joiner { return Joiner{sep: "--"} }

// WithComma returns a Joiner that separates elements with a comma (",").
func (JoinConstructors) WithComma() Joiner { return Joiner{sep: ","} }

// WithSemiColon returns a Joiner that separates elements with a semicolon (";").
func (JoinConstructors) WithSemiColon() Joiner { return Joiner{sep: ";"} }

// With returns a Joiner that uses sep as the separator between elements.
// sep may be any string, including empty (equivalent to WithConcat) or
// multi-character.
func (JoinConstructors) With(sep string) Joiner { return Joiner{sep: sep} }

// Mutables joins the provided Mutable values with the separator, returning a
// new pooled Mutable. The output capacity is pre-calculated from the input
// lengths, so a single pool allocation is made (or none if the pool provides a
// buffer of sufficient capacity). Inputs are not consumed or released.
// Returns an empty Mutable for zero inputs.
func (sj Joiner) Mutables(muts ...*Mutable) *Mutable {
	sep := []byte(sj.sep)
	out := MakeMutable(sumLen(muts) + lenSeps(sj.sep, len(muts)))
	for i, m := range muts {
		if i != 0 {
			out.copyExtend(sep)
		}
		out.copyExtend(*m)
	}
	return out
}

// MutableSeq joins all Mutable values yielded by seq with the separator,
// returning a new pooled Mutable. Because the total size is unknown before
// iteration, the buffer starts at 2048 bytes and grows using
// copyExtendGrowthFactor when capacity is exceeded; this amortises
// reallocations to O(log n) for long sequences. Inputs are not released.
// Returns an empty Mutable if seq yields nothing.
func (sj Joiner) MutableSeq(seq iter.Seq[*Mutable]) *Mutable {
	sep := []byte(sj.sep)
	out := MakeMutable(2048)
	for m := range seq {
		if out.Len() != 0 {
			out.copyExtend(sep)
		}
		out.copyExtend(*m)
	}
	return out
}

// MutableSlice joins the provided Mutable slice with the separator.
// Delegates to Mutables; see that method for allocation behaviour.
func (sj Joiner) MutableSlice(muts []*Mutable) *Mutable {
	return sj.Mutables(muts...)
}

// Strings joins the provided strings with the separator, returning a new
// string. Delegates directly to strings.Join; allocates exactly one string.
func (sj Joiner) Strings(strs ...string) string { return strings.Join(strs, sj.sep) }

// StringSeq joins all strings from seq with the separator, returning a new
// string. Uses a pooled Mutable internally; allocates one string for the
// final copy on Resolve. Cannot pre-allocate because total size is unknown.
func (sj Joiner) StringSeq(seq iter.Seq[string]) string {
	return NewMutable().ExtendStringsJoin(seq, sj.sep).Resolve()
}

// StringSlice joins the provided string slice with the separator.
// Delegates to strings.Join; allocates exactly one string.
func (sj Joiner) StringSlice(seq []string) string { return strings.Join(seq, sj.sep) }

// StringsMutable joins the provided strings with the separator, returning a
// pooled Mutable. The output capacity is pre-calculated from the input
// lengths, so a single pool allocation is made (or none if the pool provides a
// buffer of sufficient capacity). Prefer this over StringSeq when the caller
// needs a Mutable rather than a string and all inputs are available upfront.
func (sj Joiner) StringsMutable(strs ...string) *Mutable {
	sep := sj.sep
	total := len(sep) * max(0, len(strs)-1)
	for _, s := range strs {
		total += len(s)
	}
	out := MakeMutable(total)
	for i, s := range strs {
		if i != 0 {
			out.copyExtendString(sep)
		}
		out.copyExtendString(s)
	}
	return out
}

// StringMutableSeq joins all strings from seq with the separator, returning a
// pooled Mutable. Because the total size is unknown before iteration the
// buffer grows dynamically using copyExtendGrowthFactor, giving O(log n)
// reallocations. Returns an empty Mutable if seq yields nothing.
func (sj Joiner) StringMutableSeq(seq iter.Seq[string]) *Mutable {
	return NewMutable().ExtendStringsJoin(seq, sj.sep)
}

// StringMutableSlice joins the provided string slice with the separator,
// returning a pooled Mutable. Delegates to StringsMutable; see that method
// for allocation behaviour.
func (sj Joiner) StringMutableSlice(strs []string) *Mutable {
	return sj.StringsMutable(strs...)
}

// Bytes joins the provided byte slices with the separator, returning a pooled
// Mutable. The output capacity is pre-calculated, so at most one pool
// allocation is made. The input slices are not modified.
// Returns an empty Mutable for zero inputs.
func (sj Joiner) Bytes(rs ...[]byte) *Mutable {
	sep := []byte(sj.sep)
	out := MakeMutable(totalLen(rs) + (len(sep) * max(0, len(rs)-1)))
	for i, b := range rs {
		if i != 0 {
			out.copyExtend(sep)
		}
		out.copyExtend(b)
	}
	return out
}

// BytesSeq joins all byte slices yielded by rs with the separator, returning a
// pooled Mutable. Cannot pre-allocate; grows dynamically using
// copyExtendGrowthFactor. Returns an empty Mutable if rs yields nothing.
func (sj Joiner) BytesSeq(rs iter.Seq[[]byte]) *Mutable {
	return NewMutable().ExtendBytesJoin(rs, []byte(sj.sep))
}

// BytesSlice joins the provided byte-slice slice with the separator.
// Delegates to Bytes; see that method for allocation behaviour.
func (sj Joiner) BytesSlice(rs [][]byte) *Mutable {
	return sj.Bytes(rs...)
}

// Buffers joins the content of the provided pooled Buffers with the separator,
// returning a new pooled Buffer. The output capacity is pre-calculated from
// the input lengths; the inputs are not consumed or released.
// Returns an empty Buffer for zero inputs.
func (sj Joiner) Buffers(rs ...*Buffer) *Buffer {
	sep := sj.sep
	out := MakeBuffer(sumLen(rs) + (len(sep) * max(0, len(rs)-1)))
	for i, b := range rs {
		if i != 0 {
			out.WriteString(sep)
		}
		out.Write(b.Bytes())
	}
	return out
}

// BufferSeq joins the content of all pooled Buffers yielded by rs with the
// separator, returning a new pooled Buffer. Cannot pre-allocate; bytes.Buffer
// grows internally using its own doubling strategy.
// Returns an empty Buffer if rs yields nothing.
func (sj Joiner) BufferSeq(rs iter.Seq[*Buffer]) *Buffer {
	sep := sj.sep
	out := MakeBuffer(0)
	var ct int
	for b := range rs {
		if ct != 0 {
			out.WriteString(sep)
		}
		ct++
		out.Write(b.Bytes())
	}
	return out
}

func lenSeps(sep string, num int) int { return len(sep) * max(0, num-1) }

// BufferSlice joins the content of the provided Buffer slice (by value) with
// the separator, returning a new pooled Buffer. The output capacity is
// pre-calculated. Note that Buffer values are copied into the slice parameter;
// prefer BufferPtrsSlice to avoid copying when working with pointers.
func (sj Joiner) BufferSlice(rs []Buffer) *Buffer {
	out := MakeBuffer(sumLenBuf(rs) + lenSeps(sj.sep, len(rs)))
	for i := range rs {
		if i != 0 {
			out.WriteString(sj.sep)
		}
		out.Write(rs[i].Bytes())
	}
	return out
}

// BufferPtrsSlice joins the content of the provided Buffer pointer slice with
// the separator, returning a new pooled Buffer. The output capacity is
// pre-calculated. Prefer this over BufferSlice to avoid value copies.
func (sj Joiner) BufferPtrsSlice(rs []*Buffer) *Buffer {
	out := MakeBuffer(sumLen(rs) + lenSeps(sj.sep, len(rs)))
	for i := range rs {
		if i != 0 {
			out.WriteString(sj.sep)
		}
		out.Write(rs[i].Bytes())
	}
	return out
}

// BytesBuffer joins the content of the provided bytes.Buffers with the
// separator, returning a new *bytes.Buffer. Always allocates (bytes.Buffer is
// not pooled). The output capacity is pre-calculated to avoid internal
// growth reallocations. Inputs are not modified.
// Returns an empty *bytes.Buffer for zero inputs.
func (sj Joiner) BytesBuffer(rs ...*bytes.Buffer) *bytes.Buffer {
	sep := sj.sep
	out := new(bytes.Buffer)
	out.Grow(sumLen(rs) + lenSeps(sj.sep, len(rs)))
	for i, b := range rs {
		if i != 0 {
			out.WriteString(sep)
		}
		out.Write(b.Bytes())
	}
	return out
}

// BytesBufferSeq joins the content of all *bytes.Buffers yielded by rs with
// the separator, returning a new *bytes.Buffer. Always allocates. Cannot
// pre-allocate; bytes.Buffer grows using its own internal strategy.
// Returns an empty *bytes.Buffer if rs yields nothing.
func (sj Joiner) BytesBufferSeq(rs iter.Seq[*bytes.Buffer]) *bytes.Buffer {
	sep := sj.sep
	out := new(bytes.Buffer)
	var ct int
	for b := range rs {
		if ct != 0 {
			out.WriteString(sep)
		}
		ct++
		out.Write(b.Bytes())
	}
	return out
}

// BytesBufferSlice joins the content of the provided bytes.Buffer slice (by
// value) with the separator, returning a new *bytes.Buffer. Always allocates.
// The output capacity is pre-calculated. Note that bytes.Buffer values are
// copied into the slice parameter; prefer BytesBufferPtrsSlice when working
// with pointers.
func (sj Joiner) BytesBufferSlice(rs []bytes.Buffer) *bytes.Buffer {
	out := new(bytes.Buffer)
	out.Grow(lenSeps(sj.sep, len(rs)) + sumLenBytesBuf(rs))
	for i := range rs {
		if i != 0 {
			out.WriteString(sj.sep)
		}
		out.Write(rs[i].Bytes())
	}
	return out
}

// BytesBufferPtrsSlice joins the content of the provided *bytes.Buffer slice
// with the separator. Delegates to BytesBuffer; see that method for allocation
// behaviour.
func (sj Joiner) BytesBufferPtrsSlice(rs []*bytes.Buffer) *bytes.Buffer {
	return sj.BytesBuffer(rs...)
}

// Readers composes the provided readers into a single io.Reader with the
// separator interleaved between each. No content is copied at construction
// time; reads are lazy via io.MultiReader. Each separator is a fresh
// strings.NewReader, so the composed reader can only be read once.
// Returns an empty reader for zero inputs; returns the single reader directly
// (without wrapping) for one input or an empty separator.
func (sj Joiner) Readers(rs ...io.Reader) io.Reader { return joinReaders(sj.sep, rs) }

// ReaderSeq composes all readers yielded by rs into a single io.Reader with
// the separator interleaved. The sequence is fully collected into a slice
// before composing, so all readers are buffered by reference before the first
// Read call. Reads are lazy; no content is copied at construction time.
func (sj Joiner) ReaderSeq(rs iter.Seq[io.Reader]) io.Reader {
	return sj.ReaderSlice(slices.Collect(rs))
}

// ReaderSlice composes the provided reader slice into a single io.Reader with
// the separator interleaved. Delegates to Readers; see that method for
// behaviour and edge cases.
func (sj Joiner) ReaderSlice(rs []io.Reader) io.Reader { return joinReaders(sj.sep, rs) }

////////////////////////////////////////////////////////////////////////
//
// helpers --

func totalLen[E string | []byte, S ~[]E](sl S) (size int) {
	for _, str := range sl {
		size += len(str)
	}
	return
}

func sumLenBuf(sl []Buffer) (size int) {
	for _, str := range sl {
		size += str.Len()
	}
	return
}

func sumLenBytesBuf(sl []bytes.Buffer) (size int) {
	for _, str := range sl {
		size += str.Len()
	}
	return
}

func sumLen[E interface{ Len() int }, S ~[]E](sl S) (size int) {
	for _, str := range sl {
		size += str.Len()
	}
	return
}

// joinReaders interleaves sep between each reader and returns a combined io.Reader.
func joinReaders(sep string, rs []io.Reader) io.Reader {
	if len(rs) == 0 {
		return strings.NewReader("")
	}
	if len(sep) == 0 || len(rs) == 1 {
		return io.MultiReader(rs...)
	}
	readers := make([]io.Reader, 0, len(rs)*2-1)
	for i, r := range rs {
		if i != 0 {
			readers = append(readers, strings.NewReader(sep))
		}
		readers = append(readers, r)
	}
	return io.MultiReader(readers...)
}
