package internal

func Unwind[T any](in T) []T {
	var out []T
	for {
		switch wi := any(in).(type) {
		case interface{ Unwrap() []T }:
			items := wi.Unwrap()
			for _, i := range items {
				if any(i) == nil {
					continue
				}
				out = append(out, i)
			}

			return out
		case interface{ Unwrap() T }:
			in = wi.Unwrap()

			switch any(in).(type) {
			case nil:
				return out
			default:
				out = append(out, in)
			}
		case nil:
			return out
		default:
			return append(out, in)
		}
	}
}
