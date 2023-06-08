package internal

import (
	"context"
)

// use internally for iterations we know are cannot block. USE WITH CAUTION
var BackgroundContext = context.Background()

func IsType[T any](in any) bool { _, ok := in.(T); return ok }
