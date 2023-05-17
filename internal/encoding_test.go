package internal

import "testing"

func TestEncoding(t *testing.T) {
	t.Run("WriteString", func(t *testing.T) {
		t.Run("IgnoreNoop", func(t *testing.T) {
			buf := &IgnoreNewLinesBuffer{}
			buf.WriteString("hello")
			if buf.String() != "hello" {
				t.Error(buf.String())
			}
		})
		t.Run("Strip", func(t *testing.T) {
			buf := &IgnoreNewLinesBuffer{}
			buf.WriteString("\n\nhello\n\t   ")
			if buf.String() != "hello" {
				t.Error(buf.String())
			}
		})
	})
	t.Run("WriteString", func(t *testing.T) {
		t.Run("IgnoreNoop", func(t *testing.T) {
			buf := &IgnoreNewLinesBuffer{}
			buf.Write([]byte("hello"))
			if buf.String() != "hello" {
				t.Error(buf.String())
			}
		})
		t.Run("Strip", func(t *testing.T) {
			buf := &IgnoreNewLinesBuffer{}
			buf.Write([]byte("\n\nhello\n\t   "))
			if buf.String() != "hello" {
				t.Error(buf.String())
			}
		})
	})

}
