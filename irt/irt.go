// Package irt (for IteratoRTools), provides a collection of stateless iterator handling functions, with zero dependencies on other fun packages.
package irt

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"io"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
)

// Collect consumes the sequence and returns a slice of all
// elements. This combines the operations slices.Collect and
// make([]T).
//
// Unlike you can only set the capacity, not the initial length. For
// compatibility, you can specify more than one integer arguments,
// though ONLY one can be non-zero. If you specify a capacity argument
// that is less than zero, it becomes zero.
func Collect[T any](seq iter.Seq[T], args ...int) (s []T) {
	switch len(args) {
	case 0:
	// pass, use the nil slice
	case 1:
		s = make([]T, 0, idxorz(args, 0))
	case 2:
		a, b := idxorz(args, 0), idxorz(args, 1)
		if a != 0 && b != 0 {
			panic("collect can have at most one non-zero argument")
		}
		s = make([]T, 0, max(0, a, b))
	default:
		caps := slices.Collect(RemoveZeros(Slice(args)))
		if len(caps) > 1 {
			panic("can only specify ONE non-zero capaciy argument to Collect.")
		}

		s = make([]T, 0, max(0, idxorz(args, 0)))
	}
	return slices.AppendSeq(s, seq)
}

// Collect2 consumes the sequence and returns a map with all
// elements. This combines the operations maps.Collect and
// make(map[K]V).
//
// Like make(map[K]V), the optional args are args[0] sets initial
// length.
func Collect2[K comparable, V any](seq iter.Seq2[K, V], args ...int) map[K]V {
	mp := make(map[K]V, max(0, idxorz(args, 0)))
	maps.Insert(mp, seq)
	return mp
}

// CollectFirstN consumes up to n elements from the sequence and
// returns them as a slice.  If n <= 0, returns an empty slice.
func CollectFirstN[T any](seq iter.Seq[T], n int) []T {
	if n <= 0 {
		return make([]T, 0)
	}
	out := make([]T, 0, n)
	idx := 0
	for value := range seq {
		out = append(out, value)
		if idx+1 == n {
			break
		}
		idx++
	}

	return out
}

// JoinErrors consumes a sequence of errors and returns a single error
// produced by errors.Join. Returns nil if the sequence is empty.
func JoinErrors(seq iter.Seq[error]) error { return errors.Join(Collect(seq)...) }

// JoinStrings takes a sequence of strings and concatenates them. Uses
// a strings.Buffer to minimize allocation overhead.
func JoinStrings[S ~string](seq iter.Seq[S]) S {
	var buf strings.Builder

	for str := range seq {
		buf.WriteString(string(str))
	}

	return S(buf.String())
}

// JoinStringsWith concatenates a sequence of strings and returns one
// string, inserting `with` between elements. If an element in the
// iterator is the empty string, then an extra separator is not
// inserted.
func JoinStringsWith[S, T ~string](seq iter.Seq[S], with T) S {
	var buf strings.Builder
	var lastSize int

	for str := range seq {
		if buf.Len() > lastSize {
			buf.WriteString(string(with))
		}
		lastSize = buf.Len()
		buf.WriteString(string(str))
	}

	return S(buf.String())
}

// One returns a sequence containing exactly one element.
func One[T any](v T) iter.Seq[T] { return func(yield func(T) bool) { yield(v) } }

// Two returns a iterator containing exactly one pair of elements.
func Two[A, B any](a A, b B) iter.Seq2[A, B] { return func(yield func(A, B) bool) { yield(a, b) } }

// Map returns a iterator containing all key-value pairs from the map.
func Map[K comparable, V any, M ~map[K]V](mp M) iter.Seq2[K, V] { return maps.All(mp) }

// MapKV transforms a map into an iterator of KV pairs.
func MapKV[A comparable, B any, M ~map[A]B](mp M) iter.Seq[KV[A, B]] { return KVmap(mp) }

// Slice returns a sequence containing all elements from the slice.
func Slice[T any, S ~[]T](sl S) iter.Seq[T] { return slices.Values(sl) }

// Args returns a sequence containing all provided arguments.
func Args[T any](items ...T) iter.Seq[T] { return Slice(items) }

