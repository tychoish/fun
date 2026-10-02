package pubsub

import (
	"testing"
)

func TestReduceProducesOneValue(t *testing.T) {
	r := VariadicStream(1, 2, 3).Reduce(func(a, b int) (int, error) { return a + b, nil })
	var got []int
	for r.Next(t.Context()) {
		got = append(got, r.Value())
		if len(got) > 5 {
			break
		}
	}
	if len(got) != 1 || got[0] != 6 {
		t.Fatalf("want exactly [6], got %v", got)
	}
}
