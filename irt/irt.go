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
	"sync/atomic"
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

		s = make([]T, 0, max(0, slices.Max(args)))
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
	out := make([]T, 0, min(n, 1024))
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
	var withStr string
	withStr = string(with)

	for str := range seq {
		if buf.Len() > lastSize {
			buf.WriteString(withStr)
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

// Zero returns a sequence that never yeilds any elements.
func Zero[T any]() iter.Seq[T] { return func(_ func(T) bool) {} }

// Zero2 returns a two element sequence that never yields any elements.
func Zero2[A, B any]() iter.Seq2[A, B] { return func(_ func(A, B) bool) {} }

// OrEmpty returns a noop (empty) sequence if the input sequence is
// nil, and returns the sequence otherwise.
func OrEmpty[T any](seq iter.Seq[T]) iter.Seq[T] { return ifelsedo(seq != nil, seq, Zero[T]) }

// OrEmpty2 returns a noop (empty) sequence if the input sequence is
// nil, and returns the sequence otherwise.
func OrEmpty2[A, B any](seq iter.Seq2[A, B]) iter.Seq2[A, B] {
	return ifelsedo(seq != nil, seq, Zero2[A, B])
}

// Mutable takes a slice, and returns an iterator over pointers to the
// elements in the slice. Mutations to elements during iteration
// impact the underlying slice, and may be observable outside of the
// scope of this iterator.
func Mutable[T any, S ~[]T](sl S) iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for idx := range sl {
			if !yield(&sl[idx]) {
				return
			}
		}
	}
}

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

// Reverse collects the elements from an iterator and returns an
// iterator in reverse order. The contents of the iterator must be
// materialized first.
func Reverse[T any](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) { Flush(Second(slices.Backward(Collect(seq))), yield) }
}

// Monotonic returns an infinite sequence of integers starting from 1.
//
// The sequence is re-iterable: each iteration starts again from 1.
func Monotonic() iter.Seq[int] { return MonotonicFrom(1) }

// MonotonicFrom returns an infinite sequence of integers starting
// from start. Each iteration starts again from start.
func MonotonicFrom[T ~int](start T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for next := start; yield(next); next++ {
			continue
		}
	}
}

// Range returns a sequence of integers from start to end (inclusive).
// The sequence is re-iterable, and end may be the maximum int value.
func Range[T ~int](start T, end T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for next := start; next <= end; next++ {
			if !yield(next) || next == end {
				return
			}
		}
	}
}

// Index returns a iterator where each element from the input sequence
// is paired with its 0-based index. Each iteration restarts at 0.
func Index[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		idx := 0
		for value := range seq {
			if !yield(idx, value) {
				return
			}
			idx++
		}
	}
}

// Index2 returns a pair iterator where each element from the input
// sequence is paired with its 0-based index.
func Index2[A, B any](seq iter.Seq2[A, B]) iter.Seq2[int, KV[A, B]] {
	return Index(KVjoin(seq))
}

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

// Initial returns the first value from the sequence and true.  If
// the sequence is empty, it returns the zero value and false.
func Initial[T any](seq iter.Seq[T]) (zero T, ok bool) {
	for value := range seq {
		return value, true
	}
	return
}

// Initial2 returns the first pair of values from the iterator and
// true.  If the sequence is empty, it returns zero values and false.
func Initial2[A, B any](seq iter.Seq2[A, B]) (azero A, bzero B, ok bool) {
	for k, v := range seq {
		return k, v, true
	}
	return
}

// Final returns the last value from the sequence and true.  If the
// sequence is empty, it returns the zero value and false.
func Final[T any](seq iter.Seq[T]) (out T, ok bool) {
	for value := range seq {
		out = value
		ok = true
	}
	return
}