// Any converts an arbitrary sequence to a sequence of `any` values.
func Any[T any](seq iter.Seq[T]) iter.Seq[any] { return Convert(seq, toany) }

// Any2 converts the second value of a sequence of pairs to be `any` typed, leaving the first value
// unchanged.
func Any2[A, B any](seq iter.Seq2[A, B]) iter.Seq2[A, any] { return Convert2(seq, toany2) }

// Append returns a sequence containing all elements from the input
// sequence followed by additional values provided.
func Append[T any](seq iter.Seq[T], with ...T) iter.Seq[T] { return Chain(Args(seq, Slice(with))) }

// Join concatenates multiple sequences into a single sequence,
// yielding all elements from each sequence in order.
func Join[T any](seqs ...iter.Seq[T]) iter.Seq[T] { return Chain(Slice(seqs)) }

// Join2 concatenates multiple pair sequences into a single pair
// sequence, yielding all key-value pairs from each sequence in order.
func Join2[A, B any](seqs ...iter.Seq2[A, B]) iter.Seq2[A, B] { return Chain2(Slice(seqs)) }

// Monotonic returns an infinite sequence of integers starting from 1.
func Monotonic() iter.Seq[int] { return Generate(counter()) }

// MonotonicFrom returns an infinite sequence of integers starting
// from start.
func MonotonicFrom(start int) iter.Seq[int] { return Generate(counterFrom(start - 1)) }

// Range returns a sequence of integers from start to end (inclusive).
func Range(start int, end int) iter.Seq[int] { return While(MonotonicFrom(start), predLTE(end)) }

// Index returns a iterator where each element from the input sequence
// is paired with its 0-based index.
func Index[T any](seq iter.Seq[T]) iter.Seq2[int, T] { return Flip(WithEach(seq, counterFrom(-1))) }

// Flip returns a iterator where the keys and values of the input
// sequence are swapped.
func Flip[A, B any](seq iter.Seq2[A, B]) iter.Seq2[B, A] { return Convert2(seq, flip) }

// First returns a sequence containing only the first element (key) of
// each pair in the input iterator.
func First[A, B any](seq iter.Seq2[A, B]) iter.Seq[A] { return Merge(seq, first) }

// Second returns a sequence containing only the second element
// (value) of each pair in the input iterator.
func Second[A, B any](seq iter.Seq2[A, B]) iter.Seq[B] { return Merge(seq, second) }

// Ptrs returns a sequence where each element is a pointer to the
// value in the input sequence.
func Ptrs[T any](seq iter.Seq[T]) iter.Seq[*T] { return Convert(seq, ptr) }

// PtrsWithNils returns a sequence of pointers to the values in the
// input sequence.  If a value is the zero value for its type, a nil
// pointer is produced instead.
func PtrsWithNils[T comparable](seq iter.Seq[T]) iter.Seq[*T] { return Convert(seq, ptrznil) }

// Deref returns a sequence of values dereferenced from the input
// sequence of pointers.  Nil pointers in the input sequence are
// skipped.
func Deref[T any](seq iter.Seq[*T]) iter.Seq[T] { return Convert(RemoveNils(seq), derefz) }

// DerefWithZeros returns a sequence of values dereferenced from the
// input sequence of pointers.  Nil pointers in the input sequence
// result in the zero value for the type.
func DerefWithZeros[T any](seq iter.Seq[*T]) iter.Seq[T] { return Convert(seq, derefz) }

// FirstValue returns the first value from the sequence and true.  If
// the sequence is empty, it returns the zero value and false.
func FirstValue[T any](seq iter.Seq[T]) (zero T, ok bool) {
	for value := range seq {
		return value, true
	}
	return
}

// FirstValue2 returns the first pair of values from the iterator and
// true.  If the sequence is empty, it returns zero values and false.
func FirstValue2[A, B any](seq iter.Seq2[A, B]) (azero A, bzero B, ok bool) {
	for k, v := range seq {
		return k, v, true
	}
	return
}

// Limit returns a sequence that yields at most n elements from the
// input sequence.  If n <= 0, the sequence is empty.
func Limit[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		inc := counter()
		for elem := range seq {
			if !yield(elem) || inc() >= n {
				return
			}
		}
	}
}

