package irt

import (
	"cmp"
	"iter"
)

// KV is a generic pair type that holds two values of potentially different types.
type KV[A, B any] struct {
	Key   A
	Value B
}

// WithKV creates a KV by applying a function to the first value to derive the second value.
func WithKV[A, B any](a A, with func(A) B) KV[A, B] { return MakeKV(a, with(a)) }

// MakeKV creates a KV from two values.
func MakeKV[A, B any](a A, b B) KV[A, B] { return KV[A, B]{Key: a, Value: b} }

// KVargs takes a variadic sequence of KV args and returns a pair iterator.
func KVargs[A, B any](elems ...KV[A, B]) iter.Seq2[A, B] { return KVsplit(Slice(elems)) }

////////////////////////////////////////////////////////////////////////
//
// Split/Apply

// KVjoin converts an iter.Seq2 into an iter.Seq of Elem pairs.
func KVjoin[A, B any](seq iter.Seq2[A, B]) iter.Seq[KV[A, B]] { return Merge(seq, MakeKV) }

// KVsplit converts an iter.Seq of Elem pairs into an iter.Seq2.
func KVsplit[A, B any](seq iter.Seq[KV[A, B]]) iter.Seq2[A, B] { return With2(seq, elemSplit) }

// KVapply applies a function to each Elem in a sequence.
func KVapply[A, B any](seq iter.Seq[KV[A, B]], op func(A, B)) { Apply(seq, elemApply(op)) }

func elemSplit[A, B any](in KV[A, B]) (A, B)           { return in.Split() }
func elemApply[A, B any](op func(A, B)) func(KV[A, B]) { return func(e KV[A, B]) { e.Apply(op) } }

// Split returns the First and Second values of an Elem as separate return values.
func (e KV[A, B]) Split() (A, B) { return e.Key, e.Value }

// Apply calls the provided function with the First and Second values of the Elem.
func (e KV[A, B]) Apply(op func(A, B)) { op(e.Key, e.Value) }

////////////////////////////////////////////////////////////////////////
//
// Sort / Compare

// KVcmp compares two Elem values by comparing both their First and Second fields.
func KVcmp[A, B cmp.Ordered](lh, rh KV[A, B]) int { return lh.Compare(cmpf, cmpf).With(rh) }

// KVcmpFirst compares two Elem values by comparing only their First fields.
func KVcmpFirst[A cmp.Ordered, B any](lh, rh KV[A, B]) int { return lh.CompareFirst(cmpf).With(rh) }

// KVcmpSecond compares two Elem values by comparing only their Second fields.
func KVcmpSecond[A any, B cmp.Ordered](l, r KV[A, B]) int { return l.CompareSecond(cmpf).With(r) }

// Compare returns a comparator that compares both First and Second fields using the provided comparison functions.
func (e KV[A, B]) Compare(aop func(A, A) int, bop func(B, B) int) interface{ With(KV[A, B]) int } {
	return &kvcmp[A, B]{lh: e, ac: aop, bc: bop}
}

// CompareFirst returns a comparator that compares only the First field using the provided comparison function.
func (e KV[A, B]) CompareFirst(aop func(A, A) int) interface{ With(KV[A, B]) int } {
	return &kvcmp[A, B]{lh: e, ac: aop}
}

// CompareSecond returns a comparator that compares only the Second field using the provided comparison function.
func (e KV[A, B]) CompareSecond(bop func(B, B) int) interface{ With(KV[A, B]) int } {
	return &kvcmp[A, B]{lh: e, bc: bop}
}

type kvcmp[A, B any] struct {
	lh KV[A, B]
	ac func(A, A) int
	bc func(B, B) int
}

func (ec *kvcmp[A, B]) With(rh KV[A, B]) int {
	switch {
	case ec.ac == nil && ec.bc == nil:
		panic("impossible configuration")
	case ec.ac == nil:
		return ec.bc(ec.lh.Value, rh.Value)
	case ec.bc == nil:
		return ec.ac(ec.lh.Key, rh.Key)
	default:
		return cmp.Or(ec.ac(ec.lh.Key, rh.Key), ec.bc(ec.lh.Value, rh.Value))
	}
}
