package internal

type Intish interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

type Uintish interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func Abs[T Intish](in T) T {
	if in < 0 {
		in = in * -1
	}
	return in
}

func Min[T Intish | Uintish](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T Intish | Uintish](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Sign[T Intish](in T) T {
	if in < 0 {
		return -1
	}
	return 1
}

func RoundUpToMultiple[T Intish](a, b T) T {
	if factor := Min(Sign(a), Sign(b)); factor == -1 {
		return -1 * RoundDownToMultiple(Abs(a), Abs(b))
	}
	multiple := Min(a, b)
	out := Max(a, b)

	out += multiple - (out % multiple)

	return out
}

func RoundDownToMultiple[T Intish](a, b T) T {
	if factor := Min(Sign(a), Sign(b)); factor == -1 {
		return -1 * RoundUpToMultiple(Abs(a), Abs(b))
	}

	multiple := Min(a, b)
	out := Max(a, b)

	out -= out % multiple
	return out
}

func RoundToSmallestMultipe[T Intish](a, b T) T {
	return Min(RoundDownToMultiple(a, b), RoundUpToMultiple(a, b))
}
func RoundToLargestMultipe[T Intish](a, b T) T {
	return Max(RoundDownToMultiple(a, b), RoundUpToMultiple(a, b))
}
