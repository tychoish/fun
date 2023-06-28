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
