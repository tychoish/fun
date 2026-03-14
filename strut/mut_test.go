package strut

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"
	"unicode"
)

func TestMutableLen(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"empty", []byte{}, 0},
		{"single byte", []byte("a"), 1},
		{"multiple bytes", []byte("hello"), 5},
		{"unicode", []byte("hello 世界"), 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.input)
			if got := mut.Len(); got != tt.want {
				t.Errorf("Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableCap(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		cap  int
	}{
		{"empty", []byte{}, 0},
		{"with capacity", make([]byte, 5, 10), 10},
		{"full capacity", make([]byte, 10), 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.Cap(); got != tt.cap {
				t.Errorf("Cap() = %v, want %v", got, tt.cap)
			}
		})
	}
}

func TestMutableGrow(t *testing.T) {
	tests := []struct {
		name      string
		initial   []byte
		grow      int
		wantLen   int
		minCap    int
		wantPanic bool
	}{
		{"grow empty", []byte{}, 10, 0, 10, false},
		{"grow existing", []byte("hello"), 5, 5, 10, false},
		{"negative grow", []byte("test"), -1, 0, 0, true},
		{"zero grow", []byte("test"), 0, 4, 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic but did not get one")
					}
				}()
			}

			mut := Mutable(tt.initial)
			mut.Grow(tt.grow)

			if !tt.wantPanic {
				if got := mut.Len(); got != tt.wantLen {
					t.Errorf("Len() after Grow() = %v, want %v", got, tt.wantLen)
				}
				if got := mut.Cap(); got < tt.minCap {
					t.Errorf("Cap() after Grow() = %v, want at least %v", got, tt.minCap)
				}
			}
		})
	}
}

func TestMutableReset(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	origCap := mut.Cap()
	mut.Reset()

	if got := mut.Len(); got != 0 {
		t.Errorf("Len() after Reset() = %v, want 0", got)
	}
	if got := mut.Cap(); got != origCap {
		t.Errorf("Cap() after Reset() = %v, want %v (should preserve capacity)", got, origCap)
	}
}

