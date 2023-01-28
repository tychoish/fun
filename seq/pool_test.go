package seq

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkPool(b *testing.B) {
	const size = 10000
	b.Run("MagicPool", func(b *testing.B) {
		pool := getElementPool("string")
		cache := map[int][]*Element[string]{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < size; j++ {
				elem := getElement(pool, fmt.Sprint("element=", i))
				cache[i] = append(cache[i], elem)
			}
			cache[i] = nil
		}
	})
	b.Run("MagicPoolForceGC", func(b *testing.B) {
		pool := getElementPool("string")
		cache := map[int][]*Element[string]{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < size; j++ {
				elem := getElement(pool, fmt.Sprint("element=", i))
				cache[i] = append(cache[i], elem)
			}
			cache[i] = nil
			runtime.GC()
		}
	})
	b.Run("MagicPoolForce", func(b *testing.B) {
		pool := getElementPool("string")
		cache := map[int][]*Element[string]{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < size; j++ {
				elem := getElement(pool, fmt.Sprint("element=", i))
				cache[i] = append(cache[i], elem)
			}
			cache[i] = nil
			b.StopTimer()
			runtime.GC()
			runtime.Gosched()
			time.Sleep(2 * time.Millisecond)
			b.StartTimer()
		}
	})
	b.Run("NormalPool", func(b *testing.B) {
		pool := &sync.Pool{New: func() any { return &Element[string]{} }}
		cache := map[int][]*Element[string]{}
		for i := 0; i < b.N; i++ {
			for j := 0; j < size; j++ {
				elem := pool.Get().(*Element[string])
				elem.item = fmt.Sprint("element=", i)
				elem.ok = true
				cache[i] = append(cache[i], elem)
			}
			for idx := range cache[i] {
				pool.Put(cache[i][idx])
			}
		}
	})

}
