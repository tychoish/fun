package dt

import (
	"encoding/json"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// This file contains map-specific pair functionality that cannot be
// used to form the tuple type.

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p *Pairs[K, V]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	buf.WriteByte('{')

	idx := 0
	if err := p.Process(fun.MakeProcessor(func(item Pair[K, V]) error {
		if idx != 0 {
			buf.WriteByte(',')
		}
		idx++
		if err := enc.Encode(item.Key); err != nil {
			return ers.Wrapf(err, "key at %d", idx)
		}
		buf.WriteByte(':')

		return ers.Wrapf(enc.Encode(item.Value), "key at %d", idx)
	})).Wait(); err != nil {
		return nil, err
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON provides consistent JSON decoding for Pairs
// objects. It reads a JSON document into a map and converts it to
// pairs, and appends it to the existing Pairs objects without
// removing or resetting the current object.
func (p *Pairs[K, V]) UnmarshalJSON(in []byte) error {
	t := make(map[K]V)
	if err := json.Unmarshal(in, &t); err != nil {
		return err
	}

	p.ConsumeMap(t)
	return nil
}

// ConsumeValues adds all of the values in the input iterator,
// generating the keys using the function provided.
func (p *Pairs[K, V]) ConsumeValues(iter *fun.Iterator[V], keyf func(V) K) fun.Worker {
	return p.Consume(fun.Converter(func(in V) Pair[K, V] { return MakePair(keyf(in), in) }).Process(iter))
}

// ConsumeSlice adds all the values in the input slice to the Pairs
// object, creating the keys using the function provide.
func (p *Pairs[K, V]) ConsumeSlice(in []V, keyf func(V) K) {
	Sliceify(in).Observe(func(value V) { p.Add(keyf(value), value) })
}

// ConsumeMap adds all of the items in a map to the Pairs object.
func (p *Pairs[K, V]) ConsumeMap(in map[K]V) { Mapify(in).Iterator().Observe(p.Push).Ignore().Wait() }

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p *Pairs[K, V]) Map() map[K]V {
	p.init()
	out := make(map[K]V, p.ll.Len())
	for i := p.ll.Front(); i.OK(); i = i.Next() {
		pair := i.Value()
		if _, ok := out[pair.Key]; ok {
			continue
		}

		out[pair.Key] = pair.Value
	}

	return out
}
