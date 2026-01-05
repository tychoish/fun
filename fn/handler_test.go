package fn

import (
	"io"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
)

func TestHandler(t *testing.T) {
	t.Run("Check", func(t *testing.T) {
		var ob Handler[int] = func(_ int) {
			panic(io.EOF)
		}
		assert.ErrorIs(t, ob.RecoverPanic(100), io.EOF)
	})
	t.Run("Smoke", func(t *testing.T) {
		counter := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 42)
			counter++
		}
		ob(42)
		check.Equal(t, counter, 1)
		ob.Job(42)(t.Context())
		check.Equal(t, counter, 2)
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
		ob.WithRecover(oe).Read(100)
		assert.Equal(t, 1, count)
	})
	t.Run("If", func(t *testing.T) {
		count := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 100)
			count++
		}

		ob.If(false).Read(100)
		assert.Equal(t, 0, count)
		ob.If(true).Read(100)
		assert.Equal(t, 1, count)
	})
	t.Run("When", func(t *testing.T) {
		should := false
		count := 0
		var ob Handler[int] = func(in int) {
			check.Equal(t, in, 100)
			count++
		}
		wob := ob.When(func() bool { return should })
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

			cob := ob.WithNext(func(in int) { check.Equal(t, count, 1); count++; check.Equal(t, in, 100) })
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

			cob := ob.Join(ob, ob, ob)
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
	t.Run("Locker", func(t *testing.T) {
		// this is mostly just tempting the race detecor
		wg := &sync.WaitGroup{}
		count := 0
		var ob Handler[int] = func(in int) {
			defer wg.Done()
			count++
			check.Equal(t, in, 100)
		}

		mtx := &sync.Mutex{}
		lob := ob.WithLocker(mtx)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go lob(100)
		}
		wg.Wait()

		assert.Equal(t, count, 10)
	})
	t.Run("Skip", func(t *testing.T) {
		count := 0
		of := NewHandler(func(i int) { count++; check.Equal(t, i, 42) })
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
		of := NewHandler(func(i int) { count++; check.Equal(t, i, 42) }).
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
	t.Run("Panics", func(t *testing.T) {
		err := NewHandler(func(in int) { assert.Equal(t, in, 33); panic("foo") }).RecoverPanic(33)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
	})
	t.Run("JoinHandlers", func(t *testing.T) {
		count := 0
		sum := 0
		reset := func() { count, sum = 0, 0 }

		handler := NewHandler(func(i int) { count++; sum += i })
		t.Run("One", func(t *testing.T) {
			defer reset()
			metaHandler := JoinHandlers(irt.Args(handler, handler, handler, handler, handler))

			metaHandler(1)
			assert.Equal(t, sum, 5)
			assert.Equal(t, count, 5)
		})

		t.Run("Five", func(t *testing.T) {
			defer reset()
			metaHandler := JoinHandlers(irt.Args(handler, handler, handler, handler, handler))

			metaHandler(5)
			assert.Equal(t, sum, 25)
			assert.Equal(t, count, 5)
		})
		t.Run("NilHandling", func(t *testing.T) {
			defer reset()
			metaHandler := JoinHandlers(irt.Args(handler, nil, handler, nil, handler, handler))

			metaHandler(4)
			assert.Equal(t, sum, 16)
			assert.Equal(t, count, 4)
		})
	})
	t.Run("ErrorHandler", func(t *testing.T) {
		var called bool
		eh := func(err error) { called = true; check.Error(t, err); check.ErrorIs(t, err, io.EOF) }
		out := ErrorHandler[int](eh)(42, io.EOF)
		assert.Equal(t, 42, out)
		assert.True(t, called)
	})
	t.Run("NoopHandler", func(t *testing.T) {
		assert.NotPanic(t, func() {
			NewNoopHandler[func()]().Read(nil)
		})
	})
	t.Run("Safe", func(t *testing.T) {
		t.Run("FuncPtrs", func(t *testing.T) {
			assert.Panic(t, func() {
				var hf Handler[func()]
				hf.Read(nil)
			})
		})
		t.Run("Nums", func(t *testing.T) {
			assert.Panic(t, func() {
				var hf Handler[int]
				hf.Read(-1)
			})
		})
	})
}