// Final2 returns the last pair of values from the iterator and true.
// If the sequence is empty, it returns zero values and false.
func Final2[A, B any](seq iter.Seq2[A, B]) (aout A, bout B, ok bool) {
	for a, b := range seq {
		aout = a
		bout = b
		ok = true
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

// Cast returns a sequence of U by performing a type assertion on each
// element of seq. Because all elements of a sequence share the same
// type, a failed assertion means no element can satisfy the cast, so
// iteration stops immediately on the first failure.
func Cast[T, U any](seq iter.Seq[T]) iter.Seq[U] {
	return func(yield func(U) bool) {
		for value := range seq {
			if out, ok := any(value).(U); !ok || !yield(out) {
				return
			}
		}
	}
}

// Cast2 returns an iterator of (C, D) pairs by performing type
// assertions on the keys and values of seq independently. A failed
// key assertion stops iteration immediately; a failed value assertion
// likewise stops iteration immediately. Because all keys share one
// type and all values share another, a single failure predicts all
// subsequent failures for that position.
func Cast2[A, B, C, D any](seq iter.Seq2[A, B]) iter.Seq2[C, D] {
	return func(yield func(C, D) bool) {
		for key, value := range seq {
			if outKey, ok := any(key).(C); !ok {
				return
			} else if outVal, ok := any(value).(D); !ok || !yield(outKey, outVal) {
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

		Flush2(seq, yield)
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

		Flush2(seq, yield)
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

		Flush(seq, yield)
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

		Flush(seq, yield)
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
func ForEachWhile[T any, OP ~func(T) bool](seq iter.Seq[T], op OP) iter.Seq[T] {
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
//
// The producer goroutine starts immediately and, because Pipe returns
// a plain channel, cannot observe a consumer that stops reading: if
// you stop receiving before the channel is closed you must cancel ctx,
// or the producer goroutine blocks forever.
//
// Deprecated: Pipe offers no way to release the producer other than
// cancelling ctx. Use AsChannel, which returns a stop function, or
// WithBuffer, which releases its producer when the consumer stops
// iterating.
func Pipe[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	return opwithstart(opwithch(func(ch chan T) { seqToChan(ctx, seq, ch) }))
}

// AsChannel returns an unbuffered channel that receives all elements
// from the input sequence, and a stop function. The channel is closed
// when the sequence is exhausted, when ctx is canceled, or when stop
// is called, whichever happens first.
//
// The producer goroutine starts immediately. Calling stop (which is
// safe to call any number of times, from any goroutine, including
// after the channel has closed) releases the producer even if the
// channel is never read. Callers that do not read the channel to
// completion should call stop, typically with defer.
func AsChannel[T any](ctx context.Context, seq iter.Seq[T]) (<-chan T, func()) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan T)
	go func() {
		defer close(ch)
		defer cancel()
		flushTo(ctx, seq, ch)
	}()
	return ch, cancel
}

// Sink wraps yield so that concurrent workers can share it safely:
// calls are serialized by an internal mutex, and once yield returns
// false, every subsequent call - from any worker, forever after - short-
// circuits to false instead of invoking yield again, satisfying the
// range-over-func rule that yield must never be called after it has
// returned false. Pair with WithMutex (or another input-side lock) to
// merge multiple producers back into a single consumer, as Pool and
// Pool3 do.
func Sink[T any](yield func(T) bool) func(T) bool {
	var mtx sync.Mutex
	var done bool
	return func(v T) bool {
		mtx.Lock()
		defer mtx.Unlock()
		if done {
			return false
		}
		// stay "done" if yield panics, so a recovered panic can never
		// lead to yield being called again.
		done = true
		if !yield(v) {
			return false
		}
		done = false
		return true
	}
}

// Sink2 is the iter.Seq2 counterpart to Sink: it wraps a pair yield
// function so that concurrent workers can share it safely.
func Sink2[A, B any](yield func(A, B) bool) func(A, B) bool {
	var mtx sync.Mutex
	var done bool
	return func(a A, b B) bool {
		mtx.Lock()
		defer mtx.Unlock()
		if done {
			return false
		}
		done = true
		if !yield(a, b) {
			return false
		}
		done = false
		return true
	}
}

// Pool iterates seq and applies op to each element using a pool of
// num goroutines, merging the results into a single output sequence. op
// runs outside the input lock, so it executes in parallel across
// workers; only pulling the raw element from seq is serialized. Output
// order is not preserved.
//
// Workers run on their own goroutines, but a panic in op, in the loop
// body consuming the output, or in seq is recovered and re-raised on
// the goroutine that is iterating the result, after the workers stop.
// The loop body is invoked by workers (serialized), not necessarily on
// the iterating goroutine.
//
// ctx is checked before every pull, so a canceled context stops the
// pool without consuming further input; cancellation is silent. ctx
// cannot interrupt a pull that is already blocked: seq must itself
// honor ctx (for example by being built from a channel with Channel
// and the same ctx) if it can block. num is clamped to at least 1.
func Pool[A, B any, OP ~func(A) B](ctx context.Context, num int, seq iter.Seq[A], op OP) iter.Seq[B] {
	num = max(num, 1)
	return func(yield func(B) bool) {
		push := Sink(yield)
		poolRun(ctx, num, seq, func(a A) bool { return push(op(a)) })
	}
}

// poolRun pulls from seq under a lock and hands each element to work
// on one of num worker goroutines. It returns once all workers stop;
// the first panic raised by a worker (or by seq) is re-raised on the
// calling goroutine.
func poolRun[A any](ctx context.Context, num int, seq iter.Seq[A], work func(A) bool) {
	if ctx.Err() != nil {
		return
	}

	var (
		mtx     sync.Mutex
		halt    atomic.Bool
		failure atomic.Pointer[any]
	)
	next, stop := iter.Pull(seq)
	pull := func() (A, bool) {
		mtx.Lock()
		defer mtx.Unlock()
		return next()
	}

	wgdo(num, func() {
		defer func() {
			if r := recover(); r != nil {
				failure.CompareAndSwap(nil, &r)
				halt.Store(true)
			}
		}()
		for ctx.Err() == nil && !halt.Load() {
			a, ok := pull()
			if !ok || !work(a) {
				halt.Store(true)
				return
			}
		}
	})

	mtx.Lock()
	stop()
	mtx.Unlock()

	if r := failure.Load(); r != nil {
		panic(*r)
	}
}

// Pool2 is the iter.Seq2 counterpart to Pool: it iterates seq and
// applies op to each pair using a pool of num goroutines, merging the
// results into a single output pair sequence.
func Pool2[A, B, C, D any, OP ~func(A, B) (C, D)](ctx context.Context, num int, seq iter.Seq2[A, B], op OP) iter.Seq2[C, D] {
	return KVsplit(Pool(ctx, num, KVjoin(seq), func(kv KV[A, B]) KV[C, D] { return MakeKV(op(kv.Key, kv.Value)) }))
}

// Pool3 is the pooled counterpart to With2: it iterates seq and
// applies op to each element using a pool of num goroutines, merging
// the resulting pairs into a single output pair sequence. op runs
// outside the input lock, so it executes in parallel across workers;
// only pulling the raw element from seq is serialized. num is clamped
// to at least 1. Panic propagation and context handling are the same
// as for Pool: seq must honor ctx if it can block.
func Pool3[A, B, C any, OP ~func(A) (B, C)](ctx context.Context, num int, seq iter.Seq[A], op OP) iter.Seq2[B, C] {
	num = max(num, 1)
	return func(yield func(B, C) bool) {
		push := Sink2(yield)
		poolRun(ctx, num, seq, func(a A) bool { return push(op(a)) })
	}
}

// Chunk returns a sequence of sequences, where each inner sequence
// contains at most num elements from the input sequence. If num <= 0,
// the sequence is empty.
func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] {
	return func(yield func(iter.Seq[T]) bool) {
		if num <= 0 {
			return
		}
		next, stop := iter.Pull(seq)
		defer stop()

		for {
			// peek: only emit a chunk if it has at least one element.
			first, ok := next()
			if !ok {
				return
			}

			var (
				pulled    = 1
				pending   = true
				exhausted bool
			)

			inner := func(yield func(T) bool) {
				for !exhausted {
					value := first
					if pending {
						pending = false
					} else if pulled < num {
						if value, ok = next(); !ok {
							exhausted = true
							return
						}
						pulled++
					} else {
						return
					}
					if !yield(value) {
						return
					}
				}
			}

			if !yield(inner) {
				return
			}

			// the consumer may not have drained the chunk: skip the
			// remainder so the next chunk starts at the boundary.
			for !exhausted && pulled < num {
				if _, ok = next(); !ok {
					return
				}
				pulled++
			}
			if exhausted {
				return
			}
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
// sequence. Like Chain, nil inner sequences are skipped.
func Chain2[A, B any](seq iter.Seq[iter.Seq2[A, B]]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for inner := range seq {
			if inner == nil {
				continue
			}
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
func Until[T any, OP ~func(T) bool](seq iter.Seq[T], prd OP) iter.Seq[T] {
	return While(seq, notf(prd))
}

// Until2 returns a iterator that yields pairs from the input iterator
// until the predicate is returns true.
func Until2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], is OP) iter.Seq2[A, B] {
	return While2(seq, notf2(is))
}

// While returns a sequence that yields elements from the input
// sequence as long as the predicate prd returns true.
func While[T any, OP ~func(T) bool](seq iter.Seq[T], prd OP) iter.Seq[T] {
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
func While2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], prd OP) iter.Seq2[A, B] {
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
func Keep[T any, OP ~func(T) bool](seq iter.Seq[T], prd OP) iter.Seq[T] {
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
func Keep2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], prd OP) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			if prd(key, value) && !yield(key, value) {
				return
			}
		}
	}
}

// Shard splits the input sequence into num separate sequences. All num
// "shards" alias the same mutex-guarded shared iterator, so elements
// are distributed dynamically across whichever shard is consumed
// fastest, not as a static partition. A shard that stops early does
// not affect the others: the shared iterator is released when the
// input is exhausted, when every shard has finished, or when ctx is
// canceled, even if some shards never started or never finish. num is
// clamped to at least 1. Once ctx is canceled, every shard stops
// yielding new elements, the same way Pool's workers do.
func Shard[T any](ctx context.Context, num int, seq iter.Seq[T]) iter.Seq[iter.Seq[T]] {
	num = max(num, 1)
	return func(yield func(iter.Seq[T]) bool) {
		var (
			mtx      sync.Mutex
			next     func() (T, bool)
			stop     func()
			unwatch  func() bool
			finished = make([]bool, num)
			remain   = num
			closed   bool
		)

		// the shared pull iterator starts lazily, so shards that are
		// never iterated cost nothing, and it is stopped only when the
		// input is exhausted or every shard has finished: one shard
		// ending early must not end the others.
		release := func() { // callers hold mtx
			if closed {
				return
			}
			closed = true
			if unwatch != nil {
				unwatch()
			}
			if stop != nil {
				stop()
			}
		}
		pull := func() (out T, ok bool) {
			mtx.Lock()
			defer mtx.Unlock()
			if closed {
				return out, false
			}
			if next == nil {
				next, stop = iter.Pull(seq)
				// cancellation releases the pull even if some shards
				// never start or never finish; it is serialized with
				// next under mtx.
				unwatch = context.AfterFunc(ctx, func() {
					mtx.Lock()
					defer mtx.Unlock()
					release()
				})
			}
			if out, ok = next(); !ok {
				release()
			}
			return out, ok
		}
		finish := func(idx int) {
			mtx.Lock()
			defer mtx.Unlock()
			if finished[idx] {
				return
			}
			finished[idx] = true
			if remain--; remain == 0 {
				release()
			}
		}

		for idx := range num {
			shard := func(yield func(T) bool) {
				defer finish(idx)
				for ctx.Err() == nil {
					value, ok := pull()
					if !ok || !yield(value) {
						return
					}
				}
			}
			if !yield(shard) {
				return
			}
		}
	}
}

// Shard2 is the iter.Seq2 counterpart to Shard: it splits the input
// pair sequence into num separate pair sequences. Pairs are
// distributed.
func Shard2[A, B any](ctx context.Context, num int, seq iter.Seq2[A, B]) iter.Seq[iter.Seq2[A, B]] {
	return Convert(Shard(ctx, num, KVjoin(seq)), KVsplit)
}

// WithBuffer maintains a buffer of items read from the source
// iterator, waiting for downstream consumers of the output iterator,
// to consume them. The producer goroutine is released when the
// consumer stops iterating, as well as when ctx is canceled.
func WithBuffer[T any](ctx context.Context, seq iter.Seq[T], size int) iter.Seq[T] {
	return func(yield func(T) bool) {
		// cancel when the consumer stops (early break or exhaustion) so
		// the producer goroutine is never left blocked on a full buffer.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sink := make(chan T, max(size, 0))

		go func() { defer close(sink); flushTo(ctx, seq, sink) }()

		Flush(Channel(ctx, sink), yield)
	}
}

// WithHooks returns a sequence that calls before() when iteration
// starts and after() when iteration ends.  after() is called even if
// iteration stops early. Nil hooks are ignored.
func WithHooks[T any](seq iter.Seq[T], before func(), after func()) iter.Seq[T] {
	return func(yield func(T) bool) { whenop(before); defer whenop(after); Flush(seq, yield) }
}

// WithSetup returns a sequence that calls setup() exactly once when
// iteration starts for the first time.
func WithSetup[T any](seq iter.Seq[T], setup func()) iter.Seq[T] {
	setup = oncewhenop(setup)
	return func(yield func(T) bool) { whenop(setup); Flush(seq, yield) }
}

// WithMutex returns a sequence that synchronizes all calls to the
// underlying iterator using the provided mutex.
//
// All the WithMutex, WithRMutex and WithWMutex variants (and their
// pair forms) are single-use: the underlying iter.Pull is created
// eagerly and is stopped when the first iteration of the result ends,
// so any later iteration yields nothing. Concurrent iterations share
// the one underlying iterator.
func WithMutex[T any](seq iter.Seq[T], mtx *sync.Mutex) iter.Seq[T] {
	next, stop := iter.Pull(seq)

	return unpull(mtxdo2(mtx, next), mtxcall(mtx, stop))
}

// WithRMutex returns a sequence that synchronizes all calls to the
// underlying iterator using the provided readers/writer mutex. Locks
// the mutex for reading (shared) while advancing the iterator.
func WithRMutex[T any](seq iter.Seq[T], mtx *sync.RWMutex) iter.Seq[T] {
	next, stop := iter.Pull(seq)

	return unpull(mtxdor(mtx, next), mtxcallr(mtx, stop))
}

// WithWMutex returns a sequence that synchronizes all calls to the
// underlying iterator using the provided readers/writer mutex. Locks
// the mutex for writing (exclusive) while advancing the iterator.
func WithWMutex[T any](seq iter.Seq[T], mtx *sync.RWMutex) iter.Seq[T] {
	next, stop := iter.Pull(seq)

	return unpull(mtxdo2w(mtx, next), mtxcallw(mtx, stop))
}

// WithMutex2 returns a pair sequence that synchronizes all calls to
// the underlying iterator using the provided mutex.
func WithMutex2[A, B any](seq iter.Seq2[A, B], mtx *sync.Mutex) iter.Seq2[A, B] {
	next, stop := iter.Pull2(seq)

	return unpull2(mtxdo3(mtx, next), mtxcall(mtx, stop))
}

// WithRMutex2 returns a pair sequence that synchronizes all calls to
// the underlying iterator using the provided readers/writer
// mutex. Locks the mutex for reading (shared) while advancing the iterator.
func WithRMutex2[A, B any](seq iter.Seq2[A, B], mtx *sync.RWMutex) iter.Seq2[A, B] {
	next, stop := iter.Pull2(seq)

	return unpull2(mtxdo3r(mtx, next), mtxcallr(mtx, stop))
}

// WithWMutex2 returns a pair sequence that synchronizes all calls to
// the underlying iterator using the provided readers/writer
// mutex. Locks the mutex for writing (exclusive) while advancing the
// iterator.
func WithWMutex2[A, B any](seq iter.Seq2[A, B], mtx *sync.RWMutex) iter.Seq2[A, B] {
	next, stop := iter.Pull2(seq)

	return unpull2(mtxdo3w(mtx, next), mtxcallw(mtx, stop))
}

// Remove returns a sequence containing only the elements from the
// input sequence that do NOT satisfy the predicate prd.
func Remove[T any, OP ~func(T) bool](seq iter.Seq[T], prd OP) iter.Seq[T] {
	return Keep(seq, notf(prd))
}

// RemoveValue returns a sequence with all values equal to the provided values removed.
func RemoveValue[T comparable](seq iter.Seq[T], to T) iter.Seq[T] { return Remove(seq, equalf(to)) }

// Remove2 returns a iterator containing only the pairs from the input
// iterator that do NOT satisfy the predicate prd.
func Remove2[A, B any, OP ~func(A, B) bool](seq iter.Seq2[A, B], prd OP) iter.Seq2[A, B] {
	return Keep2(seq, notf2(prd))
}

// GroupBy consumes the sequence and groups elements into a iterator
// of keys and slices of values, using the groupBy function to
// determine the key for each element.
func GroupBy[K comparable, V any, OP ~func(V) K](seq iter.Seq[V], groupBy OP) iter.Seq2[K, []V] {
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
func Unique[T comparable](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) { Flush(Remove(seq, seen[T]()), yield) }
}

// UniqueBy returns a sequence containing only the first occurrence of
// each element from the input sequence that produces a unique key
// when passed to kfn.
func UniqueBy[K comparable, V any, OP ~func(V) K](seq iter.Seq[V], kfn OP) iter.Seq[V] {
	return func(yield func(V) bool) { Flush(First(Remove2(With(seq, kfn), seenvalue[K, V]())), yield) }
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
func Reduce[A, B any, OP ~func(B, A) B](seq iter.Seq[A], rfn OP) (out B) {
	for v := range seq {
		out = rfn(out, v)
	}
	return out
}

// Reduce2 consumes a sequence o pairs and reduces it to a single value by
// repeatedly applying rfn.
func Reduce2[A, B, C any, OP ~func(C, A, B) C](seq iter.Seq2[A, B], rfn OP) (out C) {
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

// HasValues returns true if any items are found in the iterator. Use
// this, potentially in combination with 'Keep()' and 'Remove' to
// implement arbitrary "contains"-type expressions.
func HasValues[T any](seq iter.Seq[T]) bool {
	for range seq {
		return true
	}
	return false
}

// HasValues2 returns true if any items are found in the iterator. Use
// this, potentially in combination with 'Keep()' and 'Remove' to
// implement arbitrary "contains"-type expressions.
func HasValues2[A, B any](seq iter.Seq2[A, B]) bool {
	for range seq {
		return true
	}
	return false
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

// Sort collects the elements in an iterator, sorts them, and then
// returns an iterator in ascending order.
func Sort[T cmp.Ordered](seq iter.Seq[T]) iter.Seq[T] { return slices.Values(slices.Sorted(seq)) }

// Sort2 collects the KV pairs from the iterator, sorts them by
// comparing the first value then the second value as a tiebreaker,
// and returns an iterator in ascending order.
func Sort2[A, B cmp.Ordered](seq iter.Seq2[A, B]) iter.Seq2[A, B] {
	return KVsplit(Slice(slices.SortedFunc(KVjoin(seq), KVcmp)))
}

// Sort1 collects the KV pairs from the iterator, sorts them by
// comparing only the first value, and returns an iterator in
// ascending order.
func Sort1[A cmp.Ordered, B any](seq iter.Seq2[A, B]) iter.Seq2[A, B] {
	return KVsplit(Slice(slices.SortedFunc(KVjoin(seq), KVcmpFirst)))
}

// SortBy consumes the sequence, sorts it based on the keys produced
// by the comparison function, cf, and returns a new sequence of the
// sorted elements.
func SortBy[K cmp.Ordered, T any, OP ~func(T) K](seq iter.Seq[T], cf OP) iter.Seq[T] {
	return slices.Values(slices.SortedFunc(seq, toCmp(cf)))
}

// SortBy2 consumes the iterator, sorts it based on the keys produced
// by cf, and returns a new iterator of the sorted pairs.
func SortBy2[K cmp.Ordered, A, B any, OP ~func(A, B) K](seq iter.Seq2[A, B], cf OP) iter.Seq2[A, B] {
	return KVsplit(Slice(slices.SortedFunc(KVjoin(seq), toCmp2(cf))))
}

// SortFunc consumes the sequence, sorts it using the provided
// cmp.Compare-style comparison function, and returns a new sorted
// sequence.
func SortFunc[T any, OP ~func(T, T) int](seq iter.Seq[T], cf OP) iter.Seq[T] {
	return slices.Values(slices.SortedFunc(seq, cf))
}

// SortFunc2 consumes the iterator, sorts its pairs using the provided
// cmp.Compare-style comparison function, and returns a new sorted
// iterator.
func SortFunc2[A, B any, OP ~func(A, B, A, B) int](seq iter.Seq2[A, B], cf OP) iter.Seq2[A, B] {
	return KVsplit(Slice(slices.SortedFunc(KVjoin(seq), func(l, r KV[A, B]) int {
		return cf(l.Key, l.Value, r.Key, r.Value)
	})))
}

// Flush iterates seq, passing each value to yield. It returns false
// if yield signals early termination, and true if the sequence is
// exhausted normally. Flush is the core building block for composing
// iterators: use it inside an iter.Seq body to forward elements from
// one sequence into another yield function.
func Flush[T any](seq iter.Seq[T], yield func(T) bool) bool {
	for value := range seq {
		if !yield(value) {
			return false
		}
	}
	return true
}

// Flush2 iterates seq, passing each key-value pair to yield. It
// returns false if yield signals early termination, and true if the
// sequence is exhausted normally. Flush2 is the iter.Seq2 counterpart
// to Flush: use it inside an iter.Seq2 body to forward pairs from one
// sequence into another yield function.
func Flush2[A, B any](seq iter.Seq2[A, B], yield func(A, B) bool) bool {
	for key, value := range seq {
		if !yield(key, value) {
			return false
		}
	}
	return true
}

// ReadLines returns a sequence of strings from the reader, with one
// item for every line, stopping when the reader is exhausted at the
// first error.
func ReadLines(reader io.Reader) iter.Seq[string] { return UntilError(ReadLinesErr(reader)) }

// ReadWords returns a sequence of strings from the reader, with one
// item for every word, stopping at the first error.
func ReadWords(reader io.Reader) iter.Seq[string] { return UntilError(ReadWordsErr(reader)) }

// ReadLinesErr returns a iterator of strings and errors from the
// reader. It yields each line with a nil error, and finally yields an
// empty string and the scanner's error.
func ReadLinesErr(reader io.Reader) iter.Seq2[string, error] {
	return fromReader(reader, bufio.ScanLines)
}

// ReadWordsErr returns a iterator of strings and errors from the
// reader. It yields each line with a nil error, and finally yields an
// empty string and the scanner's error.
func ReadWordsErr(reader io.Reader) iter.Seq2[string, error] {
	return fromReader(reader, bufio.ScanWords)
}

func fromReader(reader io.Reader, splitter bufio.SplitFunc) iter.Seq2[string, error] {
	scanner := bufio.NewScanner(reader)
	scanner.Split(splitter)
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
//
// The sequence is consumed by a background goroutine, started on the
// first (non-cancelled) call with a context derived from that call's
// ctx; a nil ctx is treated as context.Background. AsGenerator cannot
// detect that the caller has stopped calling the function, so callers
// that abandon the generator before exhaustion must cancel the ctx
// they passed to the first call to release the goroutine. Cancelling
// the ctx of a later call only aborts that call; the stream continues.
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
		if ctx == nil {
			ctx = context.Background()
		}
		// a cancelled call neither starts the producer (it would be
		// born dead) nor stops a running one: the cancellation is
		// local to this call.
		if ctx.Err() != nil {
			return
		}
		once.Do(func() {
			var gctx context.Context
			gctx, cancel = context.WithCancel(ctx)
			op(gctx)
		})
		select {
		case <-ctx.Done():
		case out, ok = <-ch:
			whencall(!ok, cancel)
		}
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

// Flatten a sequence/mapping of key/value pairs where the value is a
// sequence, into a sequence of pairs to single values.
func Flatten[K, V any, S iter.Seq[V]](seq iter.Seq2[K, S]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, seqVals := range seq {
			for value := range seqVals {
				if !yield(key, value) {
					return
				}
			}
		}
	}
}

// FlattenSlice a sequence/mapping of key/value pairs where the value is a
// slice of values, into a sequence of pairs to single values.
func FlattenSlice[K, V any, S ~[]V](seq iter.Seq2[K, S]) iter.Seq2[K, V] {
	return Flatten(Convert2(seq, kvslice2seq))
}

// ReverseMapping expands a mapping of keys to slices of values to a
// mapping of values to keys. Values may appear more than once in the
// output sequence.
func ReverseMapping[A, B any](seq iter.Seq2[A, []B]) iter.Seq2[B, A] {
	return func(yield func(B, A) bool) {
		for value, keys := range seq {
			for key := range Slice(keys) {
				if !yield(key, value) {
					return
				}
			}
		}
	}
}
