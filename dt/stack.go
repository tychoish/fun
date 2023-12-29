package dt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tychoish/fun"
)

// Stack provides a generic singly linked list, with an interface that
// is broadly similar to dt.List.
type Stack[T any] struct {
	head   *Item[T]
	length int
}

// NewListFromIterator builds a stack from the elements in the
// iterator. In general, any error would be the result of the input
// iterators's close method.
func NewStackFromIterator[T any](ctx context.Context, iter *fun.Iterator[T]) (*Stack[T], error) {
	out := &Stack[T]{}
	if err := out.Populate(iter).Run(ctx); err != nil {
		return nil, err
	}

	return out, nil
}

// Append adds a variadic sequence of items to the list.
func (s *Stack[T]) Append(items ...T) {
	for idx := range items {
		s.Push(items[idx])
	}
}

// Populate returns a worker that adds items from the iterator to the
// stack. Any error returned is either a context cancellation error or
// the result of a panic in the input iterator. The close method on
// the input iterator is not called.
func (s *Stack[T]) Populate(iter *fun.Iterator[T]) fun.Worker {
	return iter.Process(fun.Handle(s.Push).Processor())
}

// Item is a common wrapper for the elements in a stack.
type Item[T any] struct {
	next  *Item[T]
	stack *Stack[T]
	ok    bool
	value T
}

// NewItem produces a valid Item object for the specified value.
func NewItem[T any](in T) *Item[T] { return &Item[T]{value: in, ok: true} }

// String implements fmt.Stringer, and returns the string value of the
// item's value.
func (it *Item[T]) String() string { return fmt.Sprint(it.value) }

// Value returns the Item's underlying value. Use the Ok method to
// check the validity of the zero values.
func (it *Item[T]) Value() T { return it.value }

// Ok return true if the Value() has been set. Returns false for
// incompletely initialized values.
//
// Returns false when the item is nil.
func (it *Item[T]) Ok() bool { return it != nil && it.ok }

// Next returns the following item in the stack.
func (it *Item[T]) Next() *Item[T] { return it.next }

// In reports if an item is a member of a stack. Because item's track
// references to the stack, this is an O(1) operation.
func (it *Item[T]) In(s *Stack[T]) bool { return it.stack == s }

// Set mutates the value of an Item, returning true if the operation
// has been successful. The operation fails if the Item is the
// head item in a stack or not a member of a stack.
func (it *Item[T]) Set(v T) bool {
	if it.stack != nil && it.next == nil {
		return false
	}
	it.ok = true
	it.value = v
	return true
}

// Append inserts a new item after the following item in the stack,
// returning the new item, or if the new item is not valid, the item
// itself.
func (it *Item[T]) Append(n *Item[T]) *Item[T] {
	if n == nil || it.stack == nil || it.stack == n.stack || n.stack != nil || !n.ok {
		return it
	}
	it.stack.lazyInit()
	n.next = it.stack.head
	n.stack = it.stack
	n.stack.head = n
	n.stack.length++
	return n
}

// Remove removes the item from the stack, (at some expense, for items
// deeper in the stack.) If the operation isn't successful or possible
// the operation returns false.
func (it *Item[T]) Remove() bool {
	if it == nil || it.stack == nil || !it.ok {
		return false
	}

	// ok, go looking for the detach point in the stack.
	for next := it.stack.head; next.Ok(); next = next.next {
		// the next item is going to be the head of the new stack
		if next == it {
			it.stack.length--
			it.stack = nil
			next.next = it.next
			return true
		}
		if next.next == nil {
			break
		}
	}
	return false
}

