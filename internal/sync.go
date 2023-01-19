package internal

import (
	"context"
	"sync"
)

// WaitGroup is a simple wrapper around sync.WaitGroup, but has a Wait
// function that respects a context.
type WaitGroup struct {
	wg sync.WaitGroup
}

func (wg *WaitGroup) Add(n int) { wg.wg.Add(n) }
func (wg *WaitGroup) Done()     { wg.wg.Done() }

// Wait returns when the all waiting items are done, *or* the context
// is canceled. This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled.
func (wg *WaitGroup) Wait(ctx context.Context) {
	sig := make(chan struct{})

	go func() { defer close(sig); wg.wg.Wait() }()

	select {
	case <-ctx.Done():
	case <-sig:
	}
}
