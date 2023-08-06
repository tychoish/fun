//go:build race

package hdrhist_test

import (
	"fmt"
	"runtime"
	"sync"
)

var mtx *sync.Mutex

func init() {
	mtx = &sync.Mutex{}
	fmt.Println("THE RACE DETECTOR IS DISABLED FOR HDRHIST")
}

func without(int)       { runtime.RaceEnable(); mtx.Unlock() }
func raceDetector() int { mtx.Lock(); runtime.RaceDisable(); return 0 }
