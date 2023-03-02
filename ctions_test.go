package fun

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func makeSliceOfAllN(n int, size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = n
	}
	return out
}

func TestProcess(t *testing.T) {
	t.Run("One", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			sum := 0
			ProcessAll(func(in int) { sum += in }, makeSliceOfAllN(1, 100))
			check.Equal(t, sum, 100)
		})
		t.Run("Variadic", func(t *testing.T) {
			sum := 0
			Process(func(in int) { sum += in }, makeSliceOfAllN(1, 100)...)
			check.Equal(t, sum, 100)
		})
	})
	t.Run("Two", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			sum := 0
			ProcessAll(func(in int) { sum += in }, makeSliceOfAllN(2, 128))
			check.Equal(t, sum, 256)
		})
		t.Run("Variadic", func(t *testing.T) {
			sum := 0
			Process(func(in int) { sum += in }, makeSliceOfAllN(2, 128)...)
			check.Equal(t, sum, 256)
		})
	})
}
