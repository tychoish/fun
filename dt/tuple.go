package dt

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tychoish/fun/erc"
)

// GENERATED FILE FROM PAIR IMPLEMENTATION

// Tuple represents a key-value tuple. Used by the adt synchronized map
// implementation and the set package to handle ordered key-value tuples.
type Tuple[K any, V any] struct {
	One K
	Two V
}

// MakeTuple constructs a tuple object. This is identical to using the
// literal constructor but may be more ergonomic as the compiler seems
// to be better at inferring types in function calls over literal
// constructors.
func MakeTuple[K any, V any](k K, v V) Tuple[K, V] { return Tuple[K, V]{One: k, Two: v} }

func (t Tuple[K, V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := erc.Join(
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
		return erc.Wrap(err, "tuple.One")
	}

	if len(rt) == 2 {
		if err := json.Unmarshal(rt[1], &t.Two); err != nil {
			return erc.Wrap(err, "tuple.Two")

		}
	}

	return nil
}
