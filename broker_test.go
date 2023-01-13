package fun

import (
	"context"
	"fmt"
	"testing"
)

func TestBroker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for scope, elems := range map[string][]string{
		"Basic": {"a", "b", "c"},
		"ExtraLarge": func() []string {
			out := make([]string, 100)
			for idx := range out {
				out[idx] = fmt.Sprint("value=", idx)
			}
			return out
		}(),
	} {
		t.Run(scope, func(t *testing.T) {
			t.Run("Simple", func(t *testing.T) {
				for blocking, confValue := range map[string]bool{
					"Parallel": true,
					"Serial":   false,
				} {
					t.Run(blocking, func(t *testing.T) {
						broker := NewBroker[string](BrokerOptions{ParallelDispatch: confValue})
						broker.Start(ctx)
						ch1 := broker.Subscribe(ctx)
						ch2 := broker.Subscribe(ctx)

						seen1 := make(map[string]struct{}, len(elems))
						seen2 := make(map[string]struct{}, len(elems))
						wg := &WaitGroup{}
						wg.Add(3)

						go func() {
							defer wg.Done()
							for str := range ch1 {
								seen1[str] = struct{}{}
								if len(seen1) == len(elems) {
									broker.Unsubscribe(ctx, ch1)
									return
								}
							}
						}()

						go func() {
							defer wg.Done()
							for str := range ch2 {
								seen2[str] = struct{}{}
								if len(seen2) == len(elems) {
									broker.Unsubscribe(ctx, ch2)
									return
								}
							}
						}()

						go func() {
							defer wg.Done()
							for idx := range elems {
								broker.Publish(ctx, elems[idx])
							}
						}()

						wg.Wait(ctx)
						checkMatchingSets(t, seen1, seen2)
						broker.Stop()
						broker.Wait(ctx)
						cctx, ccancel := context.WithCancel(ctx)
						ccancel()
						if broker.Subscribe(cctx) != nil {
							t.Error("should not subscribe with canceled context")
						}
					})
				}
			})
			t.Run("BufferedBroker", func(t *testing.T) {
				t.Skip("test not implemented")
			})
			t.Run("NonBlockingSends", func(t *testing.T) {
				t.Skip("test not implemented")
			})
			t.Run("ParallelPublishing", func(t *testing.T) {
				// TODO: run with race detector
				t.Skip("test not implemented")
			})
		})
	}
}

func checkMatchingSets[T comparable](t *testing.T, set1, set2 map[T]struct{}) {
	t.Helper()
	if len(set1) != len(set2) {
		t.Fatal("sets are of different lengths")
	}

	for k := range set1 {
		if _, ok := set2[k]; !ok {
			t.Error("saw unknown key in set2", k)
		}
	}

	for k := range set2 {
		if _, ok := set1[k]; !ok {
			t.Error("saw unknown key in set1", k)
		}
	}
}
