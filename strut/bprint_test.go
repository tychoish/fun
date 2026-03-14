package strut

import "testing"

func TestBprint(t *testing.T) {
	if got := Bprint("hello", " ", "world").String(); got != "hello world" {
		t.Errorf("Bprint: got %q", got)
	}
	if got := Bprintln("hello").String(); got != "hello\n" {
		t.Errorf("Bprintln: got %q", got)
	}
	if got := Bprintf("x=%d", 42).String(); got != "x=42" {
		t.Errorf("Bprintf: got %q", got)
	}
}

func TestBufPrint(t *testing.T) {
	if got := BufPrint("hello", " ", "world").String(); got != "hello world" {
		t.Errorf("BufPrint: got %q", got)
	}
	if got := BufPrintln("hello").String(); got != "hello\n" {
		t.Errorf("BufPrintln: got %q", got)
	}
	if got := BufPrintf("x=%d", 42).String(); got != "x=42" {
		t.Errorf("BufPrintf: got %q", got)
	}
}