// Limit2 returns a iterator that yields at most n pairs from the
// input iterator.  If n <= 0, the sequence is empty.
func Limit2[A, B any](seq iter.Seq2[A, B], n int) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if n <= 0 {
			return
		}
		inc := counter()
		for k, v := range seq {
			if !yield(k, v) || inc() >= n {
				return
			}
		}
	}
}

// Convert returns a sequence where each element is the result of
// applying op to the elements of the input sequence.
func Convert[A, B any, OP ~func(A) B](seq iter.Seq[A], op OP) iter.Seq[B] {
	return func(yield func(B) bool) {
		for value := range seq {
			if !yield(op(value)) {
				return
			}
		}
	}
}

// Convert2 returns a iterator where each pair is the result of
// applying op to the pairs of the input iterator.
func Convert2[A, B, C, D any, OP ~func(A, B) (C, D)](seq iter.Seq2[A, B], op OP) iter.Seq2[C, D] {
	return func(yield func(C, D) bool) {
		for key, value := range seq {
			if !yield(op(key, value)) {
				return
			}
		}
	}
}

// Modify2 applies a transformation function to each pair in the
// sequence.  If the operation is nil, the sequence is returned
// unchanged.
func Modify2[A, B any, OP ~func(A, B) (A, B)](seq iter.Seq2[A, B], op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if op != nil {
			seq = Convert2(seq, op)
		}

		flush2(seq, yield)
	}
}

// ModifyAll2 applies a sequence of transformation functions to each
// pair in the sequence.  Nil functions are filtered out and
// skipped. Each transformation is applied in order, with the result
// of one transformation passed to the next.
func ModifyAll2[A, B any](seq iter.Seq2[A, B], ops ...func(A, B) (A, B)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		operations := Collect(Remove(Slice(ops), func(op func(A, B) (A, B)) bool { return op == nil }), 0, len(ops))
		if len(operations) > 0 {
			seq = Convert2(seq, func(a A, b B) (A, B) {
				for op := range Slice(operations) {
					a, b = op(a, b)
				}
				return a, b
			})
		}

		flush2(seq, yield)
	}
}

// Modify applies a transformation function to each element in the
// sequence.  If the operation is nil, the sequence is returned
// unchanged.
func Modify[T any, OP ~func(T) T](seq iter.Seq[T], op OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		if op != nil {
			seq = Convert(seq, op)
		}

		flush(seq, yield)
	}
}

// ModifyAll applies a sequence of transformation functions to each
// element in the sequence.  Nil modification functions are
// skipped. Each transformation is applied in order, with the result
// of one transformation passed to the next.
func ModifyAll[T any, OP ~func(T) T](seq iter.Seq[T], ops ...OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		operations := Collect(Remove(Slice(ops), func(op OP) bool { return op == nil }), 0, len(ops))
		if len(operations) > 0 {
			seq = Convert(seq, func(in T) T {
				for op := range Slice(operations) {
					in = op(in)
				}
				return in
			})
		}

		flush(seq, yield)
	}
}

// Merge returns a sequence where each element is the result of
// applying op to the pairs of the input iterator.
func Merge[A, B, C any, OP ~func(A, B) C](seq iter.Seq2[A, B], op OP) iter.Seq[C] {
	return func(yield func(C) bool) {
		for key, value := range seq {
			if !yield(op(key, value)) {
				return
			}
		}
	}
}

// Generate returns an infinite sequence where each element is
// produced by calling op.
func Generate[T any, OP ~func() T](op OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		for yield(op()) {
			continue
		}
	}
}

// Generate2 returns an infinite iterator where each pair is produced by calling op.
func Generate2[A, B any, OP ~func() (A, B)](op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for yield(op()) {
			continue
		}
	}
}

// With returns a iterator where each element from the input sequence
// is paired with the result of applying op to it.
func With[A, B any, OP ~func(A) B](seq iter.Seq[A], op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for value := range seq {
			if !yield(value, op(value)) {
				return
			}
		}
	}
}

// With2 returns a iterator where each pair is produced by applying op
// to each element of the input sequence.
func With2[A, B, C any, OP ~func(A) (B, C)](seq iter.Seq[A], op OP) iter.Seq2[B, C] {
	return func(yield func(B, C) bool) {
		for value := range seq {
			if !yield(op(value)) {
				return
			}
		}
	}
}

