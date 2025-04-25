package erc

import (
	"bytes"
	"errors"
	"iter"
)

type list struct {
	num    int
	elm    element
	filter Filter
}

func (eel *list) Front() *element { return eel.root().next }
func (eel *list) Back() *element  { return eel.root().prev }
func (eel *list) root() *element {
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

func (eel *list) Error() string {
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

func (eel *list) In(elm *element) bool { return elm.list == eel }
func (elm *element) In(eel *list) bool { return elm.list == eel }
func (eel *list) Len() int {
	if eel == nil {
		return 0
	}

	return eel.num
}

// Ok returns true if the Stack object contains no errors and false
// otherwise.
func (eel *list) Ok() bool { return eel == nil || eel.elm.Ok() }

func (eel *list) Unwrap() error {
	if eel.Ok() {
		return nil
	}

	return eel.root().Err()
}

func (eel *list) Unwind() []error {
	out := make([]error, eel.num)

	for elem, idx := eel.Front(), 0; elem.Ok(); elem, idx = elem.Next(), idx+1 {
		out[idx] = elem.err
	}

	return out
}

func (eel *list) Is(target error) (ok bool) {
	for err := range eel.LIFO() {
		if ok = errors.Is(err, target); ok {
			break
		}
	}
	return
}

func (eel *list) As(target any) (ok bool) {
	for err := range eel.LIFO() {
		if ok = errors.As(err, target); ok {
			break
		}
	}
	return
}

func (eel *list) Push(err error) {
	switch werr := eel.filter.Apply(err).(type) {
	case nil:
		return
	case *Collector:
		defer werr.with(werr.lock())
		eel.Push(&werr.list)
	case *list:
		for elem := werr.Front(); elem.Ok(); elem = elem.Next() {
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
		eel.Add(werr.Unwind())
	case interface{ Unwrap() []error }:
		eel.Add(werr.Unwrap())
	default:
		eel.PushBack(err)
	}
}

func (eel *list) Add(errs []error) {
	for _, err := range errs {
		eel.Push(err)
	}
}

func (eel *list) PushBack(err error) {
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

func (eel *list) PushFront(err error) {
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

func (eel *list) FIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.Front(); elem.Ok(); elem = elem.Next() {
			if !yield(elem) {
				return
			}
		}
	}
}

func (eel *list) LIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.Back(); elem.Ok(); elem = elem.Previous() {
			if !yield(elem) {
				return
			}
		}
	}
}

type element struct {
	list *list
	next *element
	prev *element
	err  error
}

func (elm *element) Ok() bool             { return elm != nil && elm.list != nil && elm.err != nil }
func (elm *element) Err() error           { return elm.err }
func (elm *element) Error() string        { return elm.err.Error() }
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
