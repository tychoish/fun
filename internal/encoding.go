package internal

import (
	"bytes"
	"strings"
)

type IgnoreNewLinesBuffer struct {
	bytes.Buffer
}

func (b *IgnoreNewLinesBuffer) Write(in []byte) (int, error) {
	_, _ = b.Buffer.Write(bytes.TrimSpace(in))
	return len(in), nil
}

func (b *IgnoreNewLinesBuffer) WriteString(in string) (int, error) {
	_, _ = b.Buffer.WriteString(strings.TrimSpace(in))
	return len(in), nil
}
