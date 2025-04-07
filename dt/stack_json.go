package dt

import (
	"bytes"
	"encoding/json"
)

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
