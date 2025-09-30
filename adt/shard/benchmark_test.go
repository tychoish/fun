package shard

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

func BenchmarkShardedMapTypeComparison(b *testing.B) {
	for _, nShards := range []int{defaultSize, defaultSize / 2, defaultSize / 4, defaultSize / 8, defaultSize / 16, defaultSize / 32} {
		b.Run(fmt.Sprintf("%dShards", nShards), func(b *testing.B) {
			for _, bench := range []struct {
				Name    string
				MapType MapType
			}{
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
						wg := &sync.WaitGroup{}
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							mp := &Map[string, int]{}
							mp.Setup(nShards, bench.MapType)
							b.StartTimer()

							for range i {
								wg.Add(1)
								go func(_ int) {
									defer wg.Done()
									for range i {
										populateMap(mp, i)
										runtime.Gosched()
									}
								}(i)
							}
							wg.Wait()
						}
					})
					b.Run("Reads", func(b *testing.B) {
						wg := &sync.WaitGroup{}
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							mp := &Map[string, int]{}
							mp.Setup(nShards, bench.MapType)
							populateMap(mp, i)
							b.StartTimer()
							for range i {
								wg.Add(1)
								go func(_ int) {
									defer wg.Done()
									for range i {
										checkMap(b, mp, i)
										runtime.Gosched()
									}
								}(i)
							}
							wg.Wait()
						}
					})
					b.Run("Mixed", func(b *testing.B) {
						wg := &sync.WaitGroup{}
						for i := 0; i < b.N; i++ {
							b.StopTimer()
							mp := &Map[string, int]{}
							mp.Setup(nShards, bench.MapType)
							populateMap(mp, b.N)
							b.StartTimer()
							for range i {
								wg.Add(1)
								go func(_ int) {
									defer wg.Done()
									for range i {
										checkMap(b, mp, i)
										runtime.Gosched()
									}
								}(i)
								wg.Add(1)
								go func(_ int) {
									defer wg.Done()
									for range i {
										populateMap(mp, i)
										runtime.Gosched()
									}
								}(i)
							}
							wg.Wait()
						}
					})
				})
			}
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
