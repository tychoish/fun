package dt

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

func TestOptional(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		var opt Optional[string]
		check.True(t, ft.Not(opt.Ok()))
		check.Equal(t, opt.Resolve(), "")
		out, ok := opt.Get()
		check.True(t, ft.Not(ok))
		check.Equal(t, out, "")
	})
	t.Run("Functions", func(t *testing.T) {
		opt := NewOptional(400)
		future := opt.Future()
		handler := opt.Handler()
		check.Equal(t, 400, future.Resolve())
		handler.Handle(100)
		check.Equal(t, 100, future.Resolve())
		handler.Handle(300)
		check.Equal(t, 300, future.Resolve())
	})
	t.Run("SetAndGet", func(t *testing.T) {
		opt := NewOptional(400)
		check.Equal(t, 400, opt.Resolve())
		opt.Set(100)
		check.Equal(t, 100, opt.Resolve())
		opt.Set(300)
		check.Equal(t, 300, opt.Resolve())
	})
	t.Run("Constructor", func(t *testing.T) {
		opt := NewOptional("hello")
		check.True(t, opt.Ok())
		check.Equal(t, opt.Resolve(), "hello")
		out, ok := opt.Get()
		check.True(t, ok)
		check.Equal(t, out, "hello")
	})
	t.Run("Default", func(t *testing.T) {
		opt := NewOptional("hello")
		opt.Default("world")
		check.Equal(t, opt.Resolve(), "hello")

		opt.Reset()
		assert.True(t, ft.Not(opt.Ok()))

		opt.Default("world")
		check.Equal(t, opt.Resolve(), "world")
		opt.Default("hello")
		check.Equal(t, opt.Resolve(), "world")
	})
	t.Run("Default", func(t *testing.T) {
		opt := NewOptional("hello")
		count := 0
		helloFuture := func() string { count++; return "hello" }
		worldFuture := func() string { count++; return "world" }

		opt.DefaultFuture(worldFuture)
		check.Equal(t, opt.Resolve(), "hello")
		check.Equal(t, count, 0)

		opt.Reset()
		assert.True(t, ft.Not(opt.Ok()))

		opt.DefaultFuture(helloFuture)
		check.Equal(t, opt.Resolve(), "hello")
		opt.DefaultFuture(worldFuture)
		check.Equal(t, opt.Resolve(), "hello")
	})
	t.Run("Swap", func(t *testing.T) {
		opt := NewOptional(400)
		check.Equal(t, 400, opt.Swap(200))
		check.Equal(t, 200, opt.Swap(100))
		check.Equal(t, 100, opt.Swap(50))
		check.Equal(t, 50, opt.Swap(25))
	})
	t.Run("SQL", func(t *testing.T) {
		t.Run("Scan", func(t *testing.T) {
			t.Run("Nil", func(t *testing.T) {
				opt := &Optional[int]{}
				check.NotError(t, opt.Scan(nil))
			})
			t.Run("String", func(t *testing.T) {
				opt := &Optional[int]{}
				check.NotError(t, opt.Scan("100"))
				check.True(t, opt.Ok())
				check.Equal(t, 100, opt.Resolve())
			})
			t.Run("Bytes", func(t *testing.T) {
				opt := &Optional[[]byte]{}
				check.NotError(t, opt.Scan([]byte("100")))
				check.True(t, opt.Ok())
				check.Equal(t, "100", string(opt.Resolve()))
			})
			t.Run("TextMarshaler", func(t *testing.T) {
				opt := &Optional[*mockTextCodec]{}
				assert.NotError(t, opt.Scan(t.Name()))
				check.Equal(t, string(opt.v.input), t.Name())
				check.Equal(t, string(opt.v.output), "")
				check.Equal(t, opt.v.count, 1)
				check.True(t, opt.Ok())
			})
			t.Run("TextBinary", func(t *testing.T) {
				opt := &Optional[*mockBinaryCodec]{}
				assert.NotError(t, opt.Scan([]byte(t.Name())))
				check.Equal(t, string(opt.v.input), t.Name())
				check.Equal(t, string(opt.v.output), "")
				check.Equal(t, opt.v.count, 1)
				check.True(t, opt.Ok())
			})
			t.Run("Scanner", func(t *testing.T) {
				opt := &Optional[*mockSQLcell]{}
				now := time.Now()
				assert.NotError(t, opt.Scan(now))
				assert.Equal(t, now, opt.v.src.(time.Time))
				check.Equal(t, opt.v.count, 1)
				check.True(t, opt.Ok())
			})
			t.Run("UnknownTypeMismatch", func(t *testing.T) {
				opt := &Optional[uint]{}
				err := opt.Scan(-1)
				assert.Error(t, err)
				assert.ErrorIs(t, err, ers.ErrInvalidRuntimeType)
				assert.ErrorIs(t, err, ers.ErrInvalidInput)
			})
			t.Run("IntegerUpconvert", func(t *testing.T) {
				opt := &Optional[int64]{}
				err := opt.Scan(int(100))
				assert.NotError(t, err)
				check.True(t, opt.Ok())
				assert.Equal(t, int64(100), opt.v)
			})
		})
		t.Run("Value", func(t *testing.T) {
			t.Run("Unset", func(t *testing.T) {
				opt := &Optional[int]{}
				out, err := opt.Value()
				check.NotError(t, err)
				check.Nil(t, out)
			})
			t.Run("WhenSet", func(t *testing.T) {
				opt := NewOptional(42)
				out, err := opt.Value()
				assert.NotError(t, err)
				check.NotNil(t, out)
				check.Equal(t, out.(int64), 42)
			})
			t.Run("TrivialType", func(t *testing.T) {
				opt := NewOptional[int64](100)
				out, err := opt.Value()
				assert.NotError(t, err)
				check.Equal(t, out.(int64), 100)
			})
			t.Run("IntegersSpecialCase", func(t *testing.T) {
				opt := NewOptional[int64](100)
				out, err := opt.Value()
				assert.NotError(t, err)
				check.Equal(t, out.(int64), 100)
			})
			t.Run("Valuer", func(t *testing.T) {
				now := time.Now()
				opt := NewOptional(&mockSQLcell{src: now})
				val, err := opt.Value()
				assert.NotError(t, err)
				check.Equal(t, val.(time.Time), now)
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("Text", func(t *testing.T) {
				opt := NewOptional(&mockTextCodec{output: []byte("hello earthling!")})
				val, err := opt.Value()
				assert.NotError(t, err)
				check.Equal(t, string(opt.v.output), string(val.([]byte)))
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("Binary", func(t *testing.T) {
				opt := NewOptional(&mockBinaryCodec{output: []byte("hello earthling!")})
				val, err := opt.Value()
				assert.NotError(t, err)
				check.Equal(t, string(opt.v.output), string(val.([]byte)))
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("ImpossibleType", func(t *testing.T) {
				opt := NewOptional[*testing.T](t)
				out, err := opt.Value()
				check.Error(t, err)
				check.Equal(t, out, nil)
			})
		})
	})
	t.Run("Text", func(t *testing.T) {
		t.Run("Marshal", func(t *testing.T) {
			t.Run("JSON", func(t *testing.T) {
				opt := &Optional[*mockJSONcodec]{}
				opt.Set(&mockJSONcodec{output: []byte("helloJSON")})
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "helloJSON")
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("YAML", func(t *testing.T) {
				opt := &Optional[*mockYamlCodec]{}
				opt.Set(&mockYamlCodec{output: []byte("helloYAML")})
				val, err := opt.MarshalText()
				assert.Equal(t, string(val), "helloYAML")
				assert.NotError(t, err)
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("Generic", func(t *testing.T) {
				opt := &Optional[*mockGenericCodec]{}
				opt.Set(&mockGenericCodec{output: []byte("hello whatever")})
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello whatever")
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("StdEncoding", func(t *testing.T) {
				opt := &Optional[*mockTextCodec]{}
				opt.Set(&mockTextCodec{output: []byte("hello text")})
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello text")
			})
			t.Run("string", func(t *testing.T) {
				opt := &Optional[string]{}
				opt.Set("helloStr")
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "helloStr")
			})
			t.Run("[]byte", func(t *testing.T) {
				opt := &Optional[[]byte]{}
				opt.Set([]byte("hello george"))
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello george")
			})
			t.Run("Fallback", func(t *testing.T) {
				opt := &Optional[*int]{}
				val, err := opt.MarshalText()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "null")
			})
			t.Run("FallbackError", func(t *testing.T) {
				opt := &Optional[fn.Future[string]]{}
				opt.Set(t.Name)
				_, err := opt.MarshalText()
				assert.Error(t, err)
			})
		})
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("JSON", func(t *testing.T) {
				opt := &Optional[*mockJSONcodec]{}
				check.NotError(t, opt.UnmarshalText([]byte("hello jason")))
				check.Equal(t, opt.v.count, 1)
				check.Equal(t, string(opt.v.input), "hello jason")
				check.True(t, opt.Ok())
			})
			t.Run("YAML", func(t *testing.T) {
				opt := &Optional[*mockYamlCodec]{}
				check.NotError(t, opt.UnmarshalText([]byte("hello yaml")))
				check.Equal(t, opt.v.count, 1)
				check.True(t, opt.Ok())
				check.Equal(t, string(opt.v.input), "hello yaml")
			})
			t.Run("YAML", func(t *testing.T) {
				opt := &Optional[*mockGenericCodec]{}
				check.NotError(t, opt.UnmarshalText([]byte("hello yaml")))
				check.Equal(t, opt.v.count, 1)
				check.True(t, opt.Ok())
				check.Equal(t, string(opt.v.input), "hello yaml")
			})
			t.Run("string", func(t *testing.T) {
				opt := &Optional[string]{}
				check.NotError(t, opt.UnmarshalText([]byte("hello str")))
				check.Equal(t, opt.v, "hello str")
			})
			t.Run("[]byte", func(t *testing.T) {
				opt := &Optional[[]byte]{}
				check.NotError(t, opt.UnmarshalText([]byte("hello slice")))
				check.Equal(t, string(opt.v), "hello slice")
			})
			t.Run("FallbackError", func(t *testing.T) {
				opt := &Optional[fn.Future[string]]{}
				check.Error(t, opt.UnmarshalText(nil))
				check.True(t, !opt.Ok())
			})
		})
	})
	t.Run("Binary", func(t *testing.T) {
		t.Run("Marshal", func(t *testing.T) {
			t.Run("BSON", func(t *testing.T) {
				opt := &Optional[*mockBsonCodec]{}
				opt.Set(&mockBsonCodec{output: []byte("helloBSON")})
				val, err := opt.MarshalBinary()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "helloBSON")
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("Generic", func(t *testing.T) {
				opt := &Optional[*mockGenericCodec]{}
				opt.Set(&mockGenericCodec{output: []byte("hello whatever")})
				val, err := opt.MarshalBinary()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello whatever")
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("StdEncoding", func(t *testing.T) {
				opt := &Optional[*mockBinaryCodec]{}
				opt.Set(&mockBinaryCodec{output: []byte("hello whatever")})
				val, err := opt.MarshalBinary()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello whatever")
				check.Equal(t, opt.v.count, 1)
			})
			t.Run("Byte", func(t *testing.T) {
				opt := &Optional[[]byte]{}
				opt.Set([]byte("hello ungeorge"))
				val, err := opt.MarshalBinary()
				assert.NotError(t, err)
				assert.Equal(t, string(val), "hello ungeorge")
			})
			t.Run("Unknown", func(t *testing.T) {
				opt := &Optional[*testing.T]{}
				opt.Set(t)
				val, err := opt.MarshalBinary()
				check.Nil(t, val)
				assert.Error(t, err)
			})
		})
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("BSON", func(t *testing.T) {
				opt := &Optional[*mockBsonCodec]{}
				check.NotError(t, opt.UnmarshalBinary([]byte("hello bson")))
				check.Equal(t, opt.v.count, 1)
				check.Equal(t, string(opt.v.input), "hello bson")
				check.True(t, opt.Ok())
			})
			t.Run("Generic", func(t *testing.T) {
				opt := &Optional[*mockGenericCodec]{}
				check.NotError(t, opt.UnmarshalBinary([]byte("hello whoever")))
				check.Equal(t, opt.v.count, 1)
				check.Equal(t, string(opt.v.input), "hello whoever")
				check.True(t, opt.Ok())
			})
			t.Run("StdEncoding", func(t *testing.T) {
				opt := &Optional[*mockBinaryCodec]{}
				check.NotError(t, opt.UnmarshalBinary([]byte("hello runtime")))
				check.Equal(t, opt.v.count, 1)
				check.Equal(t, string(opt.v.input), "hello runtime")
				check.True(t, opt.Ok())
			})
			t.Run("Byte", func(t *testing.T) {
				opt := &Optional[[]byte]{}
				check.NotError(t, opt.UnmarshalBinary([]byte("hello slice")))
				check.Equal(t, string(opt.v), "hello slice")
			})
			t.Run("Unknown", func(t *testing.T) {
				opt := &Optional[fn.Future[string]]{}
				check.Error(t, opt.UnmarshalBinary([]byte("hello")))
				check.True(t, !opt.Ok())
			})
		})
	})

}

type mockSQLcell struct {
	src   any
	err   error
	count int
}

func (s *mockSQLcell) Scan(src any) error           { s.count++; s.src = src; return s.err }
func (s *mockSQLcell) Value() (driver.Value, error) { s.count++; return s.src, s.err }

type mockBinaryCodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockBinaryCodec) MarshalBinary() ([]byte, error) { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockBinaryCodec) UnmarshalBinary(in []byte) error {
	mtc.count++
	mtc.input = in
	return mtc.err
}

type mockTextCodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockTextCodec) MarshalText() ([]byte, error) { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockTextCodec) UnmarshalText(in []byte) error {
	mtc.count++
	mtc.input = in
	return mtc.err
}

type mockGenericCodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockGenericCodec) Marshal() ([]byte, error)  { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockGenericCodec) Unmarshal(in []byte) error { mtc.count++; mtc.input = in; return mtc.err }

type mockJSONcodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockJSONcodec) MarshalJSON() ([]byte, error)  { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockJSONcodec) UnmarshalJSON(in []byte) error { mtc.count++; mtc.input = in; return mtc.err }

type mockYamlCodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockYamlCodec) MarshalYAML() ([]byte, error)  { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockYamlCodec) UnmarshalYAML(in []byte) error { mtc.count++; mtc.input = in; return mtc.err }

type mockBsonCodec struct {
	input  []byte
	output []byte
	err    error
	count  int
}

func (mtc *mockBsonCodec) MarshalBSON() ([]byte, error)  { mtc.count++; return mtc.output, mtc.err }
func (mtc *mockBsonCodec) UnmarshalBSON(in []byte) error { mtc.count++; mtc.input = in; return mtc.err }
