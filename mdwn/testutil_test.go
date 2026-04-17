package mdwn

import (
	"unicode/utf8"
)

// build is a test helper that runs fn against a fresh Builder and returns the
// accumulated string.
func build(fn func(*Builder)) string {
	var mb Builder
	fn(&mb)
	return mb.String()
}

// rowWidth computes the visual width (rune count) of a pipe-delimited table
// row, not counting the trailing newline.
func rowWidth(line string) int { return utf8.RuneCountInString(line) }
