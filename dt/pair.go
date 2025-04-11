package dt

// Pair represents a key-value pair. Used by the adt synchronized map
// implementation and the set package to handle ordered key-value pairs.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// Get returns only the key and value of a Pair. Provided for symmetry.
func (p Pair[K, V]) Get() (K, V) { return p.Key, p.Value }

// GetValue returns only the key of a Pair. Provided for symmetry.
func (p Pair[K, V]) GetKey() K { return p.Key }

// GetValue returns only the value of a Pair. Provided for symmetry.
func (p Pair[K, V]) GetValue() V { return p.Value }

// Set is a helper to modify the both values of a pair. This method can be chained.
func (p *Pair[K, V]) Set(k K, v V) *Pair[K, V] { p.Key = k; p.Value = v; return p }

// SetKey modifies the key of a pair. This method can be chained.
func (p *Pair[K, V]) SetKey(k K) *Pair[K, V] { p.Key = k; return p }

// SetValue modifies the value of a pair. This method can be chained.
func (p *Pair[K, V]) SetValue(v V) *Pair[K, V] { p.Value = v; return p }

// MakePair constructs a pair object. This is identical to using the
// literal constructor but may be more ergonomic as the compiler seems
// to be better at inferring types in function calls over literal
// constructors.
func MakePair[K comparable, V any](k K, v V) Pair[K, V] { return Pair[K, V]{Key: k, Value: v} }
