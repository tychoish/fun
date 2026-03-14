package strut

// Bprint formats args using default formatting and returns a new Builder
// containing the result. Analogous to fmt.Sprint.
func Bprint(args ...any) *Builder               { var b Builder; return b.AppendPrint(args...) }

// Bprintln formats args using default formatting, appends a newline, and
// returns a new Builder containing the result. Analogous to fmt.Sprintln.
func Bprintln(args ...any) *Builder             { var b Builder; return b.AppendPrintln(args...) }

// Bprintf formats according to tpl and returns a new Builder containing the
// result. Analogous to fmt.Sprintf.
func Bprintf(tpl string, args ...any) *Builder  { var b Builder; return b.AppendPrintf(tpl, args...) }

// BufPrint formats args using default formatting and returns a new Buffer
// containing the result. Analogous to fmt.Sprint.
func BufPrint(args ...any) *Buffer              { var b Buffer; return b.AppendPrint(args...) }

// BufPrintln formats args using default formatting, appends a newline, and
// returns a new Buffer containing the result. Analogous to fmt.Sprintln.
func BufPrintln(args ...any) *Buffer            { var b Buffer; return b.AppendPrintln(args...) }

// BufPrintf formats according to tpl and returns a new Buffer containing the
// result. Analogous to fmt.Sprintf.
func BufPrintf(tpl string, args ...any) *Buffer { var b Buffer; return b.AppendPrintf(tpl, args...) }
