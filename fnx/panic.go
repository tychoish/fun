package fnx

import "github.com/tychoish/fun/erc"

func invariant(cond bool, args ...any) {
	if !cond {
		panic(erc.NewInvariantError(args...))
	}
}

func must(err error, args ...any) { invariant(err == nil, args...) }