// With3 returns a iterator where each pair is produced by applying op
// to each element of the input sequence.
func With3[A, B, C any, OP ~func(A) (B, C)](seq iter.Seq[A], op OP) iter.Seq2[KV[A, B], C] {
	return func(yield func(KV[A, B], C) bool) {
		for key := range seq {
			value, check := op(key)
			if !yield(MakeKV(key, value), check) {
				return
			}
		}
	}
}

// WithEach returns a iterator where each element from the input
// sequence is paired with a value produced by calling op.
func WithEach[A, B any, OP ~func() B](seq iter.Seq[A], op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for value := range seq {
			if !yield(value, op()) {
				return
			}
		}
	}
}

// GenerateOk returns a sequence that yields values produced by gen as
// long as gen returns true.
func GenerateOk[T any, OP ~func() (T, bool)](gen OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		for val, ok := gen(); ok && yield(val); val, ok = gen() {
			continue
		}
	}
}

// GenerateWhile returns a sequence that yields values produced by op
// as long as they satisfy the while predicate.
func GenerateWhile[T any, OP ~func() T, CHECK ~func(T) bool](op OP, while CHECK) iter.Seq[T] {
	return While(Generate(op), while)
}

// GenerateOk2 returns a iterator that yields pairs produced by gen as
// long as gen returns true.
func GenerateOk2[A, B any, OP ~func() (A, B, bool)](gen OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for first, second, ok := gen(); ok && yield(first, second); first, second, ok = gen() {
			continue
		}
	}
}

// GenerateWhile2 returns a iterator that yields pairs produced by op
// as long as they satisfy the while predicate.
func GenerateWhile2[A, B any, OP ~func() (A, B), WHILE ~func(A, B) bool](op OP, while WHILE) iter.Seq2[A, B] {
	return While2(Generate2(op), while)
}

// GenerateN returns a sequence that yields exactly num elements
// produced by calling op.  If num <= 0, the sequence is empty.
func GenerateN[T any, OP ~func() T](num int, op OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < num && yield(op()); i++ {
			continue
		}
	}
}

// ForEach returns a sequence that calls op for each element of the
// input sequence during iteration.
func ForEach[T any, OP ~func(T)](seq iter.Seq[T], op OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			op(value)
			if !yield(value) {
				return
			}
		}
	}
}

// ForEachWhile returns a sequence that calls op for each element of
// the input sequence.  Iteration stops if op returns false.
func ForEachWhile[T any, OP func(T) bool](seq iter.Seq[T], op OP) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if !op(value) || !yield(value) {
				return
			}
		}
	}
}

// ForEach2 returns a iterator that calls op for each pair of the
// input iterator during iteration.
func ForEach2[A, B any, OP ~func(A, B)](seq iter.Seq2[A, B], op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			op(key, value)
			if !yield(key, value) {
				return
			}
		}
	}
}

// ForEachWhile2 returns a iterator that calls op for each pair of the
// input iterator.  Iteration stops if op returns false.
func ForEachWhile2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], op OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			if !op(key, value) || !yield(key, value) {
				return
			}
		}
	}
}

// Apply consumes the sequence and calls op for each element. Returns
// the number of elements processed.
func Apply[T any, OP ~func(T)](seq iter.Seq[T], op OP) (count int) {
	for value := range seq {
		count++
		op(value)
	}
	return count
}

// RunAll takes a sequences of nilary functions and runs them all,
// returning a count. If any functions are Nil. For other function
// types, and for nil/panic safety, use the operations in the wpa package.
func RunAll[OP ~func()](seq iter.Seq[OP]) (count int) {
	for op := range seq {
		count++
		op()
	}
	return count
}

// ApplyWhile consumes the sequence and calls op for each
// element. Iteration stops if op returns false.  Returns the number
// of elements processed.
func ApplyWhile[T any, OP ~func(T) bool](seq iter.Seq[T], op OP) (count int) {
	for value := range seq {
		count++
		if !op(value) {
			return count
		}
	}
	return count
}

// ApplyUntil consumes the sequence and calls op for each
// element. Iteration stops if op returns an error.  Returns the error
// from op, or nil if the sequence was fully consumed.
func ApplyUntil[T any, OP ~func(T) error](seq iter.Seq[T], op OP) error {
	for value := range seq {
		if err := op(value); err != nil {
			return err
		}
	}
	return nil
}

