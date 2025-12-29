package irt

import (
	"cmp"
	"iter"
)

type Elem[A, B any] struct {
	First  A
	Second B
}

func WithElem[A, B any](a A, with func(A) B) Elem[A, B]             { return NewElem(a, with(a)) }
func NewElem[A, B any](a A, b B) Elem[A, B]                         { return Elem[A, B]{First: a, Second: b} }
func Elems[A, B any](seq iter.Seq2[A, B]) iter.Seq[Elem[A, B]]      { return Merge(seq, NewElem) }
func ElemsSplit[A, B any](seq iter.Seq[Elem[A, B]]) iter.Seq2[A, B] { return With2(seq, elemSplit) }
func ElemsApply[A, B any](seq iter.Seq[Elem[A, B]], op func(A, B))  { Apply(seq, elemApply(op)) }
func ElemCmp[A, B cmp.Ordered](lh, rh Elem[A, B]) int               { return lh.Compare(cmpf, cmpf).With(rh) }
func ElemCmpFirst[A cmp.Ordered, B any](lh, rh Elem[A, B]) int      { return lh.CompareFirst(cmpf).With(rh) }
func ElemCmpSecond[A any, B cmp.Ordered](l, r Elem[A, B]) int       { return l.CompareSecond(cmpf).With(r) }
func (e Elem[A, B]) Split() (A, B)                                  { return e.First, e.Second }
func (e Elem[A, B]) Apply(op func(A, B))                            { op(e.First, e.Second) }
func elemSplit[A, B any](in Elem[A, B]) (A, B)                      { return in.Split() }
func elemApply[A, B any](op func(A, B)) func(Elem[A, B])            { return func(e Elem[A, B]) { e.Apply(op) } }

func (e Elem[A, B]) Compare(aop func(A, A) int, bop func(B, B) int) interface{ With(Elem[A, B]) int } {
	return &elemcmp[A, B]{lh: e, ac: aop, bc: bop}
}

func (e Elem[A, B]) CompareFirst(aop func(A, A) int) interface{ With(Elem[A, B]) int } {
	return &elemcmp[A, B]{lh: e, ac: aop}
}

func (e Elem[A, B]) CompareSecond(bop func(B, B) int) interface{ With(Elem[A, B]) int } {
	return &elemcmp[A, B]{lh: e, bc: bop}
}

type elemcmp[A, B any] struct {
	lh Elem[A, B]
	ac func(A, A) int
	bc func(B, B) int
}

func (ec *elemcmp[A, B]) With(rh Elem[A, B]) int {
	switch {
	case ec.ac == nil && ec.bc == nil:
		panic("impossible configuration")
	case ec.ac == nil:
		return ec.bc(ec.lh.Second, rh.Second)
	case ec.bc == nil:
		return ec.ac(ec.lh.First, rh.First)
	default:
		return cmp.Or(ec.ac(ec.lh.First, rh.First), ec.bc(ec.lh.Second, rh.Second))
	}
}
