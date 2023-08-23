package internal

func Unwind[T any](in T) []T {
	var (
		out []T
		buf []T
	)

	for {
		switch wi := any(in).(type) {
		case interface{ Unwind() []T }:
			return append(out, sparse(buffer(buf, wi.Unwind()))...)
		case interface{ Unwrap() T }:
			out = append(out, in)
			in = wi.Unwrap()
		case interface{ Unwrap() []T }:
			return append(out, sparse(buffer(buf, wi.Unwrap()))...)
		case nil:
			return out
		default:
			return append(out, in)
		}
	}
}

func buffer[T any](buf []T, slice []T) ([]T, []T) {
	buf = grow(buf, len(slice))
	buf = buf[:0]
	return buf, slice
}

func grow[T any](buf []T, size int) []T {
	var zero T
	if size < cap(buf) {
		buf = buf[:size]
	} else if cap(buf) > len(buf) {
		buf = buf[:cap(buf)]
	}

	for len(buf) < size {
		buf = append(buf, zero)
	}

	return buf
}

func sparse[T any](buf []T, items []T) []T {
	for idx := range items {
		if any(items[idx]) == nil {
			continue
		}
		buf = append(buf, items[idx])
	}
	dst := make([]T, len(buf))
	copy(dst, buf)
	return dst
}
