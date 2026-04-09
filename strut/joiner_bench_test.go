package strut

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
)

var benchJoinInputs = []string{
	"alpha", "bravo", "charlie", "delta", "echo",
	"foxtrot", "golf", "hotel", "india", "juliet",
}

// BenchmarkJoinerByType compares joining the same logical content
// across different input/output types using a space separator.
func BenchmarkJoinerByType(b *testing.B) {
	sep := JOIN.WithSpace()

	byteInputs := make([][]byte, len(benchJoinInputs))
	for i, s := range benchJoinInputs {
		byteInputs[i] = []byte(s)
	}

	mutableInputs := make([]*Mutable, len(benchJoinInputs))
	for i, s := range benchJoinInputs {
		mutableInputs[i] = MutableFrom(s)
	}
	bufInputs := make([]*Buffer, len(benchJoinInputs))
	for i, s := range benchJoinInputs {
		bufInputs[i] = &Buffer{Buffer: *bytes.NewBuffer([]byte(s))}
	}
	bbufInputs := make([]*bytes.Buffer, len(benchJoinInputs))
	for i, s := range benchJoinInputs {
		bbufInputs[i] = bytes.NewBuffer([]byte(s))
	}

	b.Run("strings.Join", func(b *testing.B) {
		for range b.N {
			_ = strings.Join(benchJoinInputs, " ")
		}
	})
	b.Run("Strings", func(b *testing.B) {
		for range b.N {
			_ = sep.Strings(benchJoinInputs...)
		}
	})
	b.Run("StringsMutable", func(b *testing.B) {
		for range b.N {
			sep.StringsMutable(benchJoinInputs...).Release()
		}
	})
	b.Run("Bytes", func(b *testing.B) {
		for range b.N {
			sep.Bytes(byteInputs...).Release()
		}
	})
	b.Run("Mutables", func(b *testing.B) {
		for range b.N {
			sep.Mutables(mutableInputs...).Release()
		}
	})
	b.Run("Buffers", func(b *testing.B) {
		for range b.N {
			sep.Buffers(bufInputs...).Release()
		}
	})
	b.Run("BytesBuffer", func(b *testing.B) {
		for range b.N {
			_ = sep.BytesBuffer(bbufInputs...)
		}
	})
}

// BenchmarkJoinerInputForm compares variadic, slice, and seq input forms
// for string and byte inputs, where the input data is not consumed.
func BenchmarkJoinerInputForm(b *testing.B) {
	sep := JOIN.WithComma()

	byteInputs := make([][]byte, len(benchJoinInputs))
	for i, s := range benchJoinInputs {
		byteInputs[i] = []byte(s)
	}

	b.Run("StringsMutable/variadic", func(b *testing.B) {
		for range b.N {
			sep.StringsMutable(benchJoinInputs...).Release()
		}
	})
	b.Run("StringsMutable/slice", func(b *testing.B) {
		for range b.N {
			sep.StringMutableSlice(benchJoinInputs).Release()
		}
	})
	b.Run("StringsMutable/seq", func(b *testing.B) {
		for range b.N {
			sep.StringMutableSeq(slices.Values(benchJoinInputs)).Release()
		}
	})

	b.Run("Bytes/variadic", func(b *testing.B) {
		for range b.N {
			sep.Bytes(byteInputs...).Release()
		}
	})
	b.Run("Bytes/slice", func(b *testing.B) {
		for range b.N {
			sep.BytesSlice(byteInputs).Release()
		}
	})
	b.Run("Bytes/seq", func(b *testing.B) {
		for range b.N {
			sep.BytesSeq(slices.Values(byteInputs)).Release()
		}
	})
}

// BenchmarkJoinerStringOutput compares string-returning join methods.
func BenchmarkJoinerStringOutput(b *testing.B) {
	sep := JOIN.WithDash()
	seq := slices.Values(benchJoinInputs)

	b.Run("strings.Join", func(b *testing.B) {
		for range b.N {
			_ = strings.Join(benchJoinInputs, "-")
		}
	})
	b.Run("Strings/variadic", func(b *testing.B) {
		for range b.N {
			_ = sep.Strings(benchJoinInputs...)
		}
	})
	b.Run("StringSlice", func(b *testing.B) {
		for range b.N {
			_ = sep.StringSlice(benchJoinInputs)
		}
	})
	b.Run("StringSeq", func(b *testing.B) {
		for range b.N {
			_ = sep.StringSeq(seq)
		}
	})
}

// BenchmarkJoinerReaders compares joining io.Readers with a separator
// against bare io.MultiReader.
func BenchmarkJoinerReaders(b *testing.B) {
	sep := JOIN.WithSpace()
	makeReaders := func() []io.Reader {
		rs := make([]io.Reader, len(benchJoinInputs))
		for i, s := range benchJoinInputs {
			rs[i] = strings.NewReader(s)
		}
		return rs
	}

	b.Run("Readers", func(b *testing.B) {
		for range b.N {
			rs := makeReaders()
			r := sep.Readers(rs...)
			io.Discard.Write([]byte{}) // keep r alive
			_ = r
		}
	})
	b.Run("ReaderSlice", func(b *testing.B) {
		for range b.N {
			rs := makeReaders()
			r := sep.ReaderSlice(rs)
			_ = r
		}
	})
	b.Run("io.MultiReader/no-sep", func(b *testing.B) {
		for range b.N {
			rs := makeReaders()
			r := io.MultiReader(rs...)
			_ = r
		}
	})
}
