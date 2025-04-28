package ft

import (
	"context"
	"time"
)

// CallWithTimeout runs the function, which is the same type as
// fun.Operation, with a new context that expires after the specified duration.
func CallWithTimeout(dur time.Duration, op func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	op(ctx)
}

// CallWithContext runs the function, which is the same type as a
// fun.Operation, with a new context that is canceled after the
// function exits.
func CallWithContext(op func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op(ctx)
}

func DoWithContext[T any](op func(context.Context) T) (out T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out = op(ctx)
	return out
}
