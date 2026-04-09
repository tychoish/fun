package strut

import (
	"bytes"
	"io"
	"iter"
	"slices"
	"strings"

	"github.com/tychoish/fun/irt"
)

type Joiner struct{ sep string }

type JoinConstructors struct{}

var JOIN JoinConstructors

func (JoinConstructors) WithSpace() Joiner       { return Joiner{sep: " "} }
func (JoinConstructors) WithDoubleSpace() Joiner { return Joiner{sep: "  "} }
func (JoinConstructors) WithTab() Joiner         { return Joiner{sep: "\t"} }
func (JoinConstructors) WithLine() Joiner        { return Joiner{sep: "\n"} }
func (JoinConstructors) WithDoubleLine() Joiner  { return Joiner{sep: "\n\n"} }
func (JoinConstructors) WithConcat() Joiner      { return Joiner{sep: ""} }
func (JoinConstructors) WithUnderscore() Joiner  { return Joiner{sep: "_"} }
func (JoinConstructors) WithDash() Joiner        { return Joiner{sep: "-"} }
func (JoinConstructors) WithDoubleDash() Joiner  { return Joiner{sep: "--"} }
func (JoinConstructors) WithComma() Joiner       { return Joiner{sep: ","} }
func (JoinConstructors) WithSemiColon() Joiner   { return Joiner{sep: ";"} }
func (JoinConstructors) With(sep string) Joiner  { return Joiner{sep: sep} }

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

func (sj Joiner) MutableSeq(seq iter.Seq[*Mutable]) *Mutable {
	sep := []byte(sj.sep)
	// Collect pointers so we can release after building output,
	// avoiding pool pollution that would cause MakeMutable to reallocate.

	out := MakeMutable(2048)
	for i, m := range irt.Index(seq) {
		if i != 0 {
			out.copyExtend(sep)
		}
		out.copyExtend(*m)
	}
	return out
}

func (sj Joiner) MutableSlice(muts []*Mutable) *Mutable {
	return sj.Mutables(muts...)
}

func (sj Joiner) Strings(strs ...string) string { return strings.Join(strs, sj.sep) }

func (sj Joiner) StringSeq(seq iter.Seq[string]) string {
	return NewMutable().ExtendStringsJoin(seq, sj.sep).Resolve()
}

func (sj Joiner) StringSlice(seq []string) string { return strings.Join(seq, sj.sep) }

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

func (sj Joiner) StringMutableSeq(seq iter.Seq[string]) *Mutable {
	return NewMutable().ExtendStringsJoin(seq, sj.sep)
}

func (sj Joiner) StringMutableSlice(strs []string) *Mutable {
	return sj.StringsMutable(strs...)
}

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

func (sj Joiner) BytesSeq(rs iter.Seq[[]byte]) *Mutable {
	return NewMutable().ExtendBytesJoin(rs, []byte(sj.sep))
}

func (sj Joiner) BytesSlice(rs [][]byte) *Mutable {
	return sj.Bytes(rs...)
}

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

func (sj Joiner) BytesBufferPtrsSlice(rs []*bytes.Buffer) *bytes.Buffer { return sj.BytesBuffer(rs...) }

func (sj Joiner) Readers(rs ...io.Reader) io.Reader { return joinReaders(sj.sep, rs) }

func (sj Joiner) ReaderSeq(rs iter.Seq[io.Reader]) io.Reader {
	return sj.ReaderSlice(slices.Collect(rs))
}
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
