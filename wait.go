package fun

import (
	"context"
	"sync"
)

type WaitGroup struct {
	wg sync.WaitGroup
}

func (wg *WaitGroup) Add(n int) { wg.wg.Add(n) }
func (wg *WaitGroup) Done()     { wg.wg.Done() }
func (wg *WaitGroup) Wait(ctx context.Context) {
	sig := make(chan struct{})

	go func() { defer close(sig); wg.wg.Wait() }()

	select {
	case <-ctx.Done():
	case <-sig:
	}
}