// ApplyUnless consumes the sequence and calls op for each
// element. Iteration stops if op returns true.  Returns the number of
// elements processed.
func ApplyUnless[T any, OP ~func(T) bool](seq iter.Seq[T], op OP) int {
	return ApplyWhile(seq, notf(op))
}

// ApplyAll consumes the sequence and calls op for each element. It
// collects all errors returned by op and returns them joined.
func ApplyAll[T any, OP ~func(T) error](seq iter.Seq[T], op OP) error {
	return JoinErrors(Convert(seq, op))
}

// ApplyAll2 consumes the iterator and calls op for each pair. It
// collects all errors returned by op and returns them joined.
func ApplyAll2[A, B any, OP ~func(A, B) error](seq iter.Seq2[A, B], op OP) error {
	return JoinErrors(Merge(seq, op))
}

// Apply2 consumes the iterator and calls op for each pair. Returns
// the number of pairs processed.
func Apply2[A, B any, OP ~func(A, B)](seq iter.Seq2[A, B], op OP) (count int) {
	for key, value := range seq {
		count++
		op(key, value)
	}
	return count
}

// ApplyWhile2 consumes the iterator and calls op for each
// pair. Iteration stops if op returns false.  Returns the number of
// pairs processed.
func ApplyWhile2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], op OP) (count int) {
	for key, value := range seq {
		count++
		if !op(key, value) {
			break
		}
	}
	return count
}

// ApplyUnless2 consumes the iterator and calls op for each
// pair. Iteration stops if op returns true.  Returns the number of
// pairs processed.
func ApplyUnless2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], op OP) int {
	return ApplyWhile2(seq, notf2(op))
}

// ApplyUntil2 consumes the iterator and calls op for each
// pair. Iteration stops if op returns an error.  Returns the error
// from op, or nil if the sequence was fully consumed.
func ApplyUntil2[A, B any, OP ~func(A, B) error](seq iter.Seq2[A, B], op OP) error {
	for key, value := range seq {
		if err := op(key, value); err != nil {
			return err
		}
	}
	return nil
}

// Channel returns a sequence that yields elements from the provided
// channel until the channel is closed or the context is canceled.
func Channel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) { loopWhile(func() bool { return yieldFrom(ctx, ch, yield) }) }
}

// Pipe returns a channel that receives all elements from the input
// sequence.  The channel is closed when the sequence is exhausted or
// the context is canceled.
func Pipe[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	return opwithstart(opwithch(func(ch chan T) { seqToChan(ctx, seq, ch) }))
}

// Chunk returns a sequence of sequences, where each inner sequence
// contains at most num elements from the input sequence. If num <= 0,
// the sequence is empty.
func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] {
	return func(yield func(iter.Seq[T]) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		for shouldContinue := num > 0; shouldContinue && yield(func(yield func(T) bool) {
			for range num {
				if value, ok := next(); !ok || !yield(value) {
					shouldContinue = false
					return
				}
			}
		}); {
			continue
		}
	}
}

// Chain flattens a sequence of sequences into a single sequence.
func Chain[T any](seq iter.Seq[iter.Seq[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for inner := range seq {
			if inner != nil {
				for value := range inner {
					if !yield(value) {
						return
					}
				}
			}
		}
	}
}

// Chain2 flattens a sequence of pair sequences into a single pair
// sequence.
func Chain2[A, B any](seq iter.Seq[iter.Seq2[A, B]]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for inner := range seq {
			for key, value := range inner {
				if !yield(key, value) {
					return
				}
			}
		}
	}
}

// ChainSlices flattens a sequence of slices into a single sequence.
func ChainSlices[T any, S ~[]T](seq iter.Seq[S]) iter.Seq[T] { return Chain(Convert(seq, Slice)) }

// ChainMaps flattens a sequence of maps into a single sequence.
func ChainMaps[A comparable, B any, M ~map[A]B](seq iter.Seq[M]) iter.Seq2[A, B] {
	return Chain2(Convert(seq, Map))
}

