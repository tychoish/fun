package fun

import (
	"iter"
	"slices"
	"testing"
)

func GenerateN[T any](num int, op func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < num && yield(op()); i++ {
			continue
		}
	}
}

func TestChunk(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		num      int
		expected [][]int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			num:      2,
			expected: [][]int{},
		},
		{
			name: "ExactChunks",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				yield(4)
			},
			num:      2,
			expected: [][]int{{1, 2}, {3, 4}},
		},
		{
			name: "PartialLastChunk",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			num:      2,
			expected: [][]int{{1, 2}, {3}},
		},
		{
			name: "SingleElementChunks",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			num:      1,
			expected: [][]int{{1}, {2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Chunk(tt.seq, tt.num)
			var result [][]int
			for chunk := range seq {
				if cv := slices.Collect(chunk); len(cv) == 0 {
					continue
				} else {
					result = append(result, cv)
				}
			}
			t.Cleanup(func() {
				if t.Failed() {
					return
				}
				t.Log(t, "result", result)
				t.Log(t, "expected", tt.expected)
			})

			if len(result) != len(tt.expected) {
				t.Errorf("Chunk() length = %v, want %v", len(result), len(tt.expected))
			}
			for i, chunk := range result {
				if i < len(tt.expected) && !slices.Equal(chunk, tt.expected[i]) {
					t.Errorf("Chunk()[%d] = %v, want %v", i, chunk, tt.expected[i])
				}
			}
		})
	}

	t.Run("ZeroChunkSize", func(t *testing.T) {
		seq := Chunk(func(yield func(int) bool) { yield(1) }, 0)
		count := 0
		for range seq {
			count++
			break
		}
		if count != 0 {
			t.Error("should not have iterated", count)
		}
	})

	t.Run("NegativeChunkSize", func(t *testing.T) {
		count := 0
		seq := Chunk(func(yield func(int) bool) { yield(1) }, -1)
		for range seq {
			count++
			break
		}
		if count != 0 {
			t.Error("should not have iterated", count)
		}
	})
	t.Run("EarlyReaturn", func(t *testing.T) {
		count := 0
		chunks := 0
		for chunk := range Chunk(GenerateN(30, func() int { count++; return count }), 3) {
			chunks++
			innerct := 0
			for item := range chunk {
				if item == 0 {
					t.Fatal("impossible")
				}
				innerct++
			}
			if chunks == 2 {
				break
			}

			if innerct == 0 {
				t.Error("should iterate", innerct, chunks, count)
			}
			if count%3 != 0 {
				t.Error("impossible chunk size", innerct, chunks, count)
			}
		}
		if count != 6 {
			t.Error("unexpected iteration", count)
		}
		if chunks != 2 {
			t.Error("unexpected chunk iteration", count)
		}
	})
}
