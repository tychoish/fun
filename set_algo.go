package fun

import "context"

// SetAddCheck adds an item from a set, returning a value if
// that item was already in the set.
func SetAddCheck[T comparable](s Set[T], item T) bool { ok := s.Check(item); s.Add(item); return ok }

// SetDeleteCheck deletes an item from a set, returning a value if
// that item was in the set.
func SetDeleteCheck[T comparable](s Set[T], item T) bool {
	ok := s.Check(item)
	s.Delete(item)
	return ok
}

// SetEqualCtx tests two sets, returning true if the sets have equal
// values, but will return early (and false) if the context is canceled.
func SetEqualCtx[T comparable](ctx context.Context, s1, s2 Set[T]) bool {
	if s1.Len() != s2.Len() {
		return false
	}

	iter1 := s1.Iterator(ctx)
	for iter1.Next(ctx) {
		if !s2.Check(iter1.Value()) {
			return false
		}
	}

	if err := iter1.Close(ctx); err != nil {
		return false
	}

	return true
}

// SetEqual tests two sets, returning true if the sets have equal
// values.
func SetEqual[T comparable](s1, s2 Set[T]) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return SetEqualCtx(ctx, s1, s2)
}