// RemoveNils returns a sequence containing all non-nil pointers from
// the input sequence.
func RemoveNils[T any](seq iter.Seq[*T]) iter.Seq[*T] { return Remove(seq, isNil) }

// RemoveZeros returns a sequence containing all non-zero values from
// the input sequence.
func RemoveZeros[T comparable](seq iter.Seq[T]) iter.Seq[T] { return Remove(seq, isZero) }

// RemoveErrors returns a sequence containing only the values from
// pairs where the error is nil.
func RemoveErrors[T any](seq iter.Seq2[T, error]) iter.Seq[T] { return First(Remove2(seq, isError2)) }

// KeepErrors returns a sequence containing only the non-nil errors
// from the input sequence.
func KeepErrors(seq iter.Seq[error]) iter.Seq[error] { return Keep(seq, isError) }

// KeepOk returns a sequence containing only the values from pairs
// where the boolean is true.
func KeepOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T] { return First(Keep2(seq, isOk)) }

// WhileOk returns a sequence that yields values from pairs as long as
// the boolean is true.
func WhileOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T] { return First(While2(seq, isOk)) }

// WhileSuccess returns a sequence that yields values from pairs as
// long as the error is nil.
func WhileSuccess[T any](seq iter.Seq2[T, error]) iter.Seq[T] { return First(While2(seq, isSuccess2)) }

// UntilNil returns a sequence of dereferenced values from the input
// sequence of pointers, stopping when a nil pointer is encountered.
func UntilNil[T any](seq iter.Seq[*T]) iter.Seq[T] { return Deref(Until(seq, isNil)) }

// UntilError returns a sequence of values from pairs, stopping when a
// non-nil error is encountered.
func UntilError[T any](seq iter.Seq2[T, error]) iter.Seq[T] { return First(Until2(seq, isError2)) }

// Until returns a sequence that yields elements from the input
// sequence until the predicate prd returns true.
func Until[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] { return While(seq, notf(prd)) }

// Until2 returns a iterator that yields pairs from the input iterator
// until the predicate is returns true.
func Until2[A, B any](seq iter.Seq2[A, B], is func(A, B) bool) iter.Seq2[A, B] {
	return While2(seq, notf2(is))
}

// While returns a sequence that yields elements from the input
// sequence as long as the predicate prd returns true.
func While[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			switch {
			case prd(value) && yield(value):
				continue
			default:
				return
			}
		}
	}
}

// While2 returns a iterator that yields pairs from the input iterator
// as long as the predicate prd returns true.
func While2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			switch {
			case prd(key, value) && yield(key, value):
				continue
			default:
				return
			}
		}
	}
}

// Keep returns a sequence containing only the elements from the input
// sequence that satisfy the predicate prd.
func Keep[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if prd(value) && !yield(value) {
				return
			}
		}
	}
}

// Keep2 returns a iterator containing only the pairs from the input
// iterator that satisfy the predicate prd.
func Keep2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			if prd(key, value) && !yield(key, value) {
				return
			}
		}
	}
}

// Shard splits the input sequence into num separate
// sequences. Elements are distributed.
func Shard[T any](ctx context.Context, num int, seq iter.Seq[T]) iter.Seq[iter.Seq[T]] {
	return GenerateOk(repeat(num, curry2(Channel, ctx, Pipe(ctx, seq))))
}

// WithBuffer maintains a buffer of items read from the source
// iterator, waiting for downstream consumers of the output iterator,
// to consume them.
func WithBuffer[T any](ctx context.Context, seq iter.Seq[T], size int) iter.Seq[T] {
	return func(yield func(T) bool) {
		sink := make(chan T, size)

		go func() { defer close(sink); flushTo(ctx, seq, sink) }()

		flush(Channel(ctx, sink), yield)
	}
}

// WithHooks returns a sequence that calls before() when iteration
// starts and after() when iteration ends.  after() is called even if
// iteration stops early. Nil hooks are ignored.
func WithHooks[T any](seq iter.Seq[T], before func(), after func()) iter.Seq[T] {
	return func(yield func(T) bool) { whenop(before); defer whenop(after); flush(seq, yield) }
}

// WithSetup returns a sequence that calls setup() exactly once when
// iteration starts for the first time.
func WithSetup[T any](seq iter.Seq[T], setup func()) iter.Seq[T] {
	setup = oncewhenop(setup)
	return func(yield func(T) bool) { whenop(setup); flush(seq, yield) }
}

