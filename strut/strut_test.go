package strut

import "io"

var (
	_ io.Writer       = &Buffer{}
	_ io.Writer       = &Builder{}
	_ io.StringWriter = &Builder{}
	_ io.StringWriter = &Buffer{}
	_ io.ByteWriter   = &Buffer{}
	_ io.ByteWriter   = &Builder{}
	_ io.Reader       = &Buffer{}
)
