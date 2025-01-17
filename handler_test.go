package fun

import (
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestHandler(t *testing.T) {
	t.Run("Check", func(t *testing.T) {
		var ob Handler[int] = func(_ int) {
			panic(io.EOF)

		}
		assert.ErrorIs(t, ob.RecoverPanic(100), io.EOF)
	})
	t.Run("Safe", func(t *testing.T) {
		count := 0
		oe := func(err error) {
			assert.ErrorIs(t, err, io.EOF)
			if err != nil {
				count++
			}
		}

		var ob Handler[int] = func(_ int) {
			panic(io.EOF)

		}
		ob.WithRecover(oe).Handle(100)
		assert.Equal(t, 1, count)
	})
	t.Run("HandlePassthrough", func(t *testing.T) {
		called := 0
		pth := HandlePassthrough[int](func(err error) { check.Equal(t, called, 1); called++; check.Error(t, err) })
		out := pth(func() (int, error) { called++; return 42, errors.New("exists") }())
		check.Equal(t, called, 2)
		check.Equal(t, out, 42)
	})
	t.Run("If", func(t *testing.T) {
		count := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 100)
			count++
		}

		ob.If(false).Handle(100)
		assert.Equal(t, 0, count)
		ob.If(true).Handle(100)
		assert.Equal(t, 1, count)
	})
	t.Run("When", func(t *testing.T) {
		should := false
		count := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 100)
			count++
		}
		wob := ob.Skip(func(int) bool { return should })
		wob(100)
		assert.Equal(t, 0, count)
		should = true
		wob(100)
		assert.Equal(t, 1, count)
		should = false
		wob(100)
		assert.Equal(t, 1, count)
		should = true
		wob(100)
		assert.Equal(t, 2, count)
	})
	t.Run("Once", func(t *testing.T) {
		count := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 100)
			count++
		}
		oob := ob.Once()
		for i := 0; i < 100; i++ {
			oob(100)
		}
		assert.Equal(t, 1, count)
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Join", func(t *testing.T) {
			count := 0
			var ob Handler[int] = func(in int) {
				check.Equal(t, in, 100)
				check.Equal(t, count, 0)
				count++
			}

			cob := ob.Join(func(in int) { check.Equal(t, count, 1); count++; check.Equal(t, in, 100) })
			cob(100)

			assert.Equal(t, 2, count)
		})
		t.Run("PreHook", func(t *testing.T) {
			count := 0
			var ob Handler[int] = func(in int) {
				check.Equal(t, in, 100)
				check.Equal(t, count, 1)
				count++
			}

			cob := ob.PreHook(func(in int) { check.Equal(t, count, 0); count++; check.Equal(t, in, 100) })
			cob(100)

			assert.Equal(t, 2, count)
		})
		t.Run("Many", func(t *testing.T) {
			count := 0
			var ob Handler[int] = func(in int) {
				check.Equal(t, in, 100)
				count++
			}

			cob := ob.Chain(ob, ob, ob)
			cob(100)

			assert.Equal(t, 4, count)
		})
		t.Run("All", func(t *testing.T) {
			count := 0
			var ob Handler[int] = func(in int) {
				check.Equal(t, in, 100)
				count++
			}

			ob.All(100, 100, 100, 100)
			assert.Equal(t, 4, count)
		})
	})
	t.Run("Lock", func(t *testing.T) {
		// this is mostly just tempting the race detecor
		wg := &sync.WaitGroup{}
		count := 0
		var ob Handler[int] = func(in int) {
			defer wg.Done()
			count++
			check.Equal(t, in, 100)
		}

		lob := ob.Lock()

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go lob(100)
		}
		wg.Wait()

		assert.Equal(t, count, 10)
	})
	t.Run("Skip", func(t *testing.T) {
		count := 0
		of := Handle(func(i int) { count++; check.Equal(t, i, 42) })
		off := of.Skip(func(i int) bool { return i == 42 })

		off(42)
		off(42)

		check.Equal(t, count, 2)

		// if the filter didn't work, this would fail the
		// assertion in the observer above, so this is
		// actually test:
		off(420)
		off(4200)

		check.Equal(t, count, 2)
	})
	t.Run("Filter", func(t *testing.T) {
		count := 0
		of := Handle(func(i int) { count++; check.Equal(t, i, 42) }).
			Skip(func(in int) bool { return in != 0 }).
			Filter(func(in int) int {
				switch in {
				case 42:
					return 0
				case 300:
					return 42
				default:
					return 0
				}
			})

		of(42)
		of(42)
		check.Equal(t, count, 0)
		of(300)
		of(300)
		check.Equal(t, count, 2)
	})
	t.Run("Error", func(t *testing.T) {
		called := 0
		oef := HF.ErrorHandler(func(_ error) { called++ })
		oef(nil)
		check.Equal(t, called, 0)
		oef(io.EOF)
		check.Equal(t, called, 1)
	})
}
