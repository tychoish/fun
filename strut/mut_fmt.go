package strut

import "fmt"

// Mprint formats using the default formats for its operands and returns
// the resulting string as a Mutable. Spaces are added between operands
// when neither is a string. The returned Mutable is obtained from the
// pool and should be released with Release() when no longer needed.
func Mprint(a ...any) *Mutable {
	// Estimate capacity based on number of arguments (avg ~10 bytes per arg)
	mut := MakeMutable(max(len(a)*10, 16))
	fmt.Fprint(mut, a...)
	return mut
}

// Mprintf formats according to a format specifier and returns the
// resulting string as a Mutable. The returned Mutable is obtained from
// the pool and should be released with Release() when no longer needed.
// Preallocates based on format string length plus estimated argument size.
func Mprintf(format string, a ...any) *Mutable {
	// Estimate capacity: format string + extra for arguments
	mut := MakeMutable(max(len(format)+len(a)*10, 32))
	fmt.Fprintf(mut, format, a...)
	return mut
}

// Mprintln formats using the default formats for its operands and
// returns the resulting string as a Mutable. Spaces are always added
// between operands and a newline is appended. The returned Mutable is
// obtained from the pool and should be released with Release() when
// no longer needed.
func Mprintln(a ...any) *Mutable {
	// Estimate capacity based on number of arguments (avg ~10 bytes per arg) + 1 for newline
	mut := MakeMutable(max(len(a)*10+1, 16))
	fmt.Fprintln(mut, a...)
	return mut
}
