package itertool

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun"
)

type none struct{}

func CheckSeenMap[T comparable](t *testing.T, elems []T, seen map[T]struct{}) {
	t.Helper()
	if len(seen) != len(elems) {
		t.Fatal("all elements not iterated", "seen=", len(seen), "vs", "elems=", len(elems))
	}
	for idx, val := range elems {
		if _, ok := seen[val]; !ok {
			t.Error("element a not observed", idx, val)
		}
	}
}

func GenerateRandomStringSlice(size int) []string {
	out := make([]string, size)
	for idx := range out {
		out[idx] = fmt.Sprint("value=", idx)
	}
	return out
}

type FixtureData[T any] struct {
	Name     string
	Elements []T
}

type FixtureIteratorConstuctors[T any] struct {
	Name        string
	Constructor func([]T) *fun.Iterator[T]
}

type FixtureIteratorFilter[T any] struct {
	Name   string
	Filter func(*fun.Iterator[T]) *fun.Iterator[T]
}

func makeIntSlice(size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return out
}