// WithMutex returns a sequence that synchronizes all calls to the
// underlying iterator using the provided mutex.
func WithMutex[T any](seq iter.Seq[T], mtx *sync.Mutex) iter.Seq[T] {
	next, stop := iter.Pull(seq)

	return unpull(mtxdo2(mtx, next), mtxcall(mtx, stop))
}

// WithMutex2 returns a pair sequence that synchronizes all calls to
// the underlying iterator using the provided mutex.
func WithMutex2[A, B any](seq iter.Seq2[A, B], mtx *sync.Mutex) iter.Seq2[A, B] {
	next, stop := iter.Pull2(seq)

	return unpull2(mtxdo3(mtx, next), mtxcall(mtx, stop))
}

// Remove returns a sequence containing only the elements from the
// input sequence that do NOT satisfy the predicate prd.
func Remove[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] { return Keep(seq, notf(prd)) }

// Remove2 returns a iterator containing only the pairs from the input
// iterator that do NOT satisfy the predicate prd.
func Remove2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return Keep2(seq, notf2(prd))
}

// GroupBy consumes the sequence and groups elements into a iterator
// of keys and slices of values, using the groupBy function to
// determine the key for each element.
func GroupBy[K comparable, V any](seq iter.Seq[V], groupBy func(V) K) iter.Seq2[K, []V] {
	grp := grouping(groups[K, V]{})
	Apply(seq, grp.with(groupBy))
	return grp.iter()
}

// Group consumes the iterator and groups values by their keys into a
// iterator of keys and slices of values.
func Group[K comparable, V any](seq iter.Seq2[K, V]) iter.Seq2[K, []V] {
	grp := grouping(groups[K, V]{})
	Apply2(seq, grp.add)
	return grp.iter()
}

// Unique returns a sequence containing only the first occurrence of
// each unique element from the input sequence.
func Unique[T comparable](seq iter.Seq[T]) iter.Seq[T] { return Remove(seq, seen[T]()) }

// UniqueBy returns a sequence containing only the first occurrence of
// each element from the input sequence that produces a unique key
// when passed to kfn.
func UniqueBy[K comparable, V any](seq iter.Seq[V], kfn func(V) K) iter.Seq[V] {
	return First(Remove2(With(seq, kfn), seenvalue[K, V]()))
}

// Count consumes the sequence and returns the total number of
// elements.
func Count[T any](seq iter.Seq[T]) (size int) {
	inc := counter()
	return Apply(seq, func(T) { size = inc() })
}

// Count2 consumes the iterator and returns the total number of pairs.
func Count2[A, B any](seq iter.Seq2[A, B]) (size int) {
	inc := counter()
	return Apply2(seq, func(A, B) { size = inc() })
}

// Reduce consumes the sequence and reduces it to a single value by
// repeatedly applying rfn.
func Reduce[A, B any](seq iter.Seq[A], rfn func(B, A) B) (out B) {
	for v := range seq {
		out = rfn(out, v)
	}
	return out
}

// Reduce2 consumes a sequence o pairs and reduces it to a single value by
// repeatedly applying rfn.
func Reduce2[A, B, C any](seq iter.Seq2[A, B], rfn func(C, A, B) C) (out C) {
	for a, b := range seq {
		out = rfn(out, a, b)
	}
	return out
}

// Contains returns true if the sequence contains an element equal to
// cmp.  Iteration stops as soon as a match is found.
func Contains[T comparable](seq iter.Seq[T], cmp T) (ok bool) {
	ApplyUnless(seq, func(in T) bool { ok = (cmp == in); return ok })
	return
}

// Equal returns true if the two sequences contain the same elements
// in the same order.
func Equal[T comparable](rh iter.Seq[T], lh iter.Seq[T]) bool {
	rhNext, rhStop := iter.Pull(rh)
	defer rhStop()

	lhNext, lhStop := iter.Pull(lh)
	defer lhStop()

	for {
		rhv, okr := rhNext()
		lhv, okl := lhNext()

		switch {
		case okr != okl:
			return false
		case !okr && !okl:
			return true
		case rhv != lhv:
			return false
		}
	}
}

