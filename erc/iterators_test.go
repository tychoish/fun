package erc

import (
	"context"
	"errors"
	"io"
	"iter"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestHandle(t *testing.T) {
	t.Parallel()

	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0, 4)
		for val := range Handle(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 0, len(result))
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(3, nil)
		}
		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0, 3)
		for val := range Handle(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 3, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 3, result[2])
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("SomeErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, err1) { // value with error is skipped
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(0, err2) { // value with error is skipped
				return
			}
			yield(3, nil)
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 4)
		for val := range Handle(seq, handler) {
			result = append(result, val)
		}

		// Only successful values are yielded
		assert.Equal(t, 3, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 3, result[2])

		// Both errors are handled
		assert.Equal(t, 2, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], err1)
		assert.ErrorIs(t, handlerErrors[1], err2)
	})

	t.Run("AllErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		err3 := errors.New("error3")
		seq := func(yield func(int, error) bool) {
			if !yield(0, err1) {
				return
			}
			if !yield(0, err2) {
				return
			}
			yield(0, err3)
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 3)
		for val := range Handle(seq, handler) {
			result = append(result, val)
		}

		// No successful values
		assert.Equal(t, 0, len(result))

		// All errors are handled
		assert.Equal(t, 3, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], err1)
		assert.ErrorIs(t, handlerErrors[1], err2)
		assert.ErrorIs(t, handlerErrors[2], err3)
	})

	t.Run("EarlyStopOnYieldFalse", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(3, nil) {
				return
			}
			yield(4, nil)
		}

		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0)
		for val := range Handle(seq, handler) {
			result = append(result, val)
			if val == 2 {
				break // stop early
			}
		}

		assert.Equal(t, 2, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("WithCollector", func(t *testing.T) {
		err1 := io.EOF
		err2 := context.Canceled
		seq := func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			if !yield("", err1) {
				return
			}
			if !yield("second", nil) {
				return
			}
			if !yield("", err2) {
				return
			}
			yield("third", nil)
		}

		ec := &Collector{}
		handler := func(err error) {
			ec.Push(err)
		}

		result := make([]string, 0, 3)
		for val := range Handle(seq, handler) {
			result = append(result, val)
		}

		// Only successful values
		assert.Equal(t, 3, len(result))
		assert.Equal(t, "first", result[0])
		assert.Equal(t, "second", result[1])
		assert.Equal(t, "third", result[2])

		// Both errors collected
		assert.Equal(t, 2, ec.Len())
		assert.ErrorIs(t, ec.Resolve(), err1)
		assert.ErrorIs(t, ec.Resolve(), err2)
	})

	t.Run("HandlerPanicsDoNotStopIteration", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, errors.New("error")) {
				return
			}
			yield(2, nil)
		}

		handler := func(err error) {
			// Handler that panics
			panic("handler panic")
		}

		// This should panic when handler is called
		defer func() {
			r := recover()
			assert.NotZero(t, r)
			assert.Equal(t, "handler panic", r)
		}()

		for range Handle(seq, handler) {
			// iteration happens
		}
	})
}

func TestHandleUntil(t *testing.T) {
	t.Parallel()

	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0, 0)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 0, len(result))
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(3, nil)
		}
		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0, 3)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 3, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 3, result[2])
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("ErrorInMiddle", func(t *testing.T) {
		err1 := errors.New("first-error")
		err2 := errors.New("second-error")
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(0, err1) { // stops here
				return
			}
			if !yield(3, nil) { // never reached
				return
			}
			yield(0, err2) // never reached
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 2)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		// Only values before first error
		assert.Equal(t, 2, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])

		// Only first error is handled
		assert.Equal(t, 1, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], err1)
		check.True(t, !errors.Is(handlerErrors[0], err2))
	})

	t.Run("ErrorAtStart", func(t *testing.T) {
		expectedErr := io.EOF
		seq := func(yield func(string, error) bool) {
			if !yield("", expectedErr) { // immediate error
				return
			}
			yield("never", nil) // never reached
		}

		handlerErrors := make([]error, 0, 1)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]string, 0, 0)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 0, len(result))
		assert.Equal(t, 1, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], expectedErr)
	})

	t.Run("ErrorAtEnd", func(t *testing.T) {
		expectedErr := context.DeadlineExceeded
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(3, nil) {
				return
			}
			yield(0, expectedErr)
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 3)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 3, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 3, result[2])
		assert.Equal(t, 1, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], expectedErr)
	})

	t.Run("EarlyStopOnYieldFalse", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(3, nil) {
				return
			}
			yield(4, nil)
		}

		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
			if val == 2 {
				break // stop early
			}
		}

		assert.Equal(t, 2, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("DifferenceFromHandle", func(t *testing.T) {
		// Demonstrate that HandleUntil stops, while Handle continues
		err1 := errors.New("stop")
		err2 := errors.New("continue")

		// HandleUntil stops at first error
		seqUntil := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, err1) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(0, err2)
		}
		handlerUntilErrors := make([]error, 0, 1)
		handlerUntil := func(err error) {
			handlerUntilErrors = append(handlerUntilErrors, err)
		}
		resultUntil := make([]int, 0, 1)
		for val := range HandleUntil(seqUntil, handlerUntil) {
			resultUntil = append(resultUntil, val)
		}
		assert.Equal(t, 1, len(resultUntil))
		assert.Equal(t, 1, len(handlerUntilErrors))

		// Handle continues through all errors
		seqHandle := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, err1) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(0, err2)
		}
		handlerErrors := make([]error, 0, 2)
		handlerHandle := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}
		resultHandle := make([]int, 0, 2)
		for val := range Handle(seqHandle, handlerHandle) {
			resultHandle = append(resultHandle, val)
		}
		assert.Equal(t, 2, len(resultHandle))
		assert.Equal(t, 2, len(handlerErrors))
	})

	t.Run("WithLogger", func(t *testing.T) {
		expectedErr := errors.New("logged error")
		seq := func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			if !yield("second", nil) {
				return
			}
			if !yield("", expectedErr) {
				return
			}
			yield("never", nil)
		}

		logged := make([]string, 0)
		handler := func(err error) {
			logged = append(logged, "ERROR: "+err.Error())
		}

		result := make([]string, 0, 2)
		for val := range HandleUntil(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 2, len(result))
		assert.Equal(t, 1, len(logged))
		assert.Equal(t, "ERROR: logged error", logged[0])
	})
}

