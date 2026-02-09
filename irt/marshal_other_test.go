package irt

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"testing"
)

// Mock types for MarshalText tests

type mockTextMarshaler struct {
	data []byte
	err  error
}

func (m mockTextMarshaler) MarshalText() ([]byte, error) {
	return m.data, m.err
}

type mockJSONMarshaler struct {
	data []byte
	err  error
}

func (m mockJSONMarshaler) MarshalJSON() ([]byte, error) {
	return m.data, m.err
}

type mockYAMLMarshaler struct {
	data []byte
	err  error
}

func (m mockYAMLMarshaler) MarshalYAML() ([]byte, error) {
	return m.data, m.err
}

type mockGenericMarshaler struct {
	data []byte
	err  error
}

func (m mockGenericMarshaler) Marshal() ([]byte, error) {
	return m.data, m.err
}

type mockBinaryMarshaler struct {
	data []byte
	err  error
}

func (m mockBinaryMarshaler) MarshalBinary() ([]byte, error) {
	return m.data, m.err
}

type mockBSONMarshaler struct {
	data []byte
	err  error
}

func (m mockBSONMarshaler) MarshalBSON() ([]byte, error) {
	return m.data, m.err
}

type unmarshalableType struct {
	ch chan int
}

func TestMarshalText(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		result, err := MarshalText(Slice([]string{}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalText() = %q, want empty", result)
		}
	})

	t.Run("SingleString", func(t *testing.T) {
		result, err := MarshalText(One("hello"))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("hello")) {
			t.Errorf("MarshalText() = %q, want %q", result, "hello")
		}
	})

	t.Run("MultipleStrings", func(t *testing.T) {
		result, err := MarshalText(Slice([]string{"foo", "bar", "baz"}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("foobarbaz")) {
			t.Errorf("MarshalText() = %q, want %q", result, "foobarbaz")
		}
	})

	t.Run("EmptyStrings", func(t *testing.T) {
		result, err := MarshalText(Slice([]string{"", "", ""}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("")) {
			t.Errorf("MarshalText() = %q, want empty", result)
		}
	})

	t.Run("SingleByteSlice", func(t *testing.T) {
		result, err := MarshalText(One([]byte("binary data")))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("binary data")) {
			t.Errorf("MarshalText() = %q, want %q", result, "binary data")
		}
	})

	t.Run("MultipleByteSlices", func(t *testing.T) {
		result, err := MarshalText(Slice([][]byte{
			[]byte("chunk1"),
			[]byte("chunk2"),
			[]byte("chunk3"),
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("chunk1chunk2chunk3")) {
			t.Errorf("MarshalText() = %q, want %q", result, "chunk1chunk2chunk3")
		}
	})

	t.Run("TextMarshalerSingle", func(t *testing.T) {
		result, err := MarshalText(One(mockTextMarshaler{
			data: []byte("text marshaled"),
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("text marshaled")) {
			t.Errorf("MarshalText() = %q, want %q", result, "text marshaled")
		}
	})

	t.Run("TextMarshalerMultiple", func(t *testing.T) {
		result, err := MarshalText(Slice([]mockTextMarshaler{
			{data: []byte("first")},
			{data: []byte("second")},
			{data: []byte("third")},
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("firstsecondthird")) {
			t.Errorf("MarshalText() = %q, want %q", result, "firstsecondthird")
		}
	})

	t.Run("TextMarshalerError", func(t *testing.T) {
		_, err := MarshalText(One(mockTextMarshaler{
			err: errors.New("text marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalText() expected error, got nil")
		} else if err.Error() != "text marshal failed" {
			t.Errorf("MarshalText() error = %q, want %q", err.Error(), "text marshal failed")
		}
	})

	t.Run("JSONMarshalerSingle", func(t *testing.T) {
		result, err := MarshalText(One(mockJSONMarshaler{
			data: []byte(`{"key":"value"}`),
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte(`{"key":"value"}`)) {
			t.Errorf("MarshalText() = %q, want %q", result, `{"key":"value"}`)
		}
	})

	t.Run("JSONMarshalerError", func(t *testing.T) {
		_, err := MarshalText(One(mockJSONMarshaler{
			err: errors.New("json marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalText() expected error, got nil")
		} else if err.Error() != "json marshal failed" {
			t.Errorf("MarshalText() error = %q, want %q", err.Error(), "json marshal failed")
		}
	})

	t.Run("YAMLMarshalerSingle", func(t *testing.T) {
		result, err := MarshalText(One(mockYAMLMarshaler{
			data: []byte("key: value\n"),
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("key: value\n")) {
			t.Errorf("MarshalText() = %q, want %q", result, "key: value\n")
		}
	})

	t.Run("YAMLMarshalerError", func(t *testing.T) {
		_, err := MarshalText(One(mockYAMLMarshaler{
			err: errors.New("yaml marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalText() expected error, got nil")
		} else if err.Error() != "yaml marshal failed" {
			t.Errorf("MarshalText() error = %q, want %q", err.Error(), "yaml marshal failed")
		}
	})

	t.Run("GenericMarshalerSingle", func(t *testing.T) {
		result, err := MarshalText(One(mockGenericMarshaler{
			data: []byte("generic data"),
		}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("generic data")) {
			t.Errorf("MarshalText() = %q, want %q", result, "generic data")
		}
	})

	t.Run("GenericMarshalerError", func(t *testing.T) {
		_, err := MarshalText(One(mockGenericMarshaler{
			err: errors.New("generic marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalText() expected error, got nil")
		} else if err.Error() != "generic marshal failed" {
			t.Errorf("MarshalText() error = %q, want %q", err.Error(), "generic marshal failed")
		}
	})

	t.Run("IntegerFallbackToJSON", func(t *testing.T) {
		result, err := MarshalText(Slice([]int{1, 2, 3}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("123")) {
			t.Errorf("MarshalText() = %q, want %q", result, "123")
		}
	})

	t.Run("StructFallbackToJSON", func(t *testing.T) {
		type testStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		result, err := MarshalText(One(testStruct{Name: "test", Value: 42}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte(`{"name":"test","value":42}`)) {
			t.Errorf("MarshalText() = %q, want %q", result, `{"name":"test","value":42}`)
		}
	})

	t.Run("UnicodeStrings", func(t *testing.T) {
		result, err := MarshalText(Slice([]string{"hello", "世界", "🌍"}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("hello世界🌍")) {
			t.Errorf("MarshalText() = %q, want %q", result, "hello世界🌍")
		}
	})

	t.Run("ErrorInMiddleOfSequence", func(t *testing.T) {
		_, err := MarshalText(Slice([]mockTextMarshaler{
			{data: []byte("first")},
			{err: errors.New("middle error")},
			{data: []byte("third")},
		}))
		if err == nil {
			t.Error("MarshalText() expected error, got nil")
		} else if err.Error() != "middle error" {
			t.Errorf("MarshalText() error = %q, want %q", err.Error(), "middle error")
		}
	})

	t.Run("NilByteSlice", func(t *testing.T) {
		var b []byte
		result, err := MarshalText(One(b))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalText() = %q, want empty", result)
		}
	})

	t.Run("BoolFallbackToJSON", func(t *testing.T) {
		result, err := MarshalText(Slice([]bool{true, false}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("truefalse")) {
			t.Errorf("MarshalText() = %q, want %q", result, "truefalse")
		}
	})

	t.Run("FloatFallbackToJSON", func(t *testing.T) {
		result, err := MarshalText(Slice([]float64{1.5, 2.7, 3.14}))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if !bytes.Equal(result, []byte("1.52.73.14")) {
			t.Errorf("MarshalText() = %q, want %q", result, "1.52.73.14")
		}
	})
}

func TestMarshalBinary(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		result, err := MarshalBinary(Slice([][]byte{}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalBinary() = %v, want empty", result)
		}
	})

	t.Run("SingleByteSlice", func(t *testing.T) {
		result, err := MarshalBinary(One([]byte{0x01, 0x02, 0x03}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0x01, 0x02, 0x03}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("MultipleByteSlices", func(t *testing.T) {
		result, err := MarshalBinary(Slice([][]byte{
			{0x01, 0x02},
			{0x03, 0x04},
			{0x05, 0x06},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("BinaryMarshalerSingle", func(t *testing.T) {
		result, err := MarshalBinary(One(mockBinaryMarshaler{
			data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("BinaryMarshalerMultiple", func(t *testing.T) {
		result, err := MarshalBinary(Slice([]mockBinaryMarshaler{
			{data: []byte{0x01}},
			{data: []byte{0x02}},
			{data: []byte{0x03}},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0x01, 0x02, 0x03}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("BinaryMarshalerError", func(t *testing.T) {
		_, err := MarshalBinary(One(mockBinaryMarshaler{
			err: errors.New("binary marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalBinary() expected error, got nil")
		} else if err.Error() != "binary marshal failed" {
			t.Errorf("MarshalBinary() error = %q, want %q", err.Error(), "binary marshal failed")
		}
	})

	t.Run("GenericMarshalerSingle", func(t *testing.T) {
		result, err := MarshalBinary(One(mockGenericMarshaler{
			data: []byte{0xCA, 0xFE},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0xCA, 0xFE}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("GenericMarshalerError", func(t *testing.T) {
		_, err := MarshalBinary(One(mockGenericMarshaler{
			err: errors.New("generic marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalBinary() expected error, got nil")
		} else if err.Error() != "generic marshal failed" {
			t.Errorf("MarshalBinary() error = %q, want %q", err.Error(), "generic marshal failed")
		}
	})

	t.Run("BSONMarshalerSingle", func(t *testing.T) {
		result, err := MarshalBinary(One(mockBSONMarshaler{
			data: []byte{0xB5, 0x0B},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		expected := []byte{0xB5, 0x0B}
		if !bytes.Equal(result, expected) {
			t.Errorf("MarshalBinary() = %v, want %v", result, expected)
		}
	})

	t.Run("BSONMarshalerError", func(t *testing.T) {
		_, err := MarshalBinary(One(mockBSONMarshaler{
			err: errors.New("bson marshal failed"),
		}))
		if err == nil {
			t.Error("MarshalBinary() expected error, got nil")
		} else if err.Error() != "bson marshal failed" {
			t.Errorf("MarshalBinary() error = %q, want %q", err.Error(), "bson marshal failed")
		}
	})

	t.Run("UnsupportedTypeString", func(t *testing.T) {
		_, err := MarshalBinary(One("cannot marshal string"))
		if err == nil {
			t.Error("MarshalBinary() expected error for unsupported type, got nil")
		}
	})

	t.Run("UnsupportedTypeInt", func(t *testing.T) {
		_, err := MarshalBinary(One(42))
		if err == nil {
			t.Error("MarshalBinary() expected error for unsupported type, got nil")
		}
	})

	t.Run("UnsupportedTypeStruct", func(t *testing.T) {
		type testStruct struct {
			Field string
		}
		_, err := MarshalBinary(One(testStruct{Field: "value"}))
		if err == nil {
			t.Error("MarshalBinary() expected error for unsupported type, got nil")
		}
	})

	t.Run("NilByteSlice", func(t *testing.T) {
		var b []byte
		result, err := MarshalBinary(One(b))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalBinary() = %v, want empty", result)
		}
	})

	t.Run("EmptyByteSlice", func(t *testing.T) {
		result, err := MarshalBinary(One([]byte{}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalBinary() = %v, want empty", result)
		}
	})

	t.Run("ErrorInMiddleOfSequence", func(t *testing.T) {
		_, err := MarshalBinary(Slice([]mockBinaryMarshaler{
			{data: []byte{0x01}},
			{err: errors.New("middle error")},
			{data: []byte{0x03}},
		}))
		if err == nil {
			t.Error("MarshalBinary() expected error, got nil")
		} else if err.Error() != "middle error" {
			t.Errorf("MarshalBinary() error = %q, want %q", err.Error(), "middle error")
		}
	})

	t.Run("LargeByteSlice", func(t *testing.T) {
		data := make([]byte, 10000)
		for i := range data {
			data[i] = byte(i % 256)
		}
		result, err := MarshalBinary(One(data))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		if !bytes.Equal(result, data) {
			t.Error("MarshalBinary() large byte slice mismatch")
		}
	})

	t.Run("MultipleEmptyByteSlices", func(t *testing.T) {
		result, err := MarshalBinary(Slice([][]byte{
			{},
			{},
			{},
		}))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}
		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalBinary() = %v, want empty", result)
		}
	})
}

func TestMarshalTextEdgeCases(t *testing.T) {
	t.Run("VeryLongSequence", func(t *testing.T) {
		seq := func(yield func(string) bool) {
			for i := 0; i < 10000; i++ {
				if !yield(fmt.Sprintf("%d", i)) {
					return
				}
			}
		}

		result, err := MarshalText(seq)
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if len(result) == 0 {
			t.Error("MarshalText() returned empty result for long sequence")
		}
	})

	t.Run("ZeroValueStrings", func(t *testing.T) {
		var s string
		result, err := MarshalText(One(s))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if !bytes.Equal(result, []byte{}) {
			t.Errorf("MarshalText() = %q, want empty", result)
		}
	})

	t.Run("ChannelFallbackError", func(t *testing.T) {
		_, err := MarshalText(One(make(chan int)))
		if err == nil {
			t.Error("MarshalText() expected error for channel type, got nil")
		}
	})

	t.Run("FunctionFallbackError", func(t *testing.T) {
		_, err := MarshalText(One(func() {}))
		if err == nil {
			t.Error("MarshalText() expected error for function type, got nil")
		}
	})
}

func TestMarshalBinaryEdgeCases(t *testing.T) {
	t.Run("VeryLongSequence", func(t *testing.T) {
		seq := func(yield func([]byte) bool) {
			for i := 0; i < 10000; i++ {
				if !yield([]byte{byte(i % 256)}) {
					return
				}
			}
		}

		result, err := MarshalBinary(seq)
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}

		if len(result) != 10000 {
			t.Errorf("MarshalBinary() len = %d, want 10000", len(result))
		}
	})

	t.Run("AllTypesRequireError", func(t *testing.T) {
		testCases := []struct {
			name string
			fn   func() error
		}{
			{"String", func() error { _, err := MarshalBinary(One("string")); return err }},
			{"Int", func() error { _, err := MarshalBinary(One(42)); return err }},
			{"Float", func() error { _, err := MarshalBinary(One(3.14)); return err }},
			{"Bool", func() error { _, err := MarshalBinary(One(true)); return err }},
			{"Map", func() error { _, err := MarshalBinary(One(map[string]int{"key": 1})); return err }},
			{"Channel", func() error { _, err := MarshalBinary(One(unmarshalableType{ch: make(chan int)})); return err }},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.fn()
				if err == nil {
					t.Errorf("Expected error for unsupported type %s, got nil", tc.name)
				}
			})
		}
	})
}

func TestMarshalTextPriorityOrder(t *testing.T) {
	t.Run("TextMarshalerBeforeJSON", func(t *testing.T) {
		type bothImpl struct {
			mockTextMarshaler
			mockJSONMarshaler
		}

		obj := bothImpl{
			mockTextMarshaler: mockTextMarshaler{data: []byte("from-text")},
			mockJSONMarshaler: mockJSONMarshaler{data: []byte("from-json")},
		}

		result, err := MarshalText(One(obj))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if !bytes.Equal(result, []byte("from-text")) {
			t.Errorf("Expected TextMarshaler to be used, got %q", result)
		}
	})

	t.Run("JSONMarshalerBeforeYAML", func(t *testing.T) {
		type bothImpl struct {
			mockJSONMarshaler
			mockYAMLMarshaler
		}

		obj := bothImpl{
			mockJSONMarshaler: mockJSONMarshaler{data: []byte("from-json")},
			mockYAMLMarshaler: mockYAMLMarshaler{data: []byte("from-yaml")},
		}

		result, err := MarshalText(One(obj))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if !bytes.Equal(result, []byte("from-json")) {
			t.Errorf("Expected JSONMarshaler to be used, got %q", result)
		}
	})

	t.Run("YAMLMarshalerBeforeGeneric", func(t *testing.T) {
		type bothImpl struct {
			mockYAMLMarshaler
			mockGenericMarshaler
		}

		obj := bothImpl{
			mockYAMLMarshaler:    mockYAMLMarshaler{data: []byte("from-yaml")},
			mockGenericMarshaler: mockGenericMarshaler{data: []byte("from-generic")},
		}

		result, err := MarshalText(One(obj))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if !bytes.Equal(result, []byte("from-yaml")) {
			t.Errorf("Expected YAMLMarshaler to be used, got %q", result)
		}
	})

	t.Run("GenericMarshalerBeforeString", func(t *testing.T) {
		// String type should use string case, not fall through to generic marshaler
		result, err := MarshalText(One("test string"))
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}

		if !bytes.Equal(result, []byte("test string")) {
			t.Errorf("Expected string to be used directly, got %q", result)
		}
	})
}

func TestMarshalBinaryPriorityOrder(t *testing.T) {
	t.Run("BinaryMarshalerBeforeGeneric", func(t *testing.T) {
		type bothImpl struct {
			mockBinaryMarshaler
			mockGenericMarshaler
		}

		obj := bothImpl{
			mockBinaryMarshaler:  mockBinaryMarshaler{data: []byte{0x01}},
			mockGenericMarshaler: mockGenericMarshaler{data: []byte{0x02}},
		}

		result, err := MarshalBinary(One(obj))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}

		if !bytes.Equal(result, []byte{0x01}) {
			t.Errorf("Expected BinaryMarshaler to be used, got %v", result)
		}
	})

	t.Run("GenericMarshalerBeforeBSON", func(t *testing.T) {
		type bothImpl struct {
			mockGenericMarshaler
			mockBSONMarshaler
		}

		obj := bothImpl{
			mockGenericMarshaler: mockGenericMarshaler{data: []byte{0x01}},
			mockBSONMarshaler:    mockBSONMarshaler{data: []byte{0x02}},
		}

		result, err := MarshalBinary(One(obj))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}

		if !bytes.Equal(result, []byte{0x01}) {
			t.Errorf("Expected GenericMarshaler to be used, got %v", result)
		}
	})

	t.Run("ByteSliceBeforeAll", func(t *testing.T) {
		data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		result, err := MarshalBinary(One(data))
		if err != nil {
			t.Fatalf("MarshalBinary() error = %v", err)
		}

		if !bytes.Equal(result, data) {
			t.Errorf("Expected byte slice to be used directly, got %v", result)
		}
	})
}

func TestMarshalTextTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    iter.Seq[any]
		expected []byte
		wantErr  bool
	}{
		{
			name:     "Empty",
			input:    Slice([]any{}),
			expected: []byte{},
			wantErr:  false,
		},
		{
			name:     "MixedMarshalers",
			input:    Slice([]any{"str", mockTextMarshaler{data: []byte("tm")}, []byte("bytes")}),
			expected: []byte("strtmbytes"),
			wantErr:  false,
		},
		{
			name:     "WithError",
			input:    Slice([]any{mockTextMarshaler{err: errors.New("fail")}}),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalText(tt.input)
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if !tt.wantErr && !bytes.Equal(result, tt.expected) {
				t.Errorf("MarshalText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMarshalBinaryTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    iter.Seq[any]
		expected []byte
		wantErr  bool
	}{
		{
			name:     "Empty",
			input:    Slice([]any{}),
			expected: []byte{},
			wantErr:  false,
		},
		{
			name: "MixedMarshalers",
			input: Slice([]any{
				[]byte{0x01},
				mockBinaryMarshaler{data: []byte{0x02}},
				mockGenericMarshaler{data: []byte{0x03}},
			}),
			expected: []byte{0x01, 0x02, 0x03},
			wantErr:  false,
		},
		{
			name:     "WithError",
			input:    Slice([]any{mockBinaryMarshaler{err: errors.New("fail")}}),
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "UnsupportedType",
			input:    Slice([]any{"string"}),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalBinary(tt.input)
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if !tt.wantErr && !bytes.Equal(result, tt.expected) {
				t.Errorf("MarshalBinary() = %v, want %v", result, tt.expected)
			}
		})
	}
}
