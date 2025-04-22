package erc

import (
	"bytes"
	"errors"
	"iter"
)

// AsList takes an error and converts it to a list if possible, if
// the error is an erc.List then this is a passthrough, and errors
// that implement {Unwind() []error} or {Unwrap() []error}, though
// preferring Unwind, are added individually to the list.
//
// For errors that provide the Unwind/Unwrap method, if these methods
// return empty slices of errors, then AsList will return nil.
func AsList(err error) *List {
	switch et := err.(type) {
	case nil:
		return nil
	case *List:
		return et
	case interface{ Unwind() []error }:
		if errs := et.Unwind(); len(errs) > 0 {
			st := &List{}
			st.Add(errs...)
			return st
		}
	case interface{ Unwrap() []error }:
		if errs := et.Unwrap(); len(errs) > 0 {
			st := &List{}
			st.Add(errs...)
			return st
		}
	default:
		st := &List{}
		st.Push(err)
		return st
	}

	return nil
}

type List struct {
	num int
	elm element
}

func (eel *List) front() *element { return eel.root().next }
func (eel *List) back() *element  { return eel.root().prev }
func (eel *List) root() *element {
	if eel.elm.next == nil {
		eel.elm.next = &eel.elm
	}
	if eel.elm.prev == nil {
		eel.elm.prev = &eel.elm
	}
	if eel.elm.list == nil {
		eel.elm.list = eel
	}
	return &eel.elm
}

func (eel *List) Error() string {
	if eel.num == 0 {
		return "<nil>"
	}

	// TODO: pool buffers.
	buf := &bytes.Buffer{}

	for elem := range eel.FIFO() {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		buf.WriteString(elem.Error())
	}

	return buf.String()
}

func (eel *List) Resolve() error { return eel.Err() }

func (eel *List) Err() error {
	switch {
	case eel == nil || eel.num == 0:
		return nil
	case eel.num == 1:
		return eel.front().Err()
	default:
		return eel
	}
}

func (eel *List) In(elm *element) bool { return elm.list == eel }
func (elm *element) In(eel *List) bool { return elm.list == eel }
func (eel *List) Len() int {
	if eel == nil {
		return 0
	}

	return eel.num
}

// Ok returns true if the Stack object contains no errors and false
// otherwise.
func (eel *List) Ok() bool { return eel == nil || eel.elm.Ok() }

// Handler provides a fn.Handler[error] typed function (though
// because ers is upstream of the root-fun package, it is not
// explicitly typed as such.) which will Add errors to the stack.
func (eel *List) Handler() func(err error) { return eel.Push }

// Future provides a fn.Future[error] typed function (though
// because ers is upstream of the root-fun package, it is not
// explicitly typed as such.) which will resolve the stack.
func (eel *List) Future() func() error { return eel.Resolve }

func (eel *List) Unwrap() error {
	if eel.Ok() {
		return nil
	}

	return eel.root().Err()
}

func (eel *List) Unwind() []error {
	out := make([]error, eel.num)

	idx := 0
	for elem := eel.front(); elem.Ok(); elem = elem.Next() {
		out[idx] = elem.err
		idx++
	}

	return out
}

func (eel *List) Is(target error) (ok bool) {
	for err := range eel.FIFO() {
		if ok = errors.Is(err, target); ok {
			break
		}
	}
	return
}

func (eel *List) As(target any) (ok bool) {
	for err := range eel.FIFO() {
		if ok = errors.As(err, target); ok {
			break
		}
	}
	return
}

func (eel *List) Push(err error) {
	switch werr := err.(type) {
	case nil:
		return
	case *List:
		for elem := werr.front(); elem.Ok(); elem = elem.Next() {
			eel.PushBack(elem.Err())
		}
	case *element:
		switch {
		case !werr.Ok() && werr.err != nil:
			eel.PushBack(werr.err)
		case werr.Ok() && !werr.In(eel):
			for elem := werr; elem.Ok(); elem = elem.Next() {
				eel.PushBack(elem.Err())
			}
		}
	case interface{ Unwind() []error }:
		eel.Add(werr.Unwind()...)
	case interface{ Unwrap() []error }:
		eel.Add(werr.Unwrap()...)
	default:
		eel.PushBack(err)
	}
}

func (eel *List) Add(errs ...error) {
	for _, err := range errs {
		eel.Push(err)
	}
}

func (eel *List) PushBack(err error) {
	if err != nil {
		eel.num++
		head := eel.root()
		elem := &element{
			list: eel,
			next: head,
			prev: head.prev,
			err:  err,
		}
		elem.next.prev = elem
		elem.prev.next = elem
	}

}

func (eel *List) PushFront(err error) {
	if err != nil {
		eel.num++
		head := eel.root()
		elem := &element{
			list: eel,
			next: head.next,
			prev: head,
			err:  err,
		}
		elem.next.prev = elem
		elem.prev.next = elem
	}
}

func (eel *List) FIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.back(); elem.Ok(); elem = elem.Previous() {
			if !yield(elem) {
				return
			}
		}
	}
}

func (eel *List) LIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.front(); elem.Ok(); elem = elem.Next() {
			if !yield(elem) {
				return
			}
		}
	}
}

type element struct {
	list *List
	next *element
	prev *element
	err  error
}

func (elm *element) Ok() bool      { return elm != nil && elm.list != nil && elm.err != nil }
func (elm *element) Err() error    { return elm.err }
func (elm *element) Error() string { return elm.err.Error() }
func (elm *element) Unwrap() error {
	if elm.Next().Ok() {
		return elm.Next()
	}

	return nil
}

func (elm *element) Is(target error) bool { return errors.Is(elm.err, target) }
func (elm *element) As(target any) bool   { return errors.As(elm.err, target) }
func (elm *element) Next() *element {
	if elm.Ok() && elm.next.Ok() {
		return elm.next
	}
	return nil
}

func (elm *element) Previous() *element {
	if elm.Ok() && elm.prev.Ok() {
		return elm.prev
	}

	return nil
}
