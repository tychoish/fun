package fun

import (
	"context"
	"sync"
)

// Wait returns when the all waiting items are done, *or* the context
// is canceled. This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled.
//
// The return value lets you know that there may still be pending work.
func Wait(ctx context.Context, wg *sync.WaitGroup) bool {
	sig := make(chan struct{})

	go func() { defer close(sig); wg.Wait() }()

	select {
	case <-ctx.Done():
		return false
	case <-sig:
		return true
	}
}
