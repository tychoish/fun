package adt

import (
	"sync"
	"testing"
)

func doTimes(num int, op func()) {
	wg := &sync.WaitGroup{}
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func() { defer wg.Done(); op() }()
	}
	wg.Wait()
}

func TestAtomic(t *testing.T) {
	t.Run("Integers", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			atom := &AtomicInteger[int32]{}
			doTimes(256, func() {
				atom.Set(100)
				if v := atom.Get(); v != 100 {
					t.Error("v is not == ", v)
				}
			})
		})
		t.Run("Add", func(t *testing.T) {
			atom := &AtomicInteger[uint]{}
			doTimes(256, func() {
				val := atom.Add(100)
				if val < 100 {
					t.Error("v is not == ", val)
				}
				if val%100 != 0 {
					t.Error("v is not == ", val)
				}
			})
		})
		t.Run("Swap", func(t *testing.T) {
			atom := &AtomicInteger[uint]{}
			doTimes(256, func() {
				val := atom.Swap(100)
				if val == 0 || val == 100 {
					val = atom.Swap(0)
					if val == 0 || val == 100 {
						return
					}
					t.Error("v is  == ", val)
				}
				t.Error("v is  == ", val)
			})
			if val := atom.Load(); val != 0 {
				t.Error(val)
			}
		})
		t.Run("CompareAndSwap", func(_ *testing.T) {
			atom := &AtomicInteger[uint32]{}
			doTimes(256, func() {
				for i := uint32(0); i < 100; i++ {
					for {
						prev := atom.Load()
						if atom.CompareAndSwap(prev, i) {
							break
						}
					}
				}
			})
		})
	})
	t.Run("Floats", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			atom := &AtomicFloat64{}
			doTimes(256, func() {
				atom.Set(100)
				if v := atom.Get(); v != 100 {
					t.Error("v is not == ", v)
				}
			})
		})
		t.Run("Add", func(t *testing.T) {
			atom := &AtomicFloat64{}
			doTimes(256, func() {
				val := atom.Add(10.5)
				if val < 10.5 {
					t.Error("v is not == ", val)
				}
			})
			cur := atom.Load()
			if cur != 10.5*256 {
				t.Error(cur)
			}
		})

		t.Run("Swap", func(t *testing.T) {
			atom := &AtomicFloat64{}
			doTimes(256, func() {
				val := atom.Swap(100)
				if val == 0 || val == 100 {
					val = atom.Swap(0)
					if val == 0 || val == 100 {
						return
					}
					t.Error("v is  == ", val)
				}
				t.Error("v is  == ", val)
			})
			cur := atom.Load()
			if cur != 0 {
				t.Error(cur)
			}
		})
		t.Run("CompareAndSwap", func(t *testing.T) {
			atom := &AtomicFloat64{}
			doTimes(256, func() {
				for i := float64(0); i < 100; i++ {
					for {
						prev := atom.Load()
						if atom.CompareAndSwap(prev, i) {
							break
						}
					}
				}
			})
			cur := atom.Load()
			if cur == 0 {
				t.Error(cur)
			}
		})
	})
}
