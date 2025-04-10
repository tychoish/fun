package internal

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestLockHelpers(t *testing.T) {
	// mostly just no panicing?
	t.Run("Mutex", func(t *testing.T) {
		mtx := &sync.Mutex{}
		var number int64
		wg := &sync.WaitGroup{}
		for range 2 * runtime.NumCPU() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 1000 {
					func() {
						defer With(Lock(mtx))
						switch number {
						case 0:
							number = 2
						case 2:
							number = 4
						case 4:
							number = 8
						case 8:
							number = 16
						case 16:
							number = 32
						case 32:
							number = 2
						default:
							panic("should never happen")
						}
						time.Sleep(1 + time.Duration(rand.Int63n(number)))
					}()
				}
			}()
		}
		wg.Wait()
		if number%4 != 0 {
			t.Error("should be multiple of 4", number)
		}
	})
	t.Run("LockerStandardIndirection", func(t *testing.T) {
		mtx := &sync.Mutex{}
		var number int64
		wg := &sync.WaitGroup{}
		for range 2 * runtime.NumCPU() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 1000 {
					func() {
						defer WithL(LockL(mtx))
						switch number {
						case 0:
							number = 2
						case 2:
							number = 4
						case 4:
							number = 8
						case 8:
							number = 16
						case 16:
							number = 32
						case 32:
							number = 2
						default:
							panic("should never happen")
						}
						time.Sleep(1 + time.Duration(rand.Int63n(number)))
					}()
				}
			}()
		}
		wg.Wait()
		if number%4 != 0 {
			t.Error("should be multiple of 4", number)
		}
	})
	t.Run("LockerRead", func(t *testing.T) {
		mtx := &sync.RWMutex{}
		var number int64
		wg := &sync.WaitGroup{}
		for range 2 * runtime.NumCPU() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 1000 {
					func() {
						defer WithL(LockL(mtx))
						switch number {
						case 0:
							number = 2
						case 2:
							number = 4
						case 4:
							number = 8
						case 8:
							number = 16
						case 16:
							number = 32
						case 32:
							number = 2
						default:
							panic("should never happen")
						}
						time.Sleep(1 + time.Duration(rand.Int63n(number)))
					}()
				}
			}()
		}
		wg.Wait()
		if number%4 != 0 {
			t.Error("should be multiple of 4", number)
		}
	})

}