// Zip returns a iterator that pairs elements from rh and
// lh. Iteration stops when either sequence is exhausted.
func Zip[A, B any](rh iter.Seq[A], lh iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		rhNext, rhStop := iter.Pull(rh)
		defer rhStop()

		lhNext, lhStop := iter.Pull(lh)
		defer lhStop()

		for {
			vr, okr := rhNext()
			if !okr {
				return
			}

			vl, okl := lhNext()
			if !okl {
				return
			}

			if !yield(vr, vl) {
				return
			}
		}
	}
}

// SortBy consumes the sequence, sorts it based on the keys produced
// by cf, and returns a new sequence of the sorted elements.
func SortBy[K cmp.Ordered, T any](seq iter.Seq[T], cf func(T) K) iter.Seq[T] {
	return slices.Values(slices.SortedFunc(seq, toCmp(cf)))
}

// SortBy2 consumes the iterator, sorts it based on the keys produced
// by cf, and returns a new iterator of the sorted pairs.
func SortBy2[K cmp.Ordered, A, B any](seq iter.Seq2[A, B], cf func(A, B) K) iter.Seq2[A, B] {
	return KVsplit(Slice(slices.SortedFunc(KVjoin(seq), toCmp2(cf))))
}

// ReadLines returns a sequence of strings from the reader, stopping
// at the first error.
func ReadLines(reader io.Reader) iter.Seq[string] { return UntilError(ReadLinesErr(reader)) }

// ReadLinesErr returns a iterator of strings and errors from the
// reader. It yields each line with a nil error, and finally yields an
// empty string and the scanner's error.
func ReadLinesErr(reader io.Reader) iter.Seq2[string, error] {
	scanner := bufio.NewScanner(reader)
	return func(yield func(string, error) bool) {
		for scanner.Scan() {
			if !yield(scanner.Text(), nil) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			yield("", scanner.Err())
		}
	}
}

// AsGenerator provides in inverse of the GenerateOk operation: the
// function will yield values. When the boolean "ok" value is false
// the sequence has been exhausted.
func AsGenerator[T any](seq iter.Seq[T]) func(context.Context) (T, bool) {
	var (
		once   sync.Once
		ch     chan T
		cancel context.CancelFunc
	)

	op := func(ctx context.Context) {
		ch = make(chan T)
		go func() {
			defer close(ch)
			defer cancel()
			for item := range seq {
				if !sendTo(ctx, item, ch) {
					return
				}
			}
		}()
	}

	return func(ctx context.Context) (out T, ok bool) {
		once.Do(func() {
			ctx, cancel = context.WithCancel(ctx)
			op(ctx)
		})
		out, ok = recieveFrom(ctx, ch)
		whencall(!ok, cancel)
		return
	}
}

// Resolve returns a sequence that lazily executes each function in the
// input sequence and yields the results. Functions are only called
// during iteration.
func Resolve[T any, F ~func() T](seq iter.Seq[F]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for operation := range seq {
			if !yield(operation()) {
				return
			}
		}
	}
}

// Resolve2 returns a pair sequence that lazily executes each function in
// the input sequence and yields the result pairs. Functions are only
// called during iteration.
func Resolve2[A, B any, F ~func() (A, B)](seq iter.Seq[F]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for operation := range seq {
			if !yield(operation()) {
				return
			}
		}
	}
}

// ResolveWrap returns a sequence that lazily executes each function in
// the input sequence with the provided argument and yields the
// results. Functions are only called during iteration.
func ResolveWrap[A, B any, F ~func(A) B](seq iter.Seq[F], wrapping A) iter.Seq[B] {
	return func(yield func(B) bool) {
		for operation := range seq {
			if !yield(operation(wrapping)) {
				return
			}
		}
	}
}

// ResolveWrap2 returns a pair sequence that lazily executes each
// function in the input sequence with the provided argument and yields
// the result pairs. Functions are only called during iteration.
func ResolveWrap2[A, B, C any, F ~func(A) (B, C)](seq iter.Seq[F], wrapping A) iter.Seq2[B, C] {
	return func(yield func(B, C) bool) {
		for operation := range seq {
			if !yield(operation(wrapping)) {
				return
			}
		}
	}
}
