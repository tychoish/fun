package dt

import (
	"bytes"
	"encoding/json"
	"iter"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// Stack provides a generic singly linked list, with an interface that
// is broadly similar to dt.List.
type Stack[T any] struct {
	head   *Item[T]
	length int
}

// Append adds a variadic sequence of items to the list.
func (s *Stack[T]) Append(items ...T) *Stack[T]      { return s.Extend(irt.Slice(items)) }
func (s *Stack[T]) Extend(seq iter.Seq[T]) *Stack[T] { irt.Apply(seq, s.Push); return s }

func (s *Stack[T]) uncheckedSetup() { s.length = 0; s.head = &Item[T]{value: s.zero(), stack: s} }
func (*Stack[T]) zero() (o T)       { return }
func (s *Stack[T]) root() *Item[T] {
	ft.Invariant(ers.If(s != nil, ErrUninitializedContainer))

	ft.CallWhen(s.head == nil, s.uncheckedSetup)

	return s.head
}

// Len returns the length of the stack. Because stack's track their
// own size, this is an O(1) operation.
func (s *Stack[T]) Len() int { return s.length }

// Push appends an item to the stack.
func (s *Stack[T]) Push(it T) { s.root().Append(NewItem(it)) }

// Head returns the item at the top of this stack. This is a non
// destructive operation.
func (s *Stack[T]) Head() *Item[T] { return s.root() }

// Pop removes the item on the top of the stack, and returns it. If
// the stack is empty, this will return, but not detach, the root item
// of the stack, which will report a false Ok() value.
func (s *Stack[T]) Pop() *Item[T] {
	out := s.root()

	if s.length > 0 {
		s.length--
		out := s.head
		out.stack = nil
		s.head = s.head.next
	}

	return out
}

// Iterator returns a native go iterator function for the items in a set.
func (s *Stack[T]) Iterator() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := (&Item[T]{next: s.head}); item.Ok(); item = item.Next() {
			if !yield(item.Value()) {
				return
			}
		}
	}
}

// IteratorPop returns a destructive stream over the Items in a
// stack. IteratorPop will not observe new items added to the
// stack during iteration.
func (s *Stack[T]) IteratorPop() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := s.Pop(); item.Ok() && item != s.head; item = s.Pop() {
			if !yield(item.Value()) {
				return
			}
		}
	}
}

////////////////////////////////////////////////////////////////////////
//
// JSON marshaling support
//
////////////////////////////////////////////////////////////////////////

// MarshalJSON produces a JSON array representing the items in the
// stack. By supporting json.Marshaler and json.Unmarshaler, Items
// and stacks can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (s *Stack[T]) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	_, _ = buf.Write([]byte("["))

	for i := s.Head(); i.Ok(); i = i.Next() {
		if i != s.Head() {
			_, _ = buf.Write([]byte(","))
		}

		e, err := i.MarshalJSON()
		if err != nil {
			return nil, err
		}

		_, _ = buf.Write(e)
	}
	_, _ = buf.Write([]byte("]"))

	return buf.Bytes(), nil
}

// UnmarshalJSON reads json input and adds that to values in the
// stack. If there are items in the stack, they are not removed. By
// supporting json.Marshaler and json.Unmarshaler, Items and stacks
// can behave as arrays in larger json objects, and can be as the
// output/input of json.Marshal and json.Unmarshal.
func (s *Stack[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	ns := &Stack[T]{}
	head := ns.Head()
	for idx := range rv {
		elem := NewItem(s.zero())
		if err := elem.UnmarshalJSON(rv[idx]); err != nil {
			return err
		}
		head = head.Append(elem)
	}
	head = s.Head()
	for it := ns.Pop(); it.Ok(); it = ns.Pop() {
		head.Append(it)
	}
	return nil
}
