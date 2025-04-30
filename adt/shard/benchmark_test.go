package shard

import (
	"runtime"
	"strconv"
	"sync"
	"testing"
)

func BenchmarkShardedMapTypeComparison(b *testing.B) {
	for _, bench := range []struct {
		Name    string
		MapType MapType
	}{
		// {
		// 	Name:    "Default",
		// 	MapType: MapTypeDefault,
		// },
		{
			Name:    "Mutex",
			MapType: MapTypeMutex,
		},
		{
			Name:    "SyncMap",
			MapType: MapTypeSync,
		},
		{
			Name:    "RWMutex",
			MapType: MapTypeRWMutex,
		},
	} {
		b.Run(bench.Name, func(b *testing.B) {
			b.Run("Populate", func(b *testing.B) {
				mp := &Map[string, int]{}
				mp.Setup(-1, bench.MapType)
				wg := &sync.WaitGroup{}
				for i := 0; i < b.N; i++ {
					for range i {
						wg.Add(1)
						go func(n int) {
							defer wg.Done()
							for range i {
								populateMap(mp, b.N*2)
								runtime.Gosched()
							}
						}(i)
					}
					wg.Wait()
				}
			})
			b.Run("Reads", func(b *testing.B) {
				mp := &Map[string, int]{}
				mp.Setup(-1, bench.MapType)
				populateMap(mp, b.N*2)
				wg := &sync.WaitGroup{}
				for i := 0; i < b.N; i++ {
					for range i {
						wg.Add(1)
						go func(n int) {
							defer wg.Done()
							for range i {
								checkMap(b, mp, b.N*2)
								runtime.Gosched()
							}
						}(i)
					}
					wg.Wait()
				}
			})
			b.Run("Mixed", func(b *testing.B) {
				mp := &Map[string, int]{}
				mp.Setup(-1, bench.MapType)
				populateMap(mp, b.N*32)
				wg := &sync.WaitGroup{}
				for i := 0; i < b.N; i++ {
					for range i {
						wg.Add(1)
						go func(n int) {
							defer wg.Done()
							for range i {
								checkMap(b, mp, b.N*32)
								runtime.Gosched()
							}
						}(i)
						wg.Add(1)
						go func(n int) {
							defer wg.Done()
							for range i {
								populateMap(mp, b.N*32)
								runtime.Gosched()
							}
						}(i)

					}
					wg.Wait()
				}
			})
		})
	}

}

func populateMap(mp *Map[string, int], n int) {
	for i := range n {
		mp.Store(strconv.Itoa(i), i)
	}
}

func checkMap(b *testing.B, mp *Map[string, int], n int) {
	for i := range n {
		str := strconv.Itoa(i)
		val, ok := mp.Load(str)
		if !ok || val != i {
			b.Error("impossible values", i, str, val, ok)
		}
	}

}
