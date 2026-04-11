package confl

import "strings"

func joinStr(args ...string) string { return strings.Join(args, "") }
func callWhen[T any](cond bool, op func(T), num T) {
	if cond {
		op(num)
	}
}

// registerAlias calls register(name, help) and, when short is non-empty,
// register(short, "short for -<name>"). Used to DRY up the long/short flag
// pair in every case of registerFlag.
func registerAlias(name, short, help string, register func(name, usage string)) {
	register(name, help)
	if short != "" {
		register(short, joinStr("short for -", name))
	}
}
