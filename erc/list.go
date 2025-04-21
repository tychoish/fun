package erc

import (
	"bytes"
	"errors"
	"fmt"
	"iter"

	"github.com/tychoish/fun/ers"
)

// Join takes a slice of errors and converts it into an *erc.Stack
// typed error. This operation has several advantages relative to
// using errors.Join(): if you call ers.Join repeatedly on the same
// error set of errors the resulting error is convertable
func Join(errs ...error) error { st := &list{}; st.Add(errs...); return st.Resolve() }

type list struct {
	num int
	elm element
}

func (eel *list) In(elm *element) bool { return eel == elm.list }
func (eel *list) Front() *element      { return eel.root().next }
func (eel *list) Back() *element       { return eel.root().prev }
func (eel *list) Resolve() error       { return eel.Err() }
func (eel *list) Is(target error) bool {
	for err := range eel.FIFO() {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (eel *list) As(target any) bool {
	for err := range eel.FIFO() {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

func (eel *list) Err() error {
	switch {
	case eel == nil || eel.num == 0:
		return nil
	case eel.num == 1:
		return eel.Front().Err()
	default:
		return eel
	}
}

func (eel *list) Len() int {
	if eel == nil {
		return 0
	}

	return eel.num
}

func (eel *list) root() *element {
	if eel.elm.err != nil {
		panic(Join(ers.Error("invalid error list"), ers.ErrInvariantViolation))
	}
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

func (eel *list) Push(err error) {
	defer func() {
		if err == nil {
			return
		}
		fmt.Println("EEL.PUSH.DEFER", eel.num, eel.Len(), err)
	}()

	switch werr := err.(type) {
	case *list:
		fmt.Println("EEL.PUSH.LIST")
		for elem := werr.root().Next(); elem.Ok(); elem.Next() {
			eel.PushBack(elem.Err())
		}
	case *element:
		fmt.Println("EEL.PUSH.ELEMENT")
		for elem := werr; elem.Ok(); elem.Next() {
			eel.PushBack(elem.Err())
		}
	case interface{ Unwind() []error }:
		fmt.Println("EEL.PUSH.UNWIND")
		eel.Add(werr.Unwind()...)
	case interface{ Unwrap() []error }:
		fmt.Println("EEL.PUSH.UNWRAP")
		eel.Add(werr.Unwrap()...)
	case nil:
		return
	default:
		fmt.Println("EEL.PUSH.DEFAULT", eel.Len(), err)
		eel.PushBack(err)
	}
}

func (eel *list) Add(errs ...error) {
	for _, err := range errs {
		eel.Push(err)
	}
}

func (eel *list) PushBack(err error) {
	if err == nil {
		return
	}

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

func (eel *list) PushFront(err error) {
	if err == nil {
		return
	}

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

func (eel *list) Unwind() []error {
	out := make([]error, eel.num)

	idx := 0
	for elem := eel.Front(); elem.Ok(); elem = elem.Next() {
		out[idx] = elem.err
		idx++
	}

	return out
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

func (eel *list) FIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.Back(); elem.Ok(); elem = elem.Previous() {
			if !yield(elem) {
				return
			}
		}
	}
}

func (eel *list) LIFO() iter.Seq[error] {
	return func(yield func(err error) bool) {
		for elem := eel.Front(); elem.Ok(); elem = elem.Next() {
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
func (elm *element) In(eel *list) bool    { return elm.list == eel }
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
