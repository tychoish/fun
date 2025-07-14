package itertool

import "github.com/tychoish/fun"

type StreamTransformer[T any, O any] interface {
	With(fun.Converter[T, O]) *fun.Stream[O]
}

type transformerImpl[T any, O any] struct{ st *fun.Stream[T] }

func (ti *transFormerImp[T, O]) With(conv fun.Converter[T, O]) *fun.Stream[O] {
	return conv.Stream(ti.st)
}

func TransformStream[T any, O any](st *fun.Stream[T]) StreamTransformer[T, O] {
	return &transformerImpl{strm: st}
}
