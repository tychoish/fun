package fun

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	wg.Add(1)
	cancel()
	if Wait(ctx, wg) {
		t.Fatal("wait group still blocked")
	}
	wg.Done()
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !Wait(ctx, wg) {
		t.Fatal("wait group should not be locked")
	}

}
