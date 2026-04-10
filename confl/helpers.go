package confl

import "strings"

func joinStr(args ...string) string { return strings.Join(args, "") }
func callWhen[T any](cond bool, op func(T), num T) {
	if cond {
		op(num)
	}
}
