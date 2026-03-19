package strut

// Bprint formats args using default formatting and returns a new Builder
// containing the result. Analogous to fmt.Sprint.
func Bprint(args ...any) *Builder { var b Builder; return b.PushPrint(args...) }

// Bprintln formats args using default formatting, appends a newline, and
// returns a new Builder containing the result. Analogous to fmt.Sprintln.
func Bprintln(args ...any) *Builder { var b Builder; return b.PushPrintln(args...) }

// Bprintf formats according to tpl and returns a new Builder containing the
// result. Analogous to fmt.Sprintf.
func Bprintf(tpl string, args ...any) *Builder { var b Builder; return b.PushPrintf(tpl, args...) }

// BufPrint formats args using default formatting and returns a new Buffer
// containing the result. Analogous to fmt.Sprint.
func BufPrint(args ...any) *Buffer { var b Buffer; return b.PushPrint(args...) }

// BufPrintln formats args using default formatting, appends a newline, and
// returns a new Buffer containing the result. Analogous to fmt.Sprintln.
func BufPrintln(args ...any) *Buffer { var b Buffer; return b.PushPrintln(args...) }

// BufPrintf formats according to tpl and returns a new Buffer containing the
// result. Analogous to fmt.Sprintf.
func BufPrintf(tpl string, args ...any) *Buffer { var b Buffer; return b.PushPrintf(tpl, args...) }