// Detach splits a stack into two, using the current Item as the head
// of the new stack. The output is always non-nil: if the item is not
// valid or not the member of a stack Detach creates a new empty
// stack. If this item is currently the head of a stack, Detach
// returns that stack.
func (it *Item[T]) Detach() *Stack[T] {
	// we're not a valid, what even.
	if !it.ok {
		return &Stack[T]{head: &Item[T]{stack: it.stack}}
	}

	// we're not in a stck, make a new stack of one
	if it.stack == nil {
		it.stack = &Stack[T]{
			head:   it,
			length: 1,
		}
		return it.stack
	}

	// if you try and detach the current head, you just want the
	// current stack.
	if it.stack.head == it {
		return it.stack
	}

	// ok, go looking for the detach point in the stack.
	seen := 0
	for next := it.stack.head; next != nil; next = next.next {
		seen++
		// the next item is going to be the head of the new stack
		if next.next == it {
			next.next = &Item[T]{stack: it.stack}
			it.stack.length = seen
			it.stack = &Stack[T]{
				head:   it,
				length: it.stack.length - seen,
			}
			break
		}
	}

	return it.stack
}

// Attach removes items from the back of the stack and appends them to
// the current item. This inverts the order of items in the input
// stack.
func (it *Item[T]) Attach(stack *Stack[T]) bool {
	if stack == nil || stack.Len() == 0 || stack == it.stack {
		return false
	}

	for n := stack.Pop(); n.Ok(); n = stack.Pop() {
		it = it.Append(n)
	}

	return true
}

// MarshalJSON returns the result of json.Marshal on the value of the
// item. By supporting json.Marshaler and json.Unmarshaler,
// Items and stacks can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (it *Item[T]) MarshalJSON() ([]byte, error) { return json.Marshal(it.Value()) }

// UnmarshalJSON reads the json value, and sets the value of the
// item to the value in the json, potentially overriding an
// existing value. By supporting json.Marshaler and json.Unmarshaler,
// Items and stacks can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (it *Item[T]) UnmarshalJSON(in []byte) error {
	var val T
	if err := json.Unmarshal(in, &val); err != nil {
		return err
	}
	it.Set(val)
	return nil
}

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
	var zero T
	ns := &Stack[T]{}
	head := ns.Head()
	for idx := range rv {
		elem := NewItem(zero)
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

func (s *Stack[T]) lazyInit() {
	if s == nil {
		panic(ErrUninitializedContainer)
	}

	if s.head == nil {
		var val T
		s.length = 0
		s.head = makeItem(val)
		s.head.ok = false
		s.head.stack = s
	}
}

func makeItem[T any](val T) *Item[T] { return &Item[T]{value: val, ok: true} }

// Len returns the length of the stack. Because stack's track their
// own size, this is an O(1) operation.
func (s *Stack[T]) Len() int { return s.length }

// Push appends an item to the stack.
func (s *Stack[T]) Push(it T) { s.lazyInit(); s.head.Append(makeItem(it)) }

// Head returns the item at the top of this stack. This is a non
// destructive operation.
func (s *Stack[T]) Head() *Item[T] { s.lazyInit(); return s.head }

// Pop removes the item on the top of the stack, and returns it. If
// the stack is empty, this will return, but not detach, the root item
// of the stack, which will report a false Ok() value.
func (s *Stack[T]) Pop() *Item[T] {
	if s.head == nil {
		s.head = &Item[T]{}
		return s.head
	}
	if s.length == 0 {
		return s.head
	}
	s.length--
	out := s.head
	out.stack = nil
	s.head = s.head.next
	return out
}

func (s *Stack[T]) Producer() fun.Producer[T] {
	item := &Item[T]{next: s.head}
	return func(ctx context.Context) (o T, _ error) {
		item = item.Next()
		if !item.Ok() {
			return o, io.EOF
		}
		return item.Value(), nil
	}
}

func (s *Stack[T]) ProducerPop() fun.Producer[T] {
	var item *Item[T]
	return func(ctx context.Context) (out T, _ error) {
		item = s.Pop()
		if item == s.head {
			return out, io.EOF
		}
		return item.Value(), nil
	}
}

// Iterator returns a non-destructive iterator over the Items in a
// stack. Iterator will not observe new items added to the stack
// during iteration.
func (s *Stack[T]) Iterator() *fun.Iterator[T] { return s.Producer().Iterator() }

// PopIterator returns a destructive iterator over the Items in a
// stack. PopIterator will not observe new items added to the
// stack during iteration.
func (s *Stack[T]) PopIterator() *fun.Iterator[T] { return s.ProducerPop().Iterator() }