func TestMutableWrite(t *testing.T) {
	tests := []struct {
		name    string
		initial []byte
		write   []byte
		want    string
	}{
		{"write to empty", []byte{}, []byte("hello"), "hello"},
		{"append", []byte("hello"), []byte(" world"), "hello world"},
		{"write empty", []byte("test"), []byte{}, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.initial)
			n, err := mut.Write(tt.write)
			if err != nil {
				t.Errorf("Write() error = %v", err)
			}
			if n != len(tt.write) {
				t.Errorf("Write() n = %v, want %v", n, len(tt.write))
			}
			if got := string(mut); got != tt.want {
				t.Errorf("after Write() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableWriteByte(t *testing.T) {
	tests := []struct {
		name    string
		initial []byte
		write   byte
		want    string
	}{
		{"write to empty", []byte{}, 'a', "a"},
		{"append", []byte("hello"), '!', "hello!"},
		{"null byte", []byte("test"), 0, "test\x00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.initial)
			err := mut.WriteByte(tt.write)
			if err != nil {
				t.Errorf("WriteByte() error = %v", err)
			}
			if got := string(mut); got != tt.want {
				t.Errorf("after WriteByte() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableWriteRune(t *testing.T) {
	tests := []struct {
		name    string
		initial []byte
		write   rune
		want    string
		wantN   int
	}{
		{"ASCII", []byte{}, 'a', "a", 1},
		{"multi-byte", []byte("hello"), '世', "hello世", 3},
		{"emoji", []byte("test"), '😀', "test😀", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.initial)
			n, err := mut.WriteRune(tt.write)
			if err != nil {
				t.Errorf("WriteRune() error = %v", err)
			}
			if n != tt.wantN {
				t.Errorf("WriteRune() n = %v, want %v", n, tt.wantN)
			}
			if got := string(mut); got != tt.want {
				t.Errorf("after WriteRune() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableWriteString(t *testing.T) {
	tests := []struct {
		name    string
		initial []byte
		write   string
		want    string
	}{
		{"write to empty", []byte{}, "hello", "hello"},
		{"append", []byte("hello"), " world", "hello world"},
		{"unicode", []byte("test"), " 世界", "test 世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.initial)
			n, err := mut.WriteString(tt.write)
			if err != nil {
				t.Errorf("WriteString() error = %v", err)
			}
			if n != len(tt.write) {
				t.Errorf("WriteString() n = %v, want %v", n, len(tt.write))
			}
			if got := string(mut); got != tt.want {
				t.Errorf("after WriteString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableReader(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"simple", []byte("hello world")},
		{"unicode", []byte("hello 世界")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			reader := mut.Reader()
			buf, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("ReadAll() error = %v", err)
			}
			if !bytes.Equal(buf, tt.data) {
				t.Errorf("Reader() content = %q, want %q", buf, tt.data)
			}
		})
	}
}

func TestMutableString(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{}, ""},
		{"simple", []byte("hello"), "hello"},
		{"unicode", []byte("世界"), "世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableCopy(t *testing.T) {
	tests := []struct {
		name    string
		src     []byte
		dstInit []byte
		want    []byte
	}{
		{"empty to empty", []byte{}, []byte{}, []byte{}},
		{"data to empty", []byte("hello"), []byte{}, []byte("hello")},
		{"data to smaller", []byte("hello"), []byte("hi"), []byte("hello")},
		{"data to larger", []byte("hi"), []byte("hello world"), []byte("hi")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := Mutable(tt.src)
			dst := Mutable(tt.dstInit)
			err := src.Copy(&dst)
			if err != nil {
				t.Errorf("Copy() error = %v", err)
			}
			if !bytes.Equal(dst, tt.want) {
				t.Errorf("Copy() dst = %q, want %q", dst, tt.want)
			}
		})
	}
}

func TestMutableCopyTo(t *testing.T) {
	tests := []struct {
		name    string
		src     []byte
		dstSize int
		wantErr bool
	}{
		{"exact size", []byte("hello"), 5, false},
		{"larger dst", []byte("hello"), 10, false},
		{"smaller dst", []byte("hello"), 3, true},
		{"empty to empty", []byte{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := Mutable(tt.src)
			dst := make([]byte, tt.dstSize)
			err := src.CopyTo(dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyTo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !bytes.Equal(dst[:len(tt.src)], tt.src) {
				t.Errorf("CopyTo() dst = %q, want %q", dst[:len(tt.src)], tt.src)
			}
		})
	}
}

func TestMutableContains(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		subslice []byte
		want     bool
	}{
		{"contains", []byte("hello world"), []byte("world"), true},
		{"not contains", []byte("hello world"), []byte("foo"), false},
		{"empty subslice", []byte("hello"), []byte{}, true},
		{"empty data", []byte{}, []byte("test"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.Contains(tt.subslice); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableContainsString(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		s    string
		want bool
	}{
		{"contains", []byte("hello world"), "world", true},
		{"not contains", []byte("hello world"), "foo", false},
		{"unicode", []byte("hello 世界"), "世界", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.ContainsString(tt.s); got != tt.want {
				t.Errorf("ContainsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{"equal", []byte("hello"), []byte("hello"), true},
		{"not equal", []byte("hello"), []byte("world"), false},
		{"empty", []byte{}, []byte{}, true},
		{"different length", []byte("hello"), []byte("hi"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.a)
			if got := mut.Equal(tt.b); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableHasPrefix(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		prefix []byte
		want   bool
	}{
		{"has prefix", []byte("hello world"), []byte("hello"), true},
		{"no prefix", []byte("hello world"), []byte("world"), false},
		{"empty prefix", []byte("hello"), []byte{}, true},
		{"longer prefix", []byte("hi"), []byte("hello"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.HasPrefix(tt.prefix); got != tt.want {
				t.Errorf("HasPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableHasSuffix(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		suffix []byte
		want   bool
	}{
		{"has suffix", []byte("hello world"), []byte("world"), true},
		{"no suffix", []byte("hello world"), []byte("hello"), false},
		{"empty suffix", []byte("hello"), []byte{}, true},
		{"longer suffix", []byte("hi"), []byte("hello"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.HasSuffix(tt.suffix); got != tt.want {
				t.Errorf("HasSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableIndex(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		sep  []byte
		want int
	}{
		{"found", []byte("hello world"), []byte("world"), 6},
		{"not found", []byte("hello world"), []byte("foo"), -1},
		{"empty sep", []byte("hello"), []byte{}, 0},
		{"multiple occurrences", []byte("hello hello"), []byte("hello"), 0}, //nolint:dupword
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.Index(tt.sep); got != tt.want {
				t.Errorf("Index() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableReplace(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		old  []byte
		new  []byte
		n    int
		want string
	}{
		{"single replace", []byte("hello world"), []byte("world"), []byte("Go"), 1, "hello Go"},
		{"replace all", []byte("foo foo foo"), []byte("foo"), []byte("bar"), -1, "bar bar bar"}, //nolint:dupword
		{"no match", []byte("hello"), []byte("world"), []byte("Go"), 1, "hello"},
		{"replace n", []byte("foo foo foo"), []byte("foo"), []byte("bar"), 2, "bar bar foo"}, //nolint:dupword
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.Replace(tt.old, tt.new, tt.n)
			if got := string(mut); got != tt.want {
				t.Errorf("Replace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableToLower(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"simple", []byte("HELLO"), "hello"},
		{"mixed", []byte("HeLLo WoRLd"), "hello world"},
		{"already lower", []byte("hello"), "hello"},
		{"unicode", []byte("ΩΔΦΓ"), "ωδφγ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.ToLower()
			if got := string(mut); got != tt.want {
				t.Errorf("ToLower() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableToUpper(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"simple", []byte("hello"), "HELLO"},
		{"mixed", []byte("HeLLo WoRLd"), "HELLO WORLD"},
		{"already upper", []byte("HELLO"), "HELLO"},
		{"unicode", []byte("ωδφγ"), "ΩΔΦΓ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.ToUpper()
			if got := string(mut); got != tt.want {
				t.Errorf("ToUpper() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableTrim(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		cutset string
		want   string
	}{
		{"trim spaces", []byte("  hello  "), " ", "hello"},
		{"trim characters", []byte("xxhelloxx"), "x", "hello"},
		{"no trim needed", []byte("hello"), " ", "hello"},
		{"trim all", []byte("xxx"), "x", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.Trim(tt.cutset)
			if got := string(mut); got != tt.want {
				t.Errorf("Trim() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableTrimSpace(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"leading and trailing", []byte("  hello  "), "hello"},
		{"only leading", []byte("  hello"), "hello"},
		{"only trailing", []byte("hello  "), "hello"},
		{"no spaces", []byte("hello"), "hello"},
		{"tabs and newlines", []byte("\t\nhello\n\t"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.TrimSpace()
			if got := string(mut); got != tt.want {
				t.Errorf("TrimSpace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableTrimPrefix(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		prefix []byte
		want   string
	}{
		{"has prefix", []byte("hello world"), []byte("hello "), "world"},
		{"no prefix", []byte("hello world"), []byte("foo"), "hello world"},
		{"empty prefix", []byte("hello"), []byte{}, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.TrimPrefix(tt.prefix)
			if got := string(mut); got != tt.want {
				t.Errorf("TrimPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableTrimSuffix(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		suffix []byte
		want   string
	}{
		{"has suffix", []byte("hello world"), []byte(" world"), "hello"},
		{"no suffix", []byte("hello world"), []byte("foo"), "hello world"},
		{"empty suffix", []byte("hello"), []byte{}, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.TrimSuffix(tt.suffix)
			if got := string(mut); got != tt.want {
				t.Errorf("TrimSuffix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableSplit(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		sep  []byte
		want []string
	}{
		{"simple split", []byte("a,b,c"), []byte(","), []string{"a", "b", "c"}},
		{"no separator", []byte("hello"), []byte(","), []string{"hello"}},
		{"empty parts", []byte("a,,c"), []byte(","), []string{"a", "", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			got := slices.Collect(mut.Split(tt.sep))
			if len(got) != len(tt.want) {
				t.Errorf("Split() returned %d parts, want %d", len(got), len(tt.want))
			}
			for i, part := range got {
				if part.String() != tt.want[i] {
					t.Errorf("Split()[%d] = %q, want %q", i, part, tt.want[i])
				}
			}
		})
	}
}

func TestMutableFields(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{"simple", []byte("hello world"), []string{"hello", "world"}},
		{"multiple spaces", []byte("hello   world"), []string{"hello", "world"}},
		{"tabs and spaces", []byte("hello\t\tworld"), []string{"hello", "world"}},
		{"empty", []byte(""), []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			got := slices.Collect(mut.Fields())
			if len(got) != len(tt.want) {
				t.Errorf("Fields() returned %d parts, want %d", len(got), len(tt.want))
			}
			for i, part := range got {
				if part.String() != tt.want[i] {
					t.Errorf("Fields()[%d] = %q, want %q", i, part, tt.want[i])
				}
			}
		})
	}
}

func TestMutableClone(t *testing.T) {
	original := Mutable([]byte("hello world"))
	clone := original.Clone()

	if !bytes.Equal(*clone, original) {
		t.Errorf("Clone() = %q, want %q", *clone, original)
	}

	// Modify clone and ensure original is unchanged
	clone.WriteByte('!')
	if bytes.Equal(*clone, original) {
		t.Error("Clone() shares underlying data with original")
	}
}

func TestMutableIsASCII(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, true},
		{"pure ASCII", []byte("hello world"), true},
		{"ASCII with numbers", []byte("test123"), true},
		{"with unicode", []byte("hello 世界"), false},
		{"high bit set", []byte{0xFF}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.IsASCII(); got != tt.want {
				t.Errorf("IsASCII() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableIsUnicode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, true},
		{"valid ASCII", []byte("hello"), true},
		{"valid unicode", []byte("hello 世界"), true},
		{"invalid UTF-8", []byte{0xFF, 0xFE}, false},
		{"partial rune", []byte("hello\xc3"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.IsUnicode(); got != tt.want {
				t.Errorf("IsUnicode() = %v, want %v for %q", got, tt.want, tt.data)
			}
		})
	}
}

func TestMutableAppend(t *testing.T) {
	mut1 := Mutable([]byte("hello"))
	mut2 := Mutable([]byte(" world"))
	mut1.Append(&mut2)

	want := "hello world"
	if got := string(mut1); got != want {
		t.Errorf("Append() = %q, want %q", got, want)
	}
}

func TestMutableCount(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		sep  []byte
		want int
	}{
		{"count occurrences", []byte("hello hello hello"), []byte("hello"), 3}, //nolint:dupword
		{"no occurrences", []byte("hello world"), []byte("foo"), 0},
		{"overlapping not counted", []byte("aaaa"), []byte("aa"), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.Count(tt.sep); got != tt.want {
				t.Errorf("Count() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableCompare(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want int
	}{
		{"equal", []byte("hello"), []byte("hello"), 0},
		{"less than", []byte("abc"), []byte("def"), -1},
		{"greater than", []byte("def"), []byte("abc"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.a)
			got := mut.Compare(tt.b)
			if (got < 0 && tt.want >= 0) || (got > 0 && tt.want <= 0) || (got == 0 && tt.want != 0) {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutableCut(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		sep        []byte
		wantBefore string
		wantAfter  string
		wantFound  bool
	}{
		{"found", []byte("hello world"), []byte(" "), "hello", "world", true},
		{"not found", []byte("hello"), []byte(","), "", "", false},
		{"empty sep", []byte("hello"), []byte(""), "", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			before, after, found := mut.Cut(tt.sep)
			if found != tt.wantFound {
				t.Errorf("Cut() found = %v, want %v", found, tt.wantFound)
			}
			if found {
				if string(before) != tt.wantBefore {
					t.Errorf("Cut() before = %q, want %q", before, tt.wantBefore)
				}
				if string(after) != tt.wantAfter {
					t.Errorf("Cut() after = %q, want %q", after, tt.wantAfter)
				}
			}
		})
	}
}

func TestMutableRepeat(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		count int
		want  string
	}{
		{"repeat once", []byte("hi"), 1, "hi"},
		{"repeat multiple", []byte("ha"), 3, "hahaha"},
		{"repeat zero", []byte("test"), 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.Repeat(tt.count)
			if got := string(mut); got != tt.want {
				t.Errorf("Repeat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMutableMap(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		fn   func(rune) rune
		want string
	}{
		{"to upper", []byte("hello"), unicode.ToUpper, "HELLO"},
		{"identity", []byte("test"), func(r rune) rune { return r }, "test"},
		{"shift", []byte("abc"), func(r rune) rune { return r + 1 }, "bcd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			mut.Map(tt.fn)
			if got := string(mut); got != tt.want {
				t.Errorf("Map() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewMutableAndRelease(t *testing.T) {
	mut := NewMutable()
	if mut == nil {
		t.Fatal("NewMutable() returned nil")
	}

	mut.WriteString("test data")
	mut.Release()

	// Get another one and ensure it was reset
	mut2 := NewMutable()
	if mut2.Len() != 0 {
		t.Errorf("Released Mutable was not reset, Len() = %d, want 0", mut2.Len())
	}
}

func TestMutableReleaseLargeBuffer(t *testing.T) {
	mut := NewMutable()
	// Create a large buffer (> 64KB)
	large := make([]byte, 65*1024)
	mut.Write(large)

	mut.Release()
	// Large buffers should not be returned to pool
	// This is hard to test directly, but we can at least ensure Release doesn't panic
}

func TestMakeMutable(t *testing.T) {
	t.Run("creates with specified capacity", func(t *testing.T) {
		mut := MakeMutable(100)
		if mut == nil {
			t.Fatal("MakeMutable() returned nil")
		}
		if mut.Len() != 0 {
			t.Errorf("MakeMutable() initial length = %d, want 0", mut.Len())
		}
		if mut.Cap() < 100 {
			t.Errorf("MakeMutable(100) capacity = %d, want >= 100", mut.Cap())
		}
	})

	t.Run("zero capacity", func(t *testing.T) {
		mut := MakeMutable(0)
		if mut == nil {
			t.Fatal("MakeMutable(0) returned nil")
		}
		if mut.Len() != 0 {
			t.Errorf("MakeMutable(0) length = %d, want 0", mut.Len())
		}
		if mut.Cap() != 0 {
			t.Errorf("MakeMutable(0) capacity = %d, want 0", mut.Cap())
		}
	})

	t.Run("can write up to capacity without realloc", func(t *testing.T) {
		capacity := 50
		mut := MakeMutable(capacity)
		initialCap := mut.Cap()

		// Write data up to capacity
		data := make([]byte, capacity)
		for i := range data {
			data[i] = byte('a' + (i % 26))
		}
		mut.Write(data)

		if mut.Len() != capacity {
			t.Errorf("After writing %d bytes, length = %d", capacity, mut.Len())
		}
		if mut.Cap() != initialCap {
			t.Errorf("Capacity changed from %d to %d (should not reallocate)", initialCap, mut.Cap())
		}
	})

	t.Run("grows beyond capacity", func(t *testing.T) {
		mut := MakeMutable(10)
		initialCap := mut.Cap()

		// Write more than capacity
		data := make([]byte, 20)
		mut.Write(data)

		if mut.Len() != 20 {
			t.Errorf("After writing 20 bytes, length = %d", mut.Len())
		}
		if mut.Cap() <= initialCap {
			t.Errorf("Capacity should have grown beyond %d, got %d", initialCap, mut.Cap())
		}
	})

	t.Run("uses pool and can be released", func(t *testing.T) {
		// MakeMutable uses the pool
		made := MakeMutable(10)
		made.WriteString("made data")

		if made.String() != "made data" {
			t.Errorf("MakeMutable data = %q, want %q", made.String(), "made data")
		}

		// Should be able to release it
		made.Release()

		// Get another instance - verify pool is working
		made2 := MakeMutable(10)
		if made2.Len() != 0 {
			t.Errorf("Released Mutable was not reset, Len() = %d, want 0", made2.Len())
		}
		made2.Release()
	})

	t.Run("grows pooled instance to requested capacity", func(t *testing.T) {
		// First, put a small buffer in the pool
		small := NewMutable()
		small.WriteString("x") // Use it briefly
		small.Release()

		// Request larger capacity - should grow the pooled instance
		large := MakeMutable(1000)
		if large.Cap() < 1000 {
			t.Errorf("MakeMutable(1000) capacity = %d, want >= 1000", large.Cap())
		}
		if large.Len() != 0 {
			t.Errorf("MakeMutable(1000) length = %d, want 0", large.Len())
		}
		large.Release()
	})
}

func TestMutableIsNullTerminated(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"null terminated", []byte("hello\x00"), true},
		{"not null terminated", []byte("hello"), false},
		{"null in middle", []byte("hel\x00lo"), false},
		{"only null", []byte{0}, true},
		{"multiple nulls at end", []byte("test\x00\x00"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.IsNullTerminated(); got != tt.want {
				t.Errorf("IsNullTerminated() = %v, want %v for %q", got, tt.want, tt.data)
			}
		})
	}
}

func TestMutablePrint(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	t.Run("print basic string", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.Print()
	})

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "hello world"
	if got := buf.String(); got != expected {
		t.Errorf("Print() wrote %q, want %q", got, expected)
	}
}

func TestMutablePrintMultiple(t *testing.T) {
	// Test multiple Print calls
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut1 := Mutable([]byte("foo"))
	mut2 := Mutable([]byte("bar"))
	mut3 := Mutable([]byte("baz"))

	mut1.Print()
	mut2.Print()
	mut3.Print()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "foobarbaz"
	if got := buf.String(); got != expected {
		t.Errorf("Print() multiple calls wrote %q, want %q", got, expected)
	}
}

func TestMutablePrintln(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	t.Run("println basic string", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.Println()
	})

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "hello world\n"
	if got := buf.String(); got != expected {
		t.Errorf("Println() wrote %q, want %q", got, expected)
	}
}

func TestMutablePrintlnMultiple(t *testing.T) {
	// Test multiple Println calls
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut1 := Mutable([]byte("line1"))
	mut2 := Mutable([]byte("line2"))
	mut3 := Mutable([]byte("line3"))

	mut1.Println()
	mut2.Println()
	mut3.Println()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "line1\nline2\nline3\n"
	if got := buf.String(); got != expected {
		t.Errorf("Println() multiple calls wrote %q, want %q", got, expected)
	}
}

func TestMutablePrintEmpty(t *testing.T) {
	// Test printing empty Mutable
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut := Mutable([]byte{})
	mut.Print()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("Print() empty wrote %q, want empty string", got)
	}
}

func TestMutablePrintlnEmpty(t *testing.T) {
	// Test Println with empty Mutable
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut := Mutable([]byte{})
	mut.Println()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "\n"
	if got := buf.String(); got != expected {
		t.Errorf("Println() empty wrote %q, want newline", got)
	}
}

func TestMutablePrintSpecialCharacters(t *testing.T) {
	// Test printing with special characters
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut := Mutable([]byte("hello\tworld\n123"))
	mut.Print()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "hello\tworld\n123"
	if got := buf.String(); got != expected {
		t.Errorf("Print() with special chars wrote %q, want %q", got, expected)
	}
}

func TestMutablePrintUnicode(t *testing.T) {
	// Test printing with Unicode characters
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	mut := Mutable([]byte("Hello 世界 🌍"))
	mut.Println()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	expected := "Hello 世界 🌍\n"
	if got := buf.String(); got != expected {
		t.Errorf("Println() with Unicode wrote %q, want %q", got, expected)
	}
}

// Additional coverage tests for String variants.
func TestMutableStringVariants(t *testing.T) {
	// Test EqualFoldString
	mut := Mutable([]byte("Hello"))
	if !mut.EqualFoldString("HELLO") {
		t.Error("EqualFoldString() should be case-insensitive")
	}

	// Test CompareString
	mut = Mutable([]byte("abc"))
	if got := mut.CompareString("def"); got >= 0 {
		t.Errorf("CompareString() = %v, want < 0", got)
	}

	// Test CutString
	mut = Mutable([]byte("hello,world"))
	before, after, found := mut.CutString(",")
	if !found || string(before) != "hello" || string(after) != "world" {
		t.Errorf("CutString() = (%q, %q, %v), want (\"hello\", \"world\", true)", before, after, found)
	}

	// Test CutPrefixString
	mut = Mutable([]byte("hello world"))
	after2, found2 := mut.CutPrefixString("hello ")
	if !found2 || string(after2) != "world" {
		t.Errorf("CutPrefixString() = (%q, %v), want (\"world\", true)", after2, found2)
	}

	// Test CutSuffixString
	mut = Mutable([]byte("hello world"))
	before2, found3 := mut.CutSuffixString(" world")
	if !found3 || string(before2) != "hello" {
		t.Errorf("CutSuffixString() = (%q, %v), want (\"hello\", true)", before2, found3)
	}
}

func TestMutableTrimFuncVariants(t *testing.T) {
	isSpace := func(r rune) bool { return r == ' ' }

	// Test TrimFunc
	mut := Mutable([]byte("  hello  "))
	mut.TrimFunc(isSpace)
	if got := string(mut); got != "hello" {
		t.Errorf("TrimFunc() = %q, want \"hello\"", got)
	}

	// Test TrimLeftFunc
	mut = Mutable([]byte("  hello  "))
	mut.TrimLeftFunc(isSpace)
	if got := string(mut); got != "hello  " {
		t.Errorf("TrimLeftFunc() = %q, want \"hello  \"", got)
	}

	// Test TrimRightFunc
	mut = Mutable([]byte("  hello  "))
	mut.TrimRightFunc(isSpace)
	if got := string(mut); got != "  hello" {
		t.Errorf("TrimRightFunc() = %q, want \"  hello\"", got)
	}
}

func TestMutableSplitVariants(t *testing.T) {
	// Test SplitNString
	mut := Mutable([]byte("a,b,c,d"))
	parts := slices.Collect(mut.SplitNString(",", 2))
	if len(parts) != 2 || parts[0].String() != "a" || parts[1].String() != "b,c,d" {
		t.Errorf("SplitNString() failed, got %v", parts)
	}

	// Test SplitAfterString
	mut = Mutable([]byte("a,b,c"))
	parts = slices.Collect(mut.SplitAfterString(","))
	if len(parts) != 3 || parts[0].String() != "a," || parts[1].String() != "b," || parts[2].String() != "c" {
		t.Errorf("SplitAfterString() failed, got %v", parts)
	}

	// Test SplitAfterNString
	mut = Mutable([]byte("a,b,c,d"))
	parts = slices.Collect(mut.SplitAfterNString(",", 2))
	if len(parts) != 2 || parts[0].String() != "a," || parts[1].String() != "b,c,d" {
		t.Errorf("SplitAfterNString() failed, got %v", parts)
	}

	// Test FieldsFunc
	isComma := func(r rune) bool { return r == ',' }
	mut = Mutable([]byte("a,b,c"))
	parts = slices.Collect(mut.FieldsFunc(isComma))
	if len(parts) != 3 || parts[0].String() != "a" || parts[1].String() != "b" || parts[2].String() != "c" {
		t.Errorf("FieldsFunc() failed, got %v", parts)
	}
}

func TestMutableIndexVariants(t *testing.T) {
	mut := Mutable([]byte("hello world"))

	// Test IndexAny
	if got := mut.IndexAny("aeiou"); got != 1 {
		t.Errorf("IndexAny() = %v, want 1", got)
	}

	// Test LastIndexAny
	if got := mut.LastIndexAny("aeiou"); got != 7 {
		t.Errorf("LastIndexAny() = %v, want 7", got)
	}

	// Test ContainsAny
	if !mut.ContainsAny("aeiou") {
		t.Error("ContainsAny() should return true for 'aeiou' (contains vowels)")
	}

	// Test ContainsAny with no match
	if mut.ContainsAny("xyz") {
		t.Error("ContainsAny() should return false for 'xyz'")
	}
}

func TestMutableToTitle(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	mut.ToTitle()
	// ToTitle in bytes package converts to title case (similar to upper for ASCII)
	if got := string(mut); got != "HELLO WORLD" {
		t.Errorf("ToTitle() = %q, want \"HELLO WORLD\"", got)
	}
}

// TestCaseModificationComprehensive tests case conversion with all 26 ASCII letters
// and proper handling of whitespace characters.
func TestCaseModificationComprehensive(t *testing.T) {
	t.Run("ToLower all ASCII letters", func(t *testing.T) {
		// Test all 26 uppercase letters
		input := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		expected := "abcdefghijklmnopqrstuvwxyz"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() all uppercase = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper all ASCII letters", func(t *testing.T) {
		// Test all 26 lowercase letters
		input := []byte("abcdefghijklmnopqrstuvwxyz")
		expected := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() all lowercase = %q, want %q", got, expected)
		}
	})

	t.Run("ToTitle all ASCII letters", func(t *testing.T) {
		// Test all 26 lowercase letters
		input := []byte("abcdefghijklmnopqrstuvwxyz")
		expected := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		mut := Mutable(input)
		mut.ToTitle()
		if got := string(mut); got != expected {
			t.Errorf("ToTitle() all lowercase = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower with whitespace", func(t *testing.T) {
		// Test that whitespace is not modified
		input := []byte("HELLO\nWORLD\tFOO BAR")
		expected := "hello\nworld\tfoo bar"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() with whitespace = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper with whitespace", func(t *testing.T) {
		// Test that whitespace is not modified
		input := []byte("hello\nworld\tfoo bar")
		expected := "HELLO\nWORLD\tFOO BAR"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() with whitespace = %q, want %q", got, expected)
		}
	})

	t.Run("ToTitle with whitespace", func(t *testing.T) {
		// Test that whitespace is not modified
		input := []byte("hello\nworld\tfoo bar")
		expected := "HELLO\nWORLD\tFOO BAR"
		mut := Mutable(input)
		mut.ToTitle()
		if got := string(mut); got != expected {
			t.Errorf("ToTitle() with whitespace = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower mixed case with all letters and whitespace", func(t *testing.T) {
		// Comprehensive test: all letters, mixed case, with whitespace
		input := []byte("The Quick BROWN Fox\nJumps OVER\tThe Lazy DOG")
		expected := "the quick brown fox\njumps over\tthe lazy dog"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() comprehensive = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper mixed case with all letters and whitespace", func(t *testing.T) {
		// Comprehensive test: all letters, mixed case, with whitespace
		input := []byte("The Quick BROWN Fox\nJumps OVER\tThe Lazy DOG")
		expected := "THE QUICK BROWN FOX\nJUMPS OVER\tTHE LAZY DOG"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() comprehensive = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower only whitespace", func(t *testing.T) {
		// Test that only whitespace is unchanged
		input := []byte(" \n\t \n \t")
		expected := " \n\t \n \t"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() only whitespace = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper only whitespace", func(t *testing.T) {
		// Test that only whitespace is unchanged
		input := []byte(" \n\t \n \t")
		expected := " \n\t \n \t"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() only whitespace = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower with digits and punctuation", func(t *testing.T) {
		// Ensure digits and punctuation are not modified
		input := []byte("HELLO123!@#WORLD456$%^")
		expected := "hello123!@#world456$%^"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() with digits/punctuation = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper with digits and punctuation", func(t *testing.T) {
		// Ensure digits and punctuation are not modified
		input := []byte("hello123!@#world456$%^")
		expected := "HELLO123!@#WORLD456$%^"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() with digits/punctuation = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower with all graphical characters", func(t *testing.T) {
		// Test comprehensive set of graphical characters (ASCII printable)
		input := []byte("ABC!@#$%^&*()_+-=[]{}|;':\",./<>?`~123DEF")
		expected := "abc!@#$%^&*()_+-=[]{}|;':\",./<>?`~123def"
		mut := Mutable(input)
		mut.ToLower()
		if got := string(mut); got != expected {
			t.Errorf("ToLower() with graphical chars = %q, want %q", got, expected)
		}
	})

	t.Run("ToUpper with all graphical characters", func(t *testing.T) {
		// Test comprehensive set of graphical characters (ASCII printable)
		input := []byte("abc!@#$%^&*()_+-=[]{}|;':\",./<>?`~123def")
		expected := "ABC!@#$%^&*()_+-=[]{}|;':\",./<>?`~123DEF"
		mut := Mutable(input)
		mut.ToUpper()
		if got := string(mut); got != expected {
			t.Errorf("ToUpper() with graphical chars = %q, want %q", got, expected)
		}
	})

	t.Run("ToTitle with all graphical characters", func(t *testing.T) {
		// Test comprehensive set of graphical characters (ASCII printable)
		input := []byte("abc!@#$%^&*()_+-=[]{}|;':\",./<>?`~123def")
		expected := "ABC!@#$%^&*()_+-=[]{}|;':\",./<>?`~123DEF"
		mut := Mutable(input)
		mut.ToTitle()
		if got := string(mut); got != expected {
			t.Errorf("ToTitle() with graphical chars = %q, want %q", got, expected)
		}
	})

	t.Run("ToLower preserves all non-letter bytes", func(t *testing.T) {
		// Test that bytes 0-64 and 91-96 and 123-127 are preserved
		// (all ASCII except a-z and A-Z)
		input := make([]byte, 0, 128)
		var expected []byte
		// Add all ASCII characters 0-127
		for b := range byte(128) {
			input = append(input, b)
			// Convert A-Z to a-z, leave everything else unchanged
			if b >= 'A' && b <= 'Z' {
				expected = append(expected, b+('a'-'A'))
			} else {
				expected = append(expected, b)
			}
		}
		mut := Mutable(input)
		mut.ToLower()
		if got := []byte(mut); !bytes.Equal(got, expected) {
			t.Errorf("ToLower() preserves non-letter bytes: got != expected")
			// Find first difference for debugging
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("First difference at byte %d: got %d, want %d", i, got[i], expected[i])
					break
				}
			}
		}
	})

	t.Run("ToUpper preserves all non-letter bytes", func(t *testing.T) {
		// Test that bytes 0-64 and 91-96 and 123-127 are preserved
		// (all ASCII except a-z and A-Z)
		input := make([]byte, 0, 128)
		var expected []byte
		// Add all ASCII characters 0-127
		for b := range byte(128) {
			input = append(input, b)
			// Convert a-z to A-Z, leave everything else unchanged
			if b >= 'a' && b <= 'z' {
				expected = append(expected, b-('a'-'A'))
			} else {
				expected = append(expected, b)
			}
		}
		mut := Mutable(input)
		mut.ToUpper()
		if got := []byte(mut); !bytes.Equal(got, expected) {
			t.Errorf("ToUpper() preserves non-letter bytes: got != expected")
			// Find first difference for debugging
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("First difference at byte %d: got %d, want %d", i, got[i], expected[i])
					break
				}
			}
		}
	})

	t.Run("Case conversion consistency", func(t *testing.T) {
		// Verify ToLower(ToUpper(x)) == ToLower(x) for all ASCII
		original := []byte("The Quick BROWN Fox 123!@# Jumps")

		mut1 := make(Mutable, len(original))
		copy(mut1, original)
		mut1.ToLower()
		lower1 := string(mut1)

		mut2 := make(Mutable, len(original))
		copy(mut2, original)
		mut2.ToUpper().ToLower()
		lower2 := string(mut2)

		if lower1 != lower2 {
			t.Errorf("ToLower consistency: ToLower(x)=%q != ToLower(ToUpper(x))=%q", lower1, lower2)
		}

		// Verify ToUpper(ToLower(x)) == ToUpper(x) for all ASCII
		mut3 := make(Mutable, len(original))
		copy(mut3, original)
		mut3.ToUpper()
		upper1 := string(mut3)

		mut4 := make(Mutable, len(original))
		copy(mut4, original)
		mut4.ToLower().ToUpper()
		upper2 := string(mut4)

		if upper1 != upper2 {
			t.Errorf("ToUpper consistency: ToUpper(x)=%q != ToUpper(ToLower(x))=%q", upper1, upper2)
		}
	})
}

func TestMutableReplaceStringVariants(t *testing.T) {
	// Test ReplaceString
	mut := Mutable([]byte("foo foo foo")) //nolint:dupword
	mut.ReplaceString("foo", "bar", 2)
	if got := string(mut); got != "bar bar foo" { //nolint:dupword
		t.Errorf("ReplaceString() = %q, want \"bar bar foo\"", got)
	}

	// Test ReplaceAllString
	mut = Mutable([]byte("foo foo foo")) //nolint:dupword
	mut.ReplaceAllString("foo", "bar")
	if got := string(mut); got != "bar bar bar" { //nolint:dupword
		t.Errorf("ReplaceAllString() = %q, want \"bar bar bar\"", got)
	}
}

// Coverage tests for previously untested functions

func TestMutableFormat(t *testing.T) {
	mut := Mutable([]byte("test"))
	result := fmt.Sprintf("%s", mut)
	if result != "test" {
		t.Errorf("Format() = %q, want \"test\"", result)
	}

	// Test with different format specifiers
	result = fmt.Sprintf("%v", mut)
	if result != "test" {
		t.Errorf("Format() with %%v = %q, want \"test\"", result)
	}
}

func TestMutableBytes(t *testing.T) {
	mut := Mutable([]byte("hello"))
	b := mut.Bytes()
	if string(b) != "hello" {
		t.Errorf("Bytes() = %q, want \"hello\"", b)
	}

	// Verify it returns the underlying slice
	if len(b) != 5 {
		t.Errorf("Bytes() len = %d, want 5", len(b))
	}
}

func TestMutableContainsRune(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		r    rune
		want bool
	}{
		{"ASCII present", []byte("hello"), 'h', true},
		{"ASCII not present", []byte("hello"), 'z', false},
		{"Unicode present", []byte("hello 世界"), '世', true},
		{"Unicode not present", []byte("hello"), '世', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := Mutable(tt.data)
			if got := mut.ContainsRune(tt.r); got != tt.want {
				t.Errorf("ContainsRune(%c) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestMutableCountString(t *testing.T) {
	mut := Mutable([]byte("hello hello hello")) //nolint:dupword
	if got := mut.CountString("hello"); got != 3 {
		t.Errorf("CountString() = %d, want 3", got)
	}

	if got := mut.CountString("world"); got != 0 {
		t.Errorf("CountString() = %d, want 0", got)
	}
}

func TestMutableEqualString(t *testing.T) {
	mut := Mutable([]byte("hello"))
	if !mut.EqualString("hello") {
		t.Error("EqualString() should return true for equal strings")
	}

	if mut.EqualString("world") {
		t.Error("EqualString() should return false for different strings")
	}
}

func TestMutableEqualFold(t *testing.T) {
	mut := Mutable([]byte("Hello"))
	if !mut.EqualFold([]byte("HELLO")) {
		t.Error("EqualFold() should be case-insensitive")
	}

	if mut.EqualFold([]byte("world")) {
		t.Error("EqualFold() should return false for different content")
	}
}

func TestMutableHasPrefixString(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	if !mut.HasPrefixString("hello") {
		t.Error("HasPrefixString() should return true")
	}

	if mut.HasPrefixString("world") {
		t.Error("HasPrefixString() should return false")
	}
}

func TestMutableHasSuffixString(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	if !mut.HasSuffixString("world") {
		t.Error("HasSuffixString() should return true")
	}

	if mut.HasSuffixString("hello") {
		t.Error("HasSuffixString() should return false")
	}
}

func TestMutableIndexString(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	if got := mut.IndexString("world"); got != 6 {
		t.Errorf("IndexString() = %d, want 6", got)
	}

	if got := mut.IndexString("foo"); got != -1 {
		t.Errorf("IndexString() = %d, want -1", got)
	}
}

func TestMutableIndexByte(t *testing.T) {
	mut := Mutable([]byte("hello"))
	if got := mut.IndexByte('e'); got != 1 {
		t.Errorf("IndexByte() = %d, want 1", got)
	}

	if got := mut.IndexByte('z'); got != -1 {
		t.Errorf("IndexByte() = %d, want -1", got)
	}
}

func TestMutableIndexRune(t *testing.T) {
	mut := Mutable([]byte("hello 世界"))
	if got := mut.IndexRune('世'); got != 6 {
		t.Errorf("IndexRune() = %d, want 6", got)
	}

	if got := mut.IndexRune('z'); got != -1 {
		t.Errorf("IndexRune() = %d, want -1", got)
	}
}

func TestMutableLastIndex(t *testing.T) {
	mut := Mutable([]byte("hello hello")) //nolint:dupword
	if got := mut.LastIndex([]byte("hello")); got != 6 {
		t.Errorf("LastIndex() = %d, want 6", got)
	}

	if got := mut.LastIndex([]byte("world")); got != -1 {
		t.Errorf("LastIndex() = %d, want -1", got)
	}
}

func TestMutableLastIndexString(t *testing.T) {
	mut := Mutable([]byte("hello hello")) //nolint:dupword
	if got := mut.LastIndexString("hello"); got != 6 {
		t.Errorf("LastIndexString() = %d, want 6", got)
	}

	if got := mut.LastIndexString("world"); got != -1 {
		t.Errorf("LastIndexString() = %d, want -1", got)
	}
}

func TestMutableLastIndexByte(t *testing.T) {
	mut := Mutable([]byte("hello"))
	if got := mut.LastIndexByte('l'); got != 3 {
		t.Errorf("LastIndexByte() = %d, want 3", got)
	}

	if got := mut.LastIndexByte('z'); got != -1 {
		t.Errorf("LastIndexByte() = %d, want -1", got)
	}
}

func TestMutableCutPrefix(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	after, found := mut.CutPrefix([]byte("hello "))
	if !found || string(after) != "world" {
		t.Errorf("CutPrefix() = (%q, %v), want (\"world\", true)", after, found)
	}

	mut = Mutable([]byte("hello world"))
	_,
		found = mut.CutPrefix([]byte("foo"))
	if found {
		t.Error("CutPrefix() should return false for non-matching prefix")
	}
}

func TestMutableCutSuffix(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	before, found := mut.CutSuffix([]byte(" world"))
	if !found || string(before) != "hello" {
		t.Errorf("CutSuffix() = (%q, %v), want (\"hello\", true)", before, found)
	}

	mut = Mutable([]byte("hello world"))
	_, found = mut.CutSuffix([]byte("foo"))
	if found {
		t.Error("CutSuffix() should return false for non-matching suffix")
	}
}

func TestMutableSplitString(t *testing.T) {
	mut := Mutable([]byte("a,b,c"))
	parts := slices.Collect(mut.SplitString(","))
	if len(parts) != 3 || string(parts[0]) != "a" || string(parts[1]) != "b" || string(parts[2]) != "c" {
		t.Errorf("SplitString() failed, got %v", parts)
	}
}

func TestMutableSplitN(t *testing.T) {
	mut := Mutable([]byte("a,b,c,d"))
	parts := slices.Collect(mut.SplitN([]byte(","), 2))
	if len(parts) != 2 || string(parts[0]) != "a" || string(parts[1]) != "b,c,d" {
		t.Errorf("SplitN() failed, got %v", parts)
	}
}

func TestMutableSplitAfter(t *testing.T) {
	mut := Mutable([]byte("a,b,c"))
	parts := slices.Collect(mut.SplitAfter([]byte(",")))
	if len(parts) != 3 || string(parts[0]) != "a," || string(parts[1]) != "b," || string(parts[2]) != "c" {
		t.Errorf("SplitAfter() failed, got %v", parts)
	}
}

func TestMutableSplitAfterN(t *testing.T) {
	mut := Mutable([]byte("a,b,c,d"))
	parts := slices.Collect(mut.SplitAfterN([]byte(","), 2))
	if len(parts) != 2 || string(parts[0]) != "a," || string(parts[1]) != "b,c,d" {
		t.Errorf("SplitAfterN() failed, got %v", parts)
	}
}

// TestIteratorConformance ensures iterator-based methods match bytes package behavior.
func TestIteratorConformance(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
		sep  []byte
		n    int
	}{
		{"simple comma", []byte("a,b,c"), []byte(","), -1},
		{"empty separator", []byte("abc"), []byte(""), -1},
		{"no match", []byte("abc"), []byte("x"), -1},
		{"empty data", []byte(""), []byte(","), -1},
		{"separator at start", []byte(",a,b"), []byte(","), -1},
		{"separator at end", []byte("a,b,"), []byte(","), -1},
		{"consecutive separators", []byte("a,,b"), []byte(","), -1},
		{"multi-byte separator", []byte("a::b::c"), []byte("::"), -1},
		{"unicode data", []byte("hello 世界 test"), []byte(" "), -1},
		{"n=0", []byte("a,b,c"), []byte(","), 0},
		{"n=1", []byte("a,b,c"), []byte(","), 1},
		{"n=2", []byte("a,b,c"), []byte(","), 2},
		{"n=10 (more than matches)", []byte("a,b,c"), []byte(","), 10},
	}

	t.Run("Split", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mut := Mutable(tc.data)
				var expected [][]byte
				if tc.n >= 0 {
					expected = bytes.SplitN(tc.data, tc.sep, tc.n)
				} else {
					expected = bytes.Split(tc.data, tc.sep)
				}

				var got []Mutable
				if tc.n >= 0 {
					got = slices.Collect(mut.SplitN(tc.sep, tc.n))
				} else {
					got = slices.Collect(mut.Split(tc.sep))
				}

				if len(got) != len(expected) {
					t.Errorf("length mismatch: got %d, want %d", len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("part[%d]: got %q, want %q", i, got[i], expected[i])
					}
				}
			})
		}
	})

	t.Run("SplitAfter", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mut := Mutable(tc.data)
				var expected [][]byte
				if tc.n >= 0 {
					expected = bytes.SplitAfterN(tc.data, tc.sep, tc.n)
				} else {
					expected = bytes.SplitAfter(tc.data, tc.sep)
				}

				var got []Mutable
				if tc.n >= 0 {
					got = slices.Collect(mut.SplitAfterN(tc.sep, tc.n))
				} else {
					got = slices.Collect(mut.SplitAfter(tc.sep))
				}

				if len(got) != len(expected) {
					t.Errorf("length mismatch: got %d, want %d", len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("part[%d]: got %q, want %q", i, got[i], expected[i])
					}
				}
			})
		}
	})

	t.Run("Fields", func(t *testing.T) {
		fieldTests := [][]byte{
			[]byte("hello world"),
			[]byte("  hello   world  "),
			[]byte("\t\nhello\t\nworld\t\n"),
			[]byte(""),
			[]byte("   "),
			[]byte("single"),
			[]byte("hello 世界 test"),
		}

		for _, data := range fieldTests {
			t.Run(string(data), func(t *testing.T) {
				mut := Mutable(data)
				expected := bytes.Fields(data)
				got := slices.Collect(mut.Fields())

				if len(got) != len(expected) {
					t.Errorf("length mismatch: got %d, want %d", len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("field[%d]: got %q, want %q", i, got[i], expected[i])
					}
				}
			})
		}
	})

	t.Run("FieldsFunc", func(t *testing.T) {
		isComma := func(r rune) bool { return r == ',' }
		isSpace := func(r rune) bool { return r == ' ' || r == '\t' }

		funcTests := []struct {
			name string
			data []byte
			fn   func(rune) bool
		}{
			{"comma separator", []byte("a,b,c"), isComma},
			{"space separator", []byte("a b c"), isSpace},
			{"empty", []byte(""), isComma},
			{"no matches", []byte("abc"), isComma},
			{"consecutive matches", []byte("a,,b"), isComma},
			{"unicode", []byte("hello 世界"), isSpace},
		}

		for _, tc := range funcTests {
			t.Run(tc.name, func(t *testing.T) {
				mut := Mutable(tc.data)
				expected := bytes.FieldsFunc(tc.data, tc.fn)
				got := slices.Collect(mut.FieldsFunc(tc.fn))

				if len(got) != len(expected) {
					t.Errorf("length mismatch: got %d, want %d", len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("field[%d]: got %q, want %q", i, got[i], expected[i])
					}
				}
			})
		}
	})
}

// TestUncoveredPaths covers code paths not hit by other tests.
func TestUncoveredPaths(t *testing.T) {
	t.Run("SplitN with empty separator", func(t *testing.T) {
		mut := Mutable([]byte("abc"))

		// n=1 with empty separator
		parts := slices.Collect(mut.SplitN([]byte(""), 1))
		if len(parts) != 1 || string(parts[0]) != "abc" {
			t.Errorf("SplitN empty sep n=1: got %v", parts)
		}

		// n=2 with empty separator
		parts = slices.Collect(mut.SplitN([]byte(""), 2))
		if len(parts) != 2 || string(parts[0]) != "a" || string(parts[1]) != "bc" {
			t.Errorf("SplitN empty sep n=2: got %v", parts)
		}

		// n=0 with empty separator
		parts = slices.Collect(mut.SplitN([]byte(""), 0))
		if len(parts) != 0 {
			t.Errorf("SplitN empty sep n=0: got %v, want empty", parts)
		}
	})

	t.Run("SplitAfterN with empty separator", func(t *testing.T) {
		mut := Mutable([]byte("abc"))

		// n=1 with empty separator
		parts := slices.Collect(mut.SplitAfterN([]byte(""), 1))
		if len(parts) != 1 || string(parts[0]) != "abc" {
			t.Errorf("SplitAfterN empty sep n=1: got %v", parts)
		}

		// n=2 with empty separator
		parts = slices.Collect(mut.SplitAfterN([]byte(""), 2))
		if len(parts) != 2 || string(parts[0]) != "a" || string(parts[1]) != "bc" {
			t.Errorf("SplitAfterN empty sep n=2: got %v", parts)
		}
	})

	t.Run("Split early termination", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c,d,e"))
		count := 0
		for range mut.Split([]byte(",")) {
			count++
			if count == 2 {
				break // Early termination
			}
		}
		if count != 2 {
			t.Errorf("Split early termination: counted %d items", count)
		}
	})

	t.Run("SplitN early termination", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c,d,e"))
		count := 0
		for range mut.SplitN([]byte(","), 10) {
			count++
			if count == 2 {
				break // Early termination
			}
		}
		if count != 2 {
			t.Errorf("SplitN early termination: counted %d items", count)
		}
	})

	t.Run("SplitN with empty separator early termination", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.SplitN([]byte(""), 10) {
			count++
			if count == 2 {
				break // Early termination
			}
		}
		if count != 2 {
			t.Errorf("SplitN empty sep early termination: counted %d items", count)
		}
	})

	t.Run("SplitAfterN early termination", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c,d,e"))
		count := 0
		for range mut.SplitAfterN([]byte(","), 10) {
			count++
			if count == 2 {
				break // Early termination
			}
		}
		if count != 2 {
			t.Errorf("SplitAfterN early termination: counted %d items", count)
		}
	})

	t.Run("Replace growing with capacity", func(t *testing.T) {
		// Test growing replacement that fits in capacity
		data := []byte("aa aa aa") //nolint:dupword
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.ReplaceAll([]byte("aa"), []byte("bbbb"))
		if string(mut) != "bbbb bbbb bbbb" { //nolint:dupword
			t.Errorf("Replace growing: got %q", string(mut))
		}
	})

	t.Run("Replace same length with n limit", func(t *testing.T) {
		mut := Mutable([]byte("aa bb aa cc aa"))
		mut.Replace([]byte("aa"), []byte("xx"), 2)
		if string(mut) != "xx bb xx cc aa" {
			t.Errorf("Replace same length n=2: got %q", string(mut))
		}
	})

	t.Run("Replace shrinking with n limit", func(t *testing.T) {
		data := []byte("aaaa bbbb aaaa cccc aaaa")
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.Replace([]byte("aaaa"), []byte("x"), 2)
		// Verify result matches bytes.Replace behavior
		expected := string(bytes.Replace(data, []byte("aaaa"), []byte("x"), 2))
		if string(mut) != expected {
			t.Errorf("Replace shrinking n=2: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("Replace growing with n limit", func(t *testing.T) {
		data := []byte("a b a c a")
		mut := make(Mutable, len(data), len(data)+20)
		copy(mut, data)
		mut.Replace([]byte("a"), []byte("xxxx"), 2)
		expected := "xxxx b xxxx c a"
		if string(mut) != expected {
			t.Errorf("Replace growing n=2: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("ToTitle with non-ASCII", func(t *testing.T) {
		mut := Mutable([]byte("hello мир"))
		mut.ToTitle()
		// Should have non-ASCII and trigger Unicode path
		if mut.IsASCII() {
			t.Errorf("ToTitle should have detected non-ASCII")
		}
	})

	t.Run("Repeat with count 0", func(t *testing.T) {
		mut := Mutable([]byte("test"))
		mut.Repeat(0)
		if len(mut) != 0 {
			t.Errorf("Repeat(0): got len %d, want 0", len(mut))
		}
	})

	t.Run("Repeat with negative count", func(t *testing.T) {
		mut := Mutable([]byte("test"))
		mut.Repeat(-5)
		if len(mut) != 0 {
			t.Errorf("Repeat(-5): got len %d, want 0", len(mut))
		}
	})

	t.Run("Fields empty after trimming", func(t *testing.T) {
		mut := Mutable([]byte("   "))
		parts := slices.Collect(mut.Fields())
		if len(parts) != 0 {
			t.Errorf("Fields on all spaces: got %d parts, want 0", len(parts))
		}
	})

	t.Run("Fields single character", func(t *testing.T) {
		mut := Mutable([]byte("a"))
		parts := slices.Collect(mut.Fields())
		if len(parts) != 1 || string(parts[0]) != "a" {
			t.Errorf("Fields single char: got %v", parts)
		}
	})

	t.Run("FieldsFunc empty result", func(t *testing.T) {
		mut := Mutable([]byte(",,,"))
		isComma := func(r rune) bool { return r == ',' }
		parts := slices.Collect(mut.FieldsFunc(isComma))
		if len(parts) != 0 {
			t.Errorf("FieldsFunc all separators: got %d parts, want 0", len(parts))
		}
	})

	t.Run("Split with empty separator on empty data", func(t *testing.T) {
		mut := Mutable([]byte(""))
		parts := slices.Collect(mut.Split([]byte("")))
		if len(parts) != 0 {
			t.Errorf("Split empty/empty: got %d parts, want 0", len(parts))
		}
	})

	t.Run("SplitAfter with empty separator on empty data", func(t *testing.T) {
		mut := Mutable([]byte(""))
		parts := slices.Collect(mut.SplitAfter([]byte("")))
		if len(parts) != 0 {
			t.Errorf("SplitAfter empty/empty: got %d parts, want 0", len(parts))
		}
	})

	t.Run("Repeat with count 1", func(t *testing.T) {
		mut := Mutable([]byte("test"))
		result := mut.Repeat(1)
		if string(*result) != "test" {
			t.Errorf("Repeat(1): got %q, want \"test\"", string(*result))
		}
	})

	t.Run("Fields early termination", func(t *testing.T) {
		mut := Mutable([]byte("a b c d e"))
		count := 0
		for range mut.Fields() {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("Fields early termination: counted %d items", count)
		}
	})

	t.Run("FieldsFunc early termination", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c,d"))
		isComma := func(r rune) bool { return r == ',' }
		count := 0
		for range mut.FieldsFunc(isComma) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("FieldsFunc early termination: counted %d items", count)
		}
	})

	t.Run("SplitAfter early termination", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c,d"))
		count := 0
		for range mut.SplitAfter([]byte(",")) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfter early termination: counted %d items", count)
		}
	})

	t.Run("SplitAfterN with empty separator early termination", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.SplitAfterN([]byte(""), 10) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfterN empty sep early termination: counted %d items", count)
		}
	})

	t.Run("Split with empty separator early termination", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.Split([]byte("")) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("Split empty sep early termination: counted %d items", count)
		}
	})

	t.Run("SplitAfter with empty separator early termination", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.SplitAfter([]byte("")) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfter empty sep early termination: counted %d items", count)
		}
	})

	t.Run("Replace with n=0", func(t *testing.T) {
		mut := Mutable([]byte("foo bar foo"))
		mut.Replace([]byte("foo"), []byte("baz"), 0)
		if string(mut) != "foo bar foo" {
			t.Errorf("Replace n=0: got %q, want no change", string(mut))
		}
	})

	t.Run("Replace with empty old", func(t *testing.T) {
		mut := Mutable([]byte("foo"))
		mut.Replace([]byte(""), []byte("x"), 5)
		if string(mut) != "foo" {
			t.Errorf("Replace empty old: got %q, want no change", string(mut))
		}
	})

	t.Run("Repeat fast path with sufficient capacity", func(t *testing.T) {
		// Create a Mutable with pre-allocated capacity
		data := []byte("ab")
		mut := make(Mutable, len(data), len(data)*5)
		copy(mut, data)
		mut.Repeat(5)
		if string(mut) != "ababababab" {
			t.Errorf("Repeat fast path: got %q, want \"ababababab\"", string(mut))
		}
	})

	t.Run("Repeat slow path without capacity", func(t *testing.T) {
		// Create a Mutable without extra capacity
		mut := Mutable([]byte("ab"))
		mut.Repeat(5)
		if string(mut) != "ababababab" {
			t.Errorf("Repeat slow path: got %q, want \"ababababab\"", string(mut))
		}
	})

	t.Run("SplitN with n equals match count", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitN([]byte(","), 3))
		// With n=3, we should get exactly 3 parts
		if len(parts) != 3 {
			t.Errorf("SplitN n=3: got %d parts, want 3", len(parts))
		}
	})

	t.Run("SplitAfterN with n equals match count", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitAfterN([]byte(","), 3))
		// With n=3, we should get exactly 3 parts
		if len(parts) != 3 {
			t.Errorf("SplitAfterN n=3: got %d parts, want 3", len(parts))
		}
	})

	t.Run("Replace same length n=-1", func(t *testing.T) {
		mut := Mutable([]byte("aa bb aa cc aa"))
		mut.Replace([]byte("aa"), []byte("xx"), -1)
		if string(mut) != "xx bb xx cc xx" {
			t.Errorf("Replace same length n=-1: got %q", string(mut))
		}
	})

	t.Run("Replace growing with zero matches", func(t *testing.T) {
		data := []byte("foo bar")
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.Replace([]byte("xyz"), []byte("abcd"), -1)
		if string(mut) != "foo bar" {
			t.Errorf("Replace growing no match: got %q", string(mut))
		}
	})

	t.Run("SplitN with n=1 and separator present", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitN([]byte(","), 1))
		if len(parts) != 1 || string(parts[0]) != "a,b,c" {
			t.Errorf("SplitN n=1: got %v, want full string", parts)
		}
	})

	t.Run("SplitAfterN with n=1 and separator present", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitAfterN([]byte(","), 1))
		if len(parts) != 1 || string(parts[0]) != "a,b,c" {
			t.Errorf("SplitAfterN n=1: got %v, want full string", parts)
		}
	})
}

// TestStringWrappers tests all String variant wrapper methods.
func TestStringWrappers(t *testing.T) {
	t.Run("SplitString", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitString(","))
		if len(parts) != 3 || string(parts[0]) != "a" {
			t.Errorf("SplitString failed: got %v", parts)
		}
	})

	t.Run("SplitNString", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitNString(",", 2))
		if len(parts) != 2 || string(parts[0]) != "a" {
			t.Errorf("SplitNString failed: got %v", parts)
		}
	})

	t.Run("SplitAfterString", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitAfterString(","))
		if len(parts) != 3 || string(parts[0]) != "a," {
			t.Errorf("SplitAfterString failed: got %v", parts)
		}
	})

	t.Run("SplitAfterNString", func(t *testing.T) {
		mut := Mutable([]byte("a,b,c"))
		parts := slices.Collect(mut.SplitAfterNString(",", 2))
		if len(parts) != 2 || string(parts[0]) != "a," {
			t.Errorf("SplitAfterNString failed: got %v", parts)
		}
	})

	t.Run("ReplaceString", func(t *testing.T) {
		mut := Mutable([]byte("foo bar foo"))
		mut.ReplaceString("foo", "baz", 1)
		if string(mut) != "baz bar foo" {
			t.Errorf("ReplaceString failed: got %q", string(mut))
		}
	})

	t.Run("ReplaceAll", func(t *testing.T) {
		mut := Mutable([]byte("foo bar foo"))
		mut.ReplaceAll([]byte("foo"), []byte("baz"))
		if string(mut) != "baz bar baz" {
			t.Errorf("ReplaceAll failed: got %q", string(mut))
		}
	})

	t.Run("ReplaceAllString", func(t *testing.T) {
		mut := Mutable([]byte("foo bar foo"))
		mut.ReplaceAllString("foo", "baz")
		if string(mut) != "baz bar baz" {
			t.Errorf("ReplaceAllString failed: got %q", string(mut))
		}
	})
}

// TestTrimMethods tests all Trim variant methods.
func TestTrimMethods(t *testing.T) {
	t.Run("Trim", func(t *testing.T) {
		mut := Mutable([]byte("!!!hello!!!"))
		mut.Trim("!")
		if string(mut) != "hello" {
			t.Errorf("Trim failed: got %q", string(mut))
		}
	})

	t.Run("TrimLeft", func(t *testing.T) {
		mut := Mutable([]byte("!!!hello!!!"))
		mut.TrimLeft("!")
		if string(mut) != "hello!!!" {
			t.Errorf("TrimLeft failed: got %q", string(mut))
		}
	})

	t.Run("TrimRight", func(t *testing.T) {
		mut := Mutable([]byte("!!!hello!!!"))
		mut.TrimRight("!")
		if string(mut) != "!!!hello" {
			t.Errorf("TrimRight failed: got %q", string(mut))
		}
	})

	t.Run("TrimSpace", func(t *testing.T) {
		mut := Mutable([]byte("  hello  "))
		mut.TrimSpace()
		if string(mut) != "hello" {
			t.Errorf("TrimSpace failed: got %q", string(mut))
		}
	})

	t.Run("TrimPrefix", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.TrimPrefix([]byte("hello "))
		if string(mut) != "world" {
			t.Errorf("TrimPrefix failed: got %q", string(mut))
		}
	})

	t.Run("TrimPrefixString", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.TrimPrefixString("hello ")
		if string(mut) != "world" {
			t.Errorf("TrimPrefixString failed: got %q", string(mut))
		}
	})

	t.Run("TrimSuffix", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.TrimSuffix([]byte(" world"))
		if string(mut) != "hello" {
			t.Errorf("TrimSuffix failed: got %q", string(mut))
		}
	})

	t.Run("TrimSuffixString", func(t *testing.T) {
		mut := Mutable([]byte("hello world"))
		mut.TrimSuffixString(" world")
		if string(mut) != "hello" {
			t.Errorf("TrimSuffixString failed: got %q", string(mut))
		}
	})

	t.Run("TrimFunc", func(t *testing.T) {
		mut := Mutable([]byte("123hello456"))
		isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
		mut.TrimFunc(isDigit)
		if string(mut) != "hello" {
			t.Errorf("TrimFunc failed: got %q", string(mut))
		}
	})

	t.Run("TrimLeftFunc", func(t *testing.T) {
		mut := Mutable([]byte("123hello456"))
		isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
		mut.TrimLeftFunc(isDigit)
		if string(mut) != "hello456" {
			t.Errorf("TrimLeftFunc failed: got %q", string(mut))
		}
	})

	t.Run("TrimRightFunc", func(t *testing.T) {
		mut := Mutable([]byte("123hello456"))
		isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
		mut.TrimRightFunc(isDigit)
		if string(mut) != "123hello" {
			t.Errorf("TrimRightFunc failed: got %q", string(mut))
		}
	})
}

// TestMapAndUnicode tests Map and IsUnicode methods.
func TestMapAndUnicode(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		mut := Mutable([]byte("hello"))
		toUpper := func(r rune) rune {
			if r >= 'a' && r <= 'z' {
				return r - ('a' - 'A')
			}
			return r
		}
		mut.Map(toUpper)
		if string(mut) != "HELLO" {
			t.Errorf("Map failed: got %q", string(mut))
		}
	})

	t.Run("IsUnicode", func(t *testing.T) {
		mut := Mutable([]byte("hello"))
		if !mut.IsUnicode() {
			t.Errorf("IsUnicode failed for valid UTF-8")
		}

		// Invalid UTF-8
		mut = Mutable([]byte{0xff, 0xfe, 0xfd})
		if mut.IsUnicode() {
			t.Errorf("IsUnicode should return false for invalid UTF-8")
		}
	})
}

// TestSplitEdgeCases tests specific edge cases to reach 100% coverage.
func TestSplitEdgeCases(t *testing.T) {
	t.Run("SplitN empty separator n>1", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		parts := slices.Collect(mut.SplitN([]byte(""), 3))
		// Should split into individual bytes up to n-1, then remainder
		if len(parts) != 3 {
			t.Errorf("SplitN empty sep n=3: got %d parts, want 3", len(parts))
		}
		if string(parts[2]) != "cde" {
			t.Errorf("SplitN empty sep n=3: last part got %q, want \"cde\"", string(parts[2]))
		}
	})

	t.Run("SplitAfterN empty separator n>1", func(t *testing.T) {
		mut := Mutable([]byte("abcde"))
		parts := slices.Collect(mut.SplitAfterN([]byte(""), 3))
		if len(parts) != 3 {
			t.Errorf("SplitAfterN empty sep n=3: got %d parts, want 3", len(parts))
		}
		if string(parts[2]) != "cde" {
			t.Errorf("SplitAfterN empty sep n=3: last part got %q, want \"cde\"", string(parts[2]))
		}
	})

	t.Run("SplitN n<0 with early termination", func(t *testing.T) {
		// Test early return when yield returns false during unlimited mode
		mut := Mutable([]byte("a,b,c,d,e"))
		count := 0
		for range mut.SplitN([]byte(","), -1) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitN n=-1 early term: counted %d", count)
		}
	})

	t.Run("SplitN empty sep with early termination", func(t *testing.T) {
		// Test early return during empty separator iteration
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.SplitN([]byte(""), 10) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitN empty sep early term: counted %d", count)
		}
	})

	t.Run("SplitAfterN n<0 with early termination", func(t *testing.T) {
		// Test early return when yield returns false during unlimited mode
		mut := Mutable([]byte("a,b,c,d,e"))
		count := 0
		for range mut.SplitAfterN([]byte(","), -1) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfterN n=-1 early term: counted %d", count)
		}
	})

	t.Run("SplitAfterN empty sep with early termination", func(t *testing.T) {
		// Test early return during empty separator iteration
		mut := Mutable([]byte("abcde"))
		count := 0
		for range mut.SplitAfterN([]byte(""), 10) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfterN empty sep early term: counted %d", count)
		}
	})

	t.Run("SplitN regular with early termination", func(t *testing.T) {
		// Test early return during regular separator iteration
		mut := Mutable([]byte("a,b,c,d"))
		count := 0
		for range mut.SplitN([]byte(","), 10) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitN regular early term: counted %d", count)
		}
	})

	t.Run("SplitAfterN regular with early termination", func(t *testing.T) {
		// Test early return during regular separator iteration
		mut := Mutable([]byte("a,b,c,d"))
		count := 0
		for range mut.SplitAfterN([]byte(","), 10) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("SplitAfterN regular early term: counted %d", count)
		}
	})

	t.Run("Split no separator found - consume final element", func(t *testing.T) {
		mut := Mutable([]byte("abc"))
		parts := slices.Collect(mut.Split([]byte(",")))
		if len(parts) != 1 || string(parts[0]) != "abc" {
			t.Errorf("Split no sep: got %v", parts)
		}
	})

	t.Run("Replace no matches", func(t *testing.T) {
		// Test Replace when no matches found
		mut := Mutable([]byte("foo bar"))
		mut.Replace([]byte("xyz"), []byte("abc"), -1)
		if string(mut) != "foo bar" {
			t.Errorf("Replace no match: got %q", string(mut))
		}
	})

	t.Run("replaceInPlace growing with prefix", func(t *testing.T) {
		// Test pattern that results in readPos > 0 after backward copying
		// This happens when growing replacements occur at the start
		data := []byte("aaa bbb ccc")
		mut := make(Mutable, len(data), len(data)+20)
		copy(mut, data)
		mut.Replace([]byte("aaa"), []byte("xxxxx"), -1)
		expected := "xxxxx bbb ccc"
		if string(mut) != expected {
			t.Errorf("Replace growing at start: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace growing multiple matches", func(t *testing.T) {
		// Test multiple growing replacements to exercise backward copy logic
		data := []byte("a b a c a")
		mut := make(Mutable, len(data), len(data)+30)
		copy(mut, data)
		mut.Replace([]byte("a"), []byte("xxxx"), -1)
		expected := "xxxx b xxxx c xxxx"
		if string(mut) != expected {
			t.Errorf("Replace growing multiple: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace growing zero matches after resize", func(t *testing.T) {
		// Test the path where we expand the slice but find zero matches
		// This is hard to hit naturally but tests defensive code
		data := []byte("foo bar baz")
		mut := make(Mutable, len(data), len(data)+20)
		copy(mut, data)
		// This should not match anything and test the zero matches path
		mut.Replace([]byte("xyz"), []byte("abcdefgh"), -1)
		if string(mut) != "foo bar baz" {
			t.Errorf("Replace growing zero matches: got %q", string(mut))
		}
	})

	t.Run("SplitN negative n values", func(t *testing.T) {
		testCases := []struct {
			name string
			data []byte
			sep  []byte
			n    int
		}{
			{"n=-1", []byte("a,b,c,d"), []byte(","), -1},
			{"n=-2", []byte("a,b,c,d"), []byte(","), -2},
			{"n=-10", []byte("a,b,c,d"), []byte(","), -10},
			{"n=-1 empty sep", []byte("abc"), []byte(""), -1},
			{"n=-5 empty sep", []byte("abc"), []byte(""), -5},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mut := Mutable(tc.data)
				// Compare with stdlib behavior
				expected := bytes.SplitN(tc.data, tc.sep, tc.n)
				got := slices.Collect(mut.SplitN(tc.sep, tc.n))

				if len(got) != len(expected) {
					t.Errorf("SplitN %s: got %d parts, want %d", tc.name, len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("SplitN %s part[%d]: got %q, want %q", tc.name, i, got[i], expected[i])
					}
				}
			})
		}
	})

	t.Run("SplitAfterN negative n values", func(t *testing.T) {
		testCases := []struct {
			name string
			data []byte
			sep  []byte
			n    int
		}{
			{"n=-1", []byte("a,b,c,d"), []byte(","), -1},
			{"n=-2", []byte("a,b,c,d"), []byte(","), -2},
			{"n=-10", []byte("a,b,c,d"), []byte(","), -10},
			{"n=-1 empty sep", []byte("abc"), []byte(""), -1},
			{"n=-5 empty sep", []byte("abc"), []byte(""), -5},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mut := Mutable(tc.data)
				// Compare with stdlib behavior
				expected := bytes.SplitAfterN(tc.data, tc.sep, tc.n)
				got := slices.Collect(mut.SplitAfterN(tc.sep, tc.n))

				if len(got) != len(expected) {
					t.Errorf("SplitAfterN %s: got %d parts, want %d", tc.name, len(got), len(expected))
					return
				}
				for i := range got {
					if !bytes.Equal(got[i], expected[i]) {
						t.Errorf("SplitAfterN %s part[%d]: got %q, want %q", tc.name, i, got[i], expected[i])
					}
				}
			})
		}
	})

	t.Run("Replace slow path - insufficient capacity", func(t *testing.T) {
		// Test Replace when result doesn't fit in capacity (slow path)
		mut := Mutable([]byte("foo foo foo")) //nolint:dupword
		// No extra capacity, replacement grows, should allocate
		mut.Replace([]byte("foo"), []byte("foobar"), -1)
		expected := "foobar foobar foobar" //nolint:dupword
		if string(mut) != expected {
			t.Errorf("Replace slow path: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace same length all matches", func(t *testing.T) {
		// Test same-length replacement with n=-1 (all matches)
		data := []byte("foo bar foo baz foo")
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.Replace([]byte("foo"), []byte("xxx"), -1)
		expected := "xxx bar xxx baz xxx"
		if string(mut) != expected {
			t.Errorf("Replace same length all: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace growing with prefix copy", func(t *testing.T) {
		// Test growing replacement where first match is not at position 0
		// This exercises the readPos > 0 path (line 702-704)
		data := []byte("prefix aaa suffix")
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.Replace([]byte("aaa"), []byte("xxxxx"), -1)
		expected := "prefix xxxxx suffix"
		if string(mut) != expected {
			t.Errorf("Replace growing with prefix: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace growing match at start", func(t *testing.T) {
		// Test growing replacement where match is at the very start
		// This should result in readPos=0, so no prefix copy needed
		data := []byte("aaa suffix")
		mut := make(Mutable, len(data), len(data)+10)
		copy(mut, data)
		mut.Replace([]byte("aaa"), []byte("xxxxx"), -1)
		expected := "xxxxx suffix"
		if string(mut) != expected {
			t.Errorf("Replace growing at start: got %q, want %q", string(mut), expected)
		}
	})

	t.Run("replaceInPlace growing multiple with prefix", func(t *testing.T) {
		// Test growing with multiple matches, first not at position 0
		// This ensures we hit the prefix copy path
		data := []byte("xx a b a c a")
		mut := make(Mutable, len(data), len(data)+50)
		copy(mut, data)
		mut.Replace([]byte("a"), []byte("xxxx"), -1)
		expected := "xx xxxx b xxxx c xxxx"
		if string(mut) != expected {
			t.Errorf("Replace growing multiple with prefix: got %q, want %q", string(mut), expected)
		}
	})
}

// TestReplaceShrinking tests that shrinking replacements correctly fall back
// to bytes.Replace instead of attempting in-place optimization (which would
// require allocating a copy anyway). This is a regression test for a bug
// where in-place shrinking caused data corruption.
func TestReplaceShrinking(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
		old  []byte
		new  []byte
		n    int
		want string
	}{
		{
			name: "shrinking multiple matches exhaust all",
			data: []byte("hello world hello universe"),
			old:  []byte("hello"),
			new:  []byte("hi"),
			n:    -1,
			want: "hi world hi universe",
		},
		{
			name: "shrinking three matches",
			data: []byte("foo bar foo baz foo"),
			old:  []byte("foo"),
			new:  []byte("x"),
			n:    -1,
			want: "x bar x baz x",
		},
		{
			name: "shrinking with n limit",
			data: []byte("test test test test"), //nolint:dupword
			old:  []byte("test"),
			new:  []byte("x"),
			n:    3,
			want: "x x x test", //nolint:dupword
		},
		{
			name: "shrinking long to short",
			data: []byte("aaaaa bbbbb aaaaa ccccc aaaaa"),
			old:  []byte("aaaaa"),
			new:  []byte("x"),
			n:    -1,
			want: "x bbbbb x ccccc x",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test with capacity to use in-place shrinking path
			mut := make(Mutable, len(tc.data), len(tc.data)+10)
			copy(mut, tc.data)

			// Verify against stdlib behavior
			expected := string(bytes.Replace(tc.data, tc.old, tc.new, tc.n))
			if tc.want != expected {
				t.Fatalf("Test case wants %q but stdlib gives %q", tc.want, expected)
			}

			mut.Replace(tc.old, tc.new, tc.n)
			got := string(mut)

			if got != tc.want {
				t.Errorf("Replace shrinking: got %q, want %q", got, tc.want)
			}

			// Also verify it matches stdlib exactly
			if got != expected {
				t.Errorf("Replace doesn't match stdlib: got %q, stdlib %q", got, expected)
			}
		})
	}
}

func TestMutableReplaceAll(t *testing.T) {
	mut := Mutable([]byte("foo foo foo")) //nolint:dupword
	mut.ReplaceAll([]byte("foo"), []byte("bar"))
	if got := string(mut); got != "bar bar bar" { //nolint:dupword
		t.Errorf("ReplaceAll() = %q, want \"bar bar bar\"", got)
	}
}

func TestMutableTrimLeft(t *testing.T) {
	mut := Mutable([]byte("  hello  "))
	mut.TrimLeft(" ")
	if got := string(mut); got != "hello  " {
		t.Errorf("TrimLeft() = %q, want \"hello  \"", got)
	}
}

func TestMutableTrimRight(t *testing.T) {
	mut := Mutable([]byte("  hello  "))
	mut.TrimRight(" ")
	if got := string(mut); got != "  hello" {
		t.Errorf("TrimRight() = %q, want \"  hello\"", got)
	}
}

func TestMutableTrimPrefixString(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	mut.TrimPrefixString("hello ")
	if got := string(mut); got != "world" {
		t.Errorf("TrimPrefixString() = %q, want \"world\"", got)
	}
}

func TestMutableTrimSuffixString(t *testing.T) {
	mut := Mutable([]byte("hello world"))
	mut.TrimSuffixString(" world")
	if got := string(mut); got != "hello" {
		t.Errorf("TrimSuffixString() = %q, want \"hello\"", got)
	}
}

func TestSprint(t *testing.T) {
	t.Run("basic usage", func(t *testing.T) {
		mut := Mprint("hello", "world")
		defer mut.Release()

		if got := mut.String(); got != "helloworld" {
			t.Errorf("Sprint() = %q, want %q", got, "helloworld")
		}
	})

	t.Run("with non-strings adds spaces", func(t *testing.T) {
		// Spaces are added between operands when neither is a string
		mut := Mprint(42, 43, 44)
		defer mut.Release()

		if got := mut.String(); got != "42 43 44" {
			t.Errorf("Sprint() = %q, want %q", got, "42 43 44")
		}
	})

	t.Run("mixed types no spaces", func(t *testing.T) {
		// No spaces when one operand is a string
		mut := Mprint("hello", 42, "world")
		defer mut.Release()

		if got := mut.String(); got != "hello42world" {
			t.Errorf("Sprint() = %q, want %q", got, "hello42world")
		}
	})

	t.Run("empty", func(t *testing.T) {
		mut := Mprint()
		defer mut.Release()

		if got := mut.String(); got != "" {
			t.Errorf("Sprint() = %q, want empty string", got)
		}
	})

	t.Run("single value", func(t *testing.T) {
		mut := Mprint(123)
		defer mut.Release()

		if got := mut.String(); got != "123" {
			t.Errorf("Sprint() = %q, want %q", got, "123")
		}
	})
}

func TestSprintf(t *testing.T) {
	t.Run("basic format", func(t *testing.T) {
		mut := Mprintf("hello %s", "world")
		defer mut.Release()

		if got := mut.String(); got != "hello world" {
			t.Errorf("Sprintf() = %q, want %q", got, "hello world")
		}
	})

	t.Run("multiple formats", func(t *testing.T) {
		mut := Mprintf("%s: %d items at $%.2f each", "Order", 3, 12.50)
		defer mut.Release()

		expected := "Order: 3 items at $12.50 each"
		if got := mut.String(); got != expected {
			t.Errorf("Sprintf() = %q, want %q", got, expected)
		}
	})

	t.Run("no arguments", func(t *testing.T) {
		mut := Mprintf("no formatting")
		defer mut.Release()

		if got := mut.String(); got != "no formatting" {
			t.Errorf("Sprintf() = %q, want %q", got, "no formatting")
		}
	})

	t.Run("complex formatting", func(t *testing.T) {
		mut := Mprintf("%04d-%02d-%02d", 2024, 3, 15)
		defer mut.Release()

		if got := mut.String(); got != "2024-03-15" {
			t.Errorf("Sprintf() = %q, want %q", got, "2024-03-15")
		}
	})
}

func TestSprintln(t *testing.T) {
	t.Run("basic usage", func(t *testing.T) {
		mut := Mprintln("hello", "world")
		defer mut.Release()

		if got := mut.String(); got != "hello world\n" {
			t.Errorf("Sprintln() = %q, want %q", got, "hello world\n")
		}
	})

	t.Run("with numbers", func(t *testing.T) {
		mut := Mprintln("count:", 42)
		defer mut.Release()

		if got := mut.String(); got != "count: 42\n" {
			t.Errorf("Sprintln() = %q, want %q", got, "count: 42\n")
		}
	})

	t.Run("empty", func(t *testing.T) {
		mut := Mprintln()
		defer mut.Release()

		if got := mut.String(); got != "\n" {
			t.Errorf("Sprintln() = %q, want newline", got)
		}
	})

	t.Run("single value", func(t *testing.T) {
		mut := Mprintln(123)
		defer mut.Release()

		if got := mut.String(); got != "123\n" {
			t.Errorf("Sprintln() = %q, want %q", got, "123\n")
		}
	})
}

func TestSprintFunctionsUsePool(t *testing.T) {
	// Verify that Sprint functions return pooled Mutables
	// that can be properly released and reused

	mut1 := Mprint("test1")
	mut1.Release()

	mut2 := Mprintf("test%d", 2)
	if mut2.Len() == 0 {
		t.Error("Sprintf should have written data")
	}
	mut2.Release()

	mut3 := Mprintln("test3")
	if mut3.Len() == 0 {
		t.Error("Sprintln should have written data")
	}
	mut3.Release()

	// Get another from pool - should be reset
	mut4 := NewMutable()
	if mut4.Len() != 0 {
		t.Errorf("Pooled Mutable not reset, Len() = %d, want 0", mut4.Len())
	}
	mut4.Release()
}

// Benchmark tests.
func BenchmarkMutableWrite(b *testing.B) {
	data := []byte("hello world")
	for i := 0; i < b.N; i++ {
		mut := Mutable(nil)
		mut.Write(data)
	}
}

func BenchmarkMutableWriteString(b *testing.B) {
	s := "hello world"
	for i := 0; i < b.N; i++ {
		mut := Mutable(nil)
		mut.WriteString(s)
	}
}

func BenchmarkMutableToLower(b *testing.B) {
	data := []byte("HELLO WORLD THIS IS A TEST")
	for i := 0; i < b.N; i++ {
		mut := Mutable(data)
		mut.ToLower()
	}
}

func BenchmarkMutableClone(b *testing.B) {
	original := Mutable([]byte("hello world this is a test"))
	for i := 0; i < b.N; i++ {
		_ = original.Clone()
	}
}
