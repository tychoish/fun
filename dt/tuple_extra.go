package dt

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

func (t Tuple[K, V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := ers.Join(
		buf.WriteByte('['),
		enc.Encode(t.One),
		buf.WriteByte(','),
		enc.Encode(t.Two),
		buf.WriteByte(']'),
	)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *Tuple[K, V]) UnmarshalJSON(in []byte) error {
	rt := []json.RawMessage{}
	if err := json.Unmarshal(in, &rt); err != nil {
		return err
	}
	if len(rt) > 2 {
		return fmt.Errorf("json value has %d items", len(rt))
	}

	if err := json.Unmarshal(rt[0], &t.One); err != nil {
		return ers.Wrap(err, "tuple.One")
	}

	if len(rt) == 2 {
		if err := json.Unmarshal(rt[1], &t.Two); err != nil {
			return ers.Wrap(err, "tuple.Two")

		}
	}

	return nil

}

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p *Tuples[K, V]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	buf.WriteByte('[')

	idx := 0
	if err := p.Process(fun.MakeProcessor(func(item Tuple[K, V]) error {
		if idx != 0 {
			buf.WriteByte(',')
		}
		idx++
		if err := enc.Encode(item); err != nil {
			return ers.Wrapf(err, "tuple at index %d", idx)
		}

		return nil
	})).Wait(); err != nil {
		return nil, err
	}

	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// UnmarshalJSON provides consistent JSON decoding for Pairs
// objects. It reads a JSON document into a map and converts it to
// pairs, and appends it to the existing Pairs objects without
// removing or resetting the current object.
func (p *Tuples[K, V]) UnmarshalJSON(in []byte) error {
	t := []json.RawMessage{}
	if err := json.Unmarshal(in, &t); err != nil {
		return err
	}
	p.init()
	for idx, item := range t {
		tuple := Tuple[K, V]{}

		if err := json.Unmarshal(item, &tuple); err != nil {
			return ers.Wrapf(err, "tuple at index %d", idx)
		}
		p.ll.PushBack(tuple)
	}

	return nil
}