func TestHandleAll(t *testing.T) {
	t.Parallel()

	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		handlerCalled := 0
		handler := func(err error) { handlerCalled++ }

		result := make([]int, 0, 0)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 0, len(result))
		assert.Equal(t, 0, handlerCalled)
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(3, nil)
		}

		handlerCalls := 0
		nilCount := 0
		handler := func(err error) {
			handlerCalls++
			if err == nil {
				nilCount++
			}
		}

		result := make([]int, 0, 3)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 3, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 3, result[2])
		// Handler is called for all iterations, even with nil errors
		assert.Equal(t, 3, handlerCalls)
		assert.Equal(t, 3, nilCount)
	})

	t.Run("SomeErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(10, err1) { // value included even with error
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(20, err2) { // value included even with error
				return
			}
			yield(3, nil)
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 5)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		// ALL values are yielded, including those with errors
		assert.Equal(t, 5, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 10, result[1])
		assert.Equal(t, 2, result[2])
		assert.Equal(t, 20, result[3])
		assert.Equal(t, 3, result[4])

		// Handler called for all iterations
		assert.Equal(t, 5, len(handlerErrors))
		assert.Equal(t, nil, handlerErrors[0])
		assert.ErrorIs(t, handlerErrors[1], err1)
		assert.Equal(t, nil, handlerErrors[2])
		assert.ErrorIs(t, handlerErrors[3], err2)
		assert.Equal(t, nil, handlerErrors[4])
	})

	t.Run("AllErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		err3 := errors.New("error3")
		seq := func(yield func(int, error) bool) {
			if !yield(100, err1) {
				return
			}
			if !yield(200, err2) {
				return
			}
			yield(300, err3)
		}

		handlerErrors := make([]error, 0)
		handler := func(err error) {
			handlerErrors = append(handlerErrors, err)
		}

		result := make([]int, 0, 3)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		// ALL values yielded, even with errors
		assert.Equal(t, 3, len(result))
		assert.Equal(t, 100, result[0])
		assert.Equal(t, 200, result[1])
		assert.Equal(t, 300, result[2])

		// All errors handled
		assert.Equal(t, 3, len(handlerErrors))
		assert.ErrorIs(t, handlerErrors[0], err1)
		assert.ErrorIs(t, handlerErrors[1], err2)
		assert.ErrorIs(t, handlerErrors[2], err3)
	})

	t.Run("EarlyStopOnYieldFalse", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(3, nil) {
				return
			}
			yield(4, nil)
		}

		handlerCalls := 0
		handler := func(err error) { handlerCalls++ }

		result := make([]int, 0)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
			if val == 2 {
				break // stop early
			}
		}

		assert.Equal(t, 2, len(result))
		assert.Equal(t, 1, result[0])
		assert.Equal(t, 2, result[1])
		assert.Equal(t, 2, handlerCalls)
	})

	t.Run("WithCollector", func(t *testing.T) {
		err1 := io.EOF
		err2 := context.Canceled
		seq := func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			if !yield("with-error-1", err1) {
				return
			}
			if !yield("second", nil) {
				return
			}
			if !yield("with-error-2", err2) {
				return
			}
			yield("third", nil)
		}

		ec := &Collector{}
		handler := func(err error) {
			ec.Push(err)
		}

		result := make([]string, 0, 5)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		// All values included
		assert.Equal(t, 5, len(result))
		assert.Equal(t, "first", result[0])
		assert.Equal(t, "with-error-1", result[1])
		assert.Equal(t, "second", result[2])
		assert.Equal(t, "with-error-2", result[3])
		assert.Equal(t, "third", result[4])

		// Only non-nil errors collected
		assert.Equal(t, 2, ec.Len())
		assert.ErrorIs(t, ec.Resolve(), err1)
		assert.ErrorIs(t, ec.Resolve(), err2)
	})

	t.Run("DifferenceFromOtherHandlers", func(t *testing.T) {
		err1 := errors.New("error")

		// HandleAll includes all values
		seqAll := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(999, err1) {
				return
			}
			yield(2, nil)
		}
		handlerAllCalls := 0
		handlerAll := func(err error) { handlerAllCalls++ }
		resultAll := make([]int, 0, 3)
		for val := range HandleAll(seqAll, handlerAll) {
			resultAll = append(resultAll, val)
		}
		assert.Equal(t, 3, len(resultAll))
		assert.Equal(t, 999, resultAll[1])
		assert.Equal(t, 3, handlerAllCalls) // called for all, including nil

		// Handle skips error values
		seqHandle := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(999, err1) {
				return
			}
			yield(2, nil)
		}
		handlerCalls := 0
		handlerHandle := func(err error) { handlerCalls++ }
		resultHandle := make([]int, 2, 2)
		for val := range Handle(seqHandle, handlerHandle) {
			resultHandle = append(resultHandle, val)
		}
		assert.Equal(t, 2, len(resultHandle))
		assert.Equal(t, 1, handlerCalls) // called only for errors

		// HandleUntil stops at first error
		seqUntil := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(999, err1) {
				return
			}
			yield(2, nil)
		}
		handlerUntilCalls := 0
		handlerUntil := func(err error) { handlerUntilCalls++ }
		resultUntil := make([]int, 0, 1)
		for val := range HandleUntil(seqUntil, handlerUntil) {
			resultUntil = append(resultUntil, val)
		}
		assert.Equal(t, 1, len(resultUntil))
		assert.Equal(t, 1, handlerUntilCalls)
	})

	t.Run("HandlerSeesAllErrors", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, errors.New("err1")) {
				return
			}
			if !yield(3, nil) {
				return
			}
			yield(4, errors.New("err2"))
		}

		errorLog := make(map[int]error)
		handler := func(err error) {
			if err != nil {
				errorLog[len(errorLog)] = err
			}
		}

		result := make([]int, 0, 4)
		for val := range HandleAll(seq, handler) {
			result = append(result, val)
		}

		assert.Equal(t, 4, len(result))
		assert.Equal(t, 2, len(errorLog))
	})
}

