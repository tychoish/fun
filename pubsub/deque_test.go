package pubsub

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/set"
)

type fixture[T any] struct {
	name     string
	add      func(T) error
	remove   func() (T, bool)
	iterator func() fun.Iterator[T]
	close    func() error
	len      func() int
	elems    []T
	reverse  bool
}

func generateRandomElems(size int) []string {
	elems := make([]string, 50)

	for i := 0; i < 50; i++ {
		elems[i] = fmt.Sprintf("value=%d", i)
	}
	return elems
}

func TestLinkedListImplementations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, f := range []func() fixture[string]{
		func() fixture[string] {
			cue := NewUnlimitedQueue[string]()

			return fixture[string]{
				name:     "QueueUnlimited",
				add:      cue.Add,
				remove:   cue.Remove,
				iterator: cue.Iterator,
				elems:    generateRandomElems(50),
				close:    cue.Close,
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewQueue[string](QueueOptions{HardLimit: 100, SoftQuota: 60}))

			return fixture[string]{
				name:     "QueueLimited",
				add:      cue.Add,
				remove:   cue.Remove,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Unlimited: true}))

			return fixture[string]{
				name:     "DequePushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Unlimited: true}))

			return fixture[string]{
				name:     "DequePushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Unlimited: true}))

			return fixture[string]{
				name:     "DequePushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				close:    cue.Close,
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Unlimited: true}))

			return fixture[string]{
				name:     "DequePushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
				close:    cue.Close,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Unlimited: true}))

			return fixture[string]{
				name:     "DequePushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		// simple capacity
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Capacity: 50}))

			return fixture[string]{
				name:     "DequeCapacityPushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Capacity: 50}))

			return fixture[string]{
				name:     "DequeCapacityPushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				close:    cue.Close,
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Capacity: 50}))

			return fixture[string]{
				name:     "DequeCapacityPushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
				close:    cue.Close,
			}
		},

		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{Capacity: 50}))

			return fixture[string]{
				name:     "DequeCapacityPushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},

		// bursty limited size
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[string]{
				name:     "DequeBurstPushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[string]{
				name:     "DequeBurstPushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				close:    cue.Close,
				len:      cue.tracker.len,
			}
		},
		func() fixture[string] {
			cue := fun.Must(NewDeque[string](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[string]{
				name:     "DequeBurstPushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    generateRandomElems(50),
				len:      cue.tracker.len,
				close:    cue.Close,
			}
		},
	} {
		fix := f()
		t.Run(fix.name, func(t *testing.T) {
			t.Run("AddRemove", func(t *testing.T) {
				fix := f()
				for _, e := range fix.elems {
					if err := fix.add(e); err != nil {
						t.Fatal(err)
					}
				}

				if fix.len() != len(fix.elems) {
					t.Fatal("add did not work")
				}

				set := set.NewUnordered[string]()
				for i := len(fix.elems); i > 0; i-- {
					out, ok := fix.remove()
					if !ok {
						t.Error("remove should not fail", i)
					}
					set.Add(out)
				}
				if set.Len() != len(fix.elems) {
					t.Fatal("did not see all expected results", set.Len(), len(fix.elems))
				}

				if fix.len() != 0 {
					t.Error("remove did not work")
				}
			})
			t.Run("Iterate", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()

				fix := f()
				for _, e := range fix.elems {
					if err := fix.add(e); err != nil {
						t.Fatal(err)
					}
				}

				seen := 0
				iter := fix.iterator()

				if err := fix.close(); err != nil {
					t.Fatal(err)
				}

				for iter.Next(ctx) {
					seen++
					if iter.Value() == "" {
						t.Fatal("problem at", seen)
					}
				}
				if seen != len(fix.elems) {
					t.Fatal("did not iterate far enough", seen, len(fix.elems))
				}

				if err := ctx.Err(); err != nil {
					t.Error("shouldn't cancel", err)
				}
				if err := iter.Close(ctx); err != nil {
					t.Fatal(err)
				}
			})

		})
	}

}
