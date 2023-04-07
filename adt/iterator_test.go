package adt

import (
	"sync"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestIterator(t *testing.T) {
	ctx := testt.Context(t)
	mp := &Map[string, int]{}
	mp.Store("one", 1)
	mp.Store("two", 2)
	mp.Store("three", 3)
	mp.Store("four", 4)

	iter := NewIterator(&sync.Mutex{}, mp.Iterator())
	seen := 0
	for iter.Next(ctx) {
		seen++
		item := iter.Value()
		if item.Value > 4 || item.Value <= 0 {
			t.Error(item)
		}
	}
	uiter := fun.Unwrap(iter)
	if uiter == iter {
		t.Error("unwrap should not be the same")
	}
	if _, ok := uiter.(*internal.ChannelIterImpl[MapItem[string, int]]); !ok {
		t.Errorf("%T", uiter)
	}

	if seen != 4 {
		t.Error(seen)
	}
	if iter.Close() != nil {
		t.Error(iter.Close())
	}

}