func TestHandlerComparison(t *testing.T) {
	t.Parallel()

	// This test demonstrates the key differences between all three handlers
	t.Run("ComprehensiveComparison", func(t *testing.T) {
		err1 := errors.New("first")
		err2 := errors.New("second")

		// Shared sequence
		makeSeq := func() iter.Seq2[int, error] {
			return func(yield func(int, error) bool) {
				if !yield(1, nil) {
					return
				}
				if !yield(100, err1) {
					return
				}
				if !yield(2, nil) {
					return
				}
				if !yield(200, err2) {
					return
				}
				yield(3, nil)
			}
		}

		// Test Handle
		handlerErrors := make([]error, 0)
		handlerHandle := func(err error) {
			if err != nil {
				handlerErrors = append(handlerErrors, err)
			}
		}
		resultHandle := make([]int, 0, 3)
		for val := range Handle(makeSeq(), handlerHandle) {
			resultHandle = append(resultHandle, val)
		}
		assert.Equal(t, 3, len(resultHandle))             // skips error values
		check.EqualItems(t, []int{1, 2, 3}, resultHandle) // no 100, 200
		assert.Equal(t, 2, len(handlerErrors))            // both errors handled

		// Test HandleUntil
		handlerUntilErrors := make([]error, 0)
		handlerUntil := func(err error) {
			if err != nil {
				handlerUntilErrors = append(handlerUntilErrors, err)
			}
		}
		resultUntil := make([]int, 0, 1)
		for val := range HandleUntil(makeSeq(), handlerUntil) {
			resultUntil = append(resultUntil, val)
		}
		assert.Equal(t, 1, len(resultUntil))        // stops at first error
		check.EqualItems(t, []int{1}, resultUntil)  // only before first error
		assert.Equal(t, 1, len(handlerUntilErrors)) // only first error handled
		assert.ErrorIs(t, handlerUntilErrors[0], err1)

		// Test HandleAll
		handlerAllErrors := make([]error, 0)
		handlerAll := func(err error) {
			handlerAllErrors = append(handlerAllErrors, err)
		}
		resultAll := make([]int, 0, 5)
		for val := range HandleAll(makeSeq(), handlerAll) {
			resultAll = append(resultAll, val)
		}
		assert.Equal(t, 5, len(resultAll))                       // all values
		check.EqualItems(t, []int{1, 100, 2, 200, 3}, resultAll) // including error values
		assert.Equal(t, 5, len(handlerAllErrors))                // handler called for all
		// handlerAllErrors includes nils
		assert.Equal(t, nil, handlerAllErrors[0])
		assert.ErrorIs(t, handlerAllErrors[1], err1)
		assert.Equal(t, nil, handlerAllErrors[2])
		assert.ErrorIs(t, handlerAllErrors[3], err2)
		assert.Equal(t, nil, handlerAllErrors[4])
	})
}
