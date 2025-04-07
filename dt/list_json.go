package dt

import (
	"encoding/json"

	"github.com/tychoish/fun/internal"
)

// UnmarshalJSON reads the json value, and sets the value of the
// element to the value in the json, potentially overriding an
// existing value. By supporting json.Marshaler and json.Unmarshaler,
// Elements and lists can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (e *Element[T]) UnmarshalJSON(in []byte) error {
	var val T
	if err := json.Unmarshal(in, &val); err != nil {
		return err
	}
	e.Set(val)
	return nil
}

// MarshalJSON produces a JSON array representing the items in the
// list. By supporting json.Marshaler and json.Unmarshaler, Elements
// and lists can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (l *List[T]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)

	_ = buf.WriteByte('[')

	if l.Len() > 0 {
		for i := l.Front(); i.Ok(); i = i.Next() {
			if i != l.Front() {
				_ = buf.WriteByte(',')
			}
			if err := enc.Encode(i.Value()); err != nil {
				return nil, err
			}
		}
	}

	_ = buf.WriteByte(']')

	return buf.Bytes(), nil
}

// UnmarshalJSON reads json input and adds that to values in the
// list. If there are elements in the list, they are not removed. By
// supporting json.Marshaler and json.Unmarshaler, Elements and lists
// can behave as arrays in larger json objects, and can be as the
// output/input of json.Marshal and json.Unmarshal.
func (l *List[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	var zero T
	tail := l.Back()
	for idx := range rv {
		elem := NewElement(zero)
		if err := elem.UnmarshalJSON(rv[idx]); err != nil {
			return err
		}
		tail = tail.Append(elem)
	}
	return nil
}
