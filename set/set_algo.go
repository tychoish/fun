package set

import "context"

// AddCheck adds an item from a set, returning a value if
// that item was already in the set.
func AddCheck[T comparable](s Set[T], item T) bool { ok := s.Check(item); s.Add(item); return ok }

// DeleteCheck deletes an item from a set, returning a value if
// that item was in the set.
func DeleteCheck[T comparable](s Set[T], item T) bool {
	ok := s.Check(item)
	s.Delete(item)
	return ok
}

// Equal tests two sets, returning true if the sets have equal
// values, but will return early (and false) if the context is canceled.
func Equal[T comparable](ctx context.Context, s1, s2 Set[T]) bool {
	if s1.Len() != s2.Len() {
		return false
	}

	iter1 := s1.Iterator()
	for iter1.Next(ctx) {
		if !s2.Check(iter1.Value()) {
			return false
		}
	}

	err := iter1.Close()
	if err != nil || ctx.Err() != nil {
		return false
	}

	return true
}
