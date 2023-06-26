package fun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/testt"
)

func CheckSeenMap[T comparable](t *testing.T, elems []T, seen map[T]struct{}) {
	t.Helper()
	if len(seen) != len(elems) {
		t.Fatal("all elements not iterated", "seen=", len(seen), "vs", "elems=", len(elems))
	}
	for idx, val := range elems {
		if _, ok := seen[val]; !ok {
			t.Error("element a not observed", idx, val)
		}
	}
}

func sum(in []int) (out int) {
	for _, num := range in {
		out += num
	}
	return out
}

type FixtureData[T any] struct {
	Name     string
	Elements []T
}

type FixtureIteratorConstuctors[T any] struct {
	Name        string
	Constructor func([]T) *Iterator[T]
}

type FixtureIteratorFilter[T any] struct {
	Name   string
	Filter func(*Iterator[T]) *Iterator[T]
}

func makeIntSlice(size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return out
}

func TestTools(t *testing.T) {
	t.Parallel()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
			t.Parallel()
			t.Run("CancelCollectChannel", func(t *testing.T) {
				bctx, bcancel := context.WithCancel(context.Background())
				defer bcancel()

				ctx, cancel := context.WithCancel(bctx)
				defer cancel()

				pipe := make(chan string, 1)
				sig := make(chan struct{})

				go func() {
					defer close(sig)
					for {
						select {
						case <-bctx.Done():
							return
						case pipe <- t.Name():
							continue
						}
					}
				}()

				output := Blocking(pipe).Iterator().Channel(ctx)
				runtime.Gosched()

				count := 0
			CONSUME:
				for {
					select {
					case _, ok := <-output:
						if ok {
							count++
							cancel()
						}
						if !ok {
							break CONSUME
						}
					case <-sig:
						break CONSUME
					case <-time.After(50 * time.Millisecond):
						break CONSUME
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})
		})
	}
}

func TestMapReduce(t *testing.T) {
	t.Parallel()
	t.Run("MapWorkerSendingBlocking", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string, 1)
		output := make(chan int)
		wg := &WaitGroup{}
		pipe <- t.Name()

		var mf Transform[string, int] = func(ctx context.Context, in string) (int, error) { return 53, nil }
		mf.Safe().Processor(Blocking(output).Send().Write, &WorkerGroupConf{}).
			ReadAll(Blocking(pipe).Receive().Producer()).
			Ignore().
			Add(ctx, wg)

		time.Sleep(10 * time.Millisecond)
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		wg.Wait(ctx)

		count := 0
	CONSUME:
		for {
			select {
			case _, ok := <-output:
				if ok {
					count++
					continue
				}
				break CONSUME
			case <-ctx.Done():
				break CONSUME
			}
		}
	})
}

func getConstructors[T comparable](t *testing.T) []FixtureIteratorConstuctors[T] {
	return []FixtureIteratorConstuctors[T]{
		{
			Name: "SlicifyIterator",
			Constructor: func(elems []T) *Iterator[T] {
				return SliceIterator(elems)
			},
		},
		{
			Name: "Slice",
			Constructor: func(elems []T) *Iterator[T] {
				return SliceIterator(elems)
			},
		},
		{
			Name: "VariadicIterator",
			Constructor: func(elems []T) *Iterator[T] {
				return VariadicIterator(elems...)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []T) *Iterator[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return Blocking(vals).Iterator()
			},
		},
	}

}

func TestIteratorImplementations(t *testing.T) {
	t.Parallel()
	elems := []FixtureData[string]{
		{
			Name:     "Basic",
			Elements: []string{"a", "b", "c", "d"},
		},
		{
			Name:     "Large",
			Elements: GenerateRandomStringSlice(100),
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(t, elems, getConstructors[string](t))
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorStringAlgoTests(t, elems, getConstructors[string](t))
	})
}

func TestIteratorAlgoInts(t *testing.T) {
	t.Parallel()

	elemGenerator := func() []int {
		e := make([]int, 100)
		for idx := range e {
			e[idx] = idx
		}
		return e
	}

	elems := []FixtureData[int]{
		{
			Name:     "Basic",
			Elements: []int{1, 2, 3, 4, 5},
		},
		{
			Name:     "Large",
			Elements: elemGenerator(),
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(t, elems, getConstructors[int](t))
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorIntegerAlgoTests(t, elems, getConstructors[int](t))
	})
}

func TestParallelForEach(t *testing.T) {
	ctx := testt.Context(t)

	t.Run("Basic", func(t *testing.T) {
		for i := int64(-1); i <= 12; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				i := i
				t.Parallel()
				elems := makeIntSlice(200)
				seen := &atomic.Int64{}
				count := &atomic.Int64{}

				err := SliceIterator(elems).ProcessParallel(
					func(ctx context.Context, in int) error {
						abs := int64(math.Abs(float64(i)))
						count.Add(1)

						jitter := time.Duration(rand.Int63n(1 + abs*int64(time.Millisecond)))
						time.Sleep(time.Millisecond + jitter)
						seen.Add(int64(in))
						return nil
					},
					WorkerGroupConfNumWorkers(int(i))).Run(ctx)

				if err != nil {
					t.Fatal(err)
				}

				check.Equal(t, int(seen.Load()), sum(elems))
				testt.Log(t, "input", elems)

				if int(count.Load()) != len(elems) {
					t.Error("unequal length slices")
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		count := &atomic.Int64{}
		errCount := &atomic.Int64{}
		err := SliceIterator(makeIntSlice(200)).
			ProcessParallel(
				func(ctx context.Context, in int) error {
					count.Add(1)
					runtime.Gosched()
					if in >= 100 {
						errCount.Add(1)
						panic("error")
					}
					return nil
				},
				WorkerGroupConfNumWorkers(3),
				WorkerGroupConfContinueOnPanic(),
				WorkerGroupConfWithErrorCollector(&Collector{}),
			).Run(ctx)
		if err == nil {
			t.Fatal("should not have errored", err)
		}

		if errCount.Load() != 100 {
			t.Error(errCount.Load())
		}
		if count.Load() != 200 {
			t.Error(count.Load())
		}
		check.Equal(t, 200, len(ers.Unwind(err)))
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seenCount := &atomic.Int64{}
		paned := &atomic.Bool{}

		err := SliceIterator(makeIntSlice(10)).
			ProcessParallel(
				func(ctx context.Context, in int) error {
					if in == 8 {
						paned.Store(true)
						// make sure something else
						// has a chance to run before
						// the event.
						runtime.Gosched()
						panic("gotcha")
					}

					seenCount.Add(1)
					<-ctx.Done()
					return nil
				},
				WorkerGroupConfNumWorkers(10),
			).Run(ctx)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		check.True(t, paned.Load())
		if seenCount.Load() != 9 {
			t.Error("should have only seen", 9, "saw", seenCount.Load())
		}
		errs := ers.Unwind(err)
		if len(errs) != 2 {
			// panic + expected
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceIterator(makeIntSlice(10)).
			ProcessParallel(
				func(ctx context.Context, in int) error {
					if in == 4 {
						cancel()
						panic("gotcha")
					}
					return nil
				},
				WorkerGroupConfNumWorkers(8),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}
	})
	t.Run("CollectAllContinuedErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := &atomic.Int64{}

		err := SliceIterator(makeIntSlice(10)).
			ProcessParallel(
				func(ctx context.Context, in int) error {
					count.Add(1)
					return fmt.Errorf("errored=%d", in)
				},
				WorkerGroupConfNumWorkers(4),
				WorkerGroupConfContinueOnError(),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}
		testt.Log(t, err)
		check.Equal(t, 10, count.Load())

		errs := ers.Unwind(err)
		if len(errs) != 10 {
			t.Error(len(errs), "!= 10", errs)
		}

	})
	t.Run("CollectAllErrors/Double", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceIterator(makeIntSlice(100)).
			ProcessParallel(
				func(ctx context.Context, in int) error {
					return fmt.Errorf("errored=%d", in)
				},
				WorkerGroupConfNumWorkers(2),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := ers.Unwind(err)
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > 2 {
			t.Error(len(errs))
		}
	})
	t.Run("CollectAllErrors/Cores", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceIterator(makeIntSlice(100)).ProcessParallel(
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			WorkerGroupConfWorkerPerCPU(),
		).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := ers.Unwind(err)
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > runtime.NumCPU() {
			t.Error(len(errs))
		}
	})
	t.Run("IncludeContextErrors", func(t *testing.T) {
		t.Run("SuppressErrorsByDefault", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := SliceIterator(makeIntSlice(2)).ProcessParallel(
				func(ctx context.Context, in int) error {
					return context.Canceled
				},
			).Run(ctx)
			check.NotError(t, err)
			if err != nil {
				t.Error("should have skipped all errors", err)
			}
		})
		t.Run("WithErrors", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := SliceIterator(makeIntSlice(2)).ProcessParallel(
				func(ctx context.Context, in int) error {
					return context.Canceled
				},
				WorkerGroupConfIncludeContextErrors(),
			).Run(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, context.Canceled)
		})
	})
}

func RunIteratorImplementationTests[T comparable](
	t *testing.T,
	elements []FixtureData[T],
	builders []FixtureIteratorConstuctors[T],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					// name := builder.Name
					t.Parallel()

					builder := func() *Iterator[T] { return baseBuilder(elems) }

					t.Run("Single", func(t *testing.T) {
						ctx := testt.Context(t)

						seen := make(map[T]struct{}, len(elems))
						iter := builder()

						for iter.Next(ctx) {
							seen[iter.Value()] = struct{}{}
						}
						if err := iter.Close(); err != nil {
							t.Error(err)
						}

						CheckSeenMap(t, elems, seen)
					})
					t.Run("Canceled", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()
						ctx, cancel := context.WithCancel(ctx)
						cancel()
						var count int

						for iter.Next(ctx) {
							count++
						}
						err := iter.Close()
						if count > len(elems) && !errors.Is(err, context.Canceled) {
							t.Fatal("should not have iterated or reported err", count, err)
						}
					})
					t.Run("PanicSafety", func(t *testing.T) {
						t.Run("Map", func(t *testing.T) {
							ctx := testt.Context(t)

							out, err := Map[T](
								baseBuilder(elems),
								func(ctx context.Context, input T) (T, error) {
									panic("whoop")
								},
							).Slice(ctx)

							if err == nil {
								t.Error("expected error")
							}

							check.ErrorIs(t, err, ErrRecoveredPanic)
							testt.Log(t, len(out), ":", out)
							if len(out) != 0 {
								t.Fatal("unexpected output", out)
							}
						})
						t.Run("ParallelMap", func(t *testing.T) {
							ctx := testt.Context(t)

							out, err := Map(
								baseBuilder(elems),
								func(ctx context.Context, input T) (T, error) {
									panic("whoop")
								},
								WorkerGroupConfNumWorkers(2),
								WorkerGroupConfContinueOnError(),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error")
							}

							assert.ErrorIs(t, err, ErrRecoveredPanic)

							if !strings.Contains(err.Error(), "whoop") {
								t.Fatalf("panic error isn't propogated %q", err.Error())
							}
							if len(out) != 0 {
								t.Fatal("unexpected output", out)
							}
						})
					})
				})
			}
		})
	}
}

func RunIteratorIntegerAlgoTests(
	t *testing.T,
	elements []FixtureData[int],
	builders []FixtureIteratorConstuctors[int],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					t.Parallel()

					t.Run("Map", func(t *testing.T) {
						t.Run("ErrorDoesNotAbort", func(t *testing.T) {
							ctx := testt.Context(t)

							out, err := Map(
								baseBuilder(elems),
								func(ctx context.Context, input int) (int, error) {
									if input == elems[2] {
										return 0, errors.New("whoop")
									}
									return input, nil
								},
								WorkerGroupConfContinueOnError(),
							).Slice(ctx)
							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "whoop" {
								t.Error(err)
							}
							testt.Log(t, err)
							if len(out) != len(elems)-1 {
								t.Fatal("unexpected output", len(out), "->", out, len(elems)-1)
							}
						})

						t.Run("PanicDoesNotAbort", func(t *testing.T) {
							ctx := testt.Context(t)

							out, err := Map(
								baseBuilder(elems),
								func(ctx context.Context, input int) (int, error) {
									if input == elems[3] {
										panic("whoops")
									}
									return input, nil
								},
								WorkerGroupConfContinueOnPanic(),
								WorkerGroupConfNumWorkers(1),
							).Slice(ctx)

							testt.Log(t, elems)

							if err == nil {
								t.Error("expected error", err)
							}
							check.ErrorIs(t, err, ErrRecoveredPanic)
							if len(out) != len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							ctx := testt.Context(t)
							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(ctx context.Context, input int) (int, error) {
									if input >= elems[2] {
										return 0, expectedErr
									}
									return input, nil
								},
								WorkerGroupConfNumWorkers(1),
							).Slice(ctx)
							if err == nil {
								t.Error("expected error")
							}
							if !errors.Is(err, expectedErr) {
								t.Error(err)
							}
							// we should abort, but there's some asynchronicity.
							if len(out) > len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ParallelErrorDoesNotAbort", func(t *testing.T) {
							ctx := testt.Context(t)

							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(ctx context.Context, input int) (int, error) {
									if input == len(elems)/2+1 {
										return 0, expectedErr
									}
									return input, nil
								},
								WorkerGroupConfNumWorkers(4),
								WorkerGroupConfContinueOnError(),
							).Slice(ctx)
							if err == nil {
								t.Error("expected error")
							}
							if !errors.Is(err, expectedErr) {
								t.Error(err)
							}

							if len(out) != len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out, len(elems))
							}
						})
					})
				})
			}
		})
	}
}

func RunIteratorStringAlgoTests(
	t *testing.T,
	elements []FixtureData[string],
	builders []FixtureIteratorConstuctors[string],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					name := builder.Name
					t.Parallel()

					builder := func() *Iterator[string] { return (baseBuilder(elems)) }
					t.Run("Channel", func(t *testing.T) {
						ctx := testt.Context(t)

						seen := make(map[string]struct{}, len(elems))
						iter := builder()
						ch := iter.Channel(ctx)
						for str := range ch {
							seen[str] = struct{}{}
						}
						CheckSeenMap(t, elems, seen)
						if err := iter.Close(); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("Collect", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()
						vals, err := iter.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}
						// skip implementation with random order
						if name != "SetIterator" {
							check.EqualItems(t, elems, vals)
						}
						if err := iter.Close(); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("Map", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()
						out := Map[string, string](
							iter,
							func(ctx context.Context, str string) (string, error) {
								return str, nil
							},
						)

						vals, err := out.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}
						testt.Log(t, vals)
						// skip implementation with random order
						if name != "SetIterator" {
							check.EqualItems(t, elems, vals)
						}
					})
					t.Run("ParallelMap", func(t *testing.T) {
						ctx := testt.Context(t)

						out := Map(
							MergeIterators(builder(), builder(), builder()),
							func(ctx context.Context, str string) (string, error) {
								for _, c := range []string{"a", "e", "i", "o", "u"} {
									str = strings.ReplaceAll(str, c, "")
								}
								return strings.TrimSpace(str), nil
							},
							WorkerGroupConfNumWorkers(4),
						)

						vals, err := out.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}
						longString := strings.Join(vals, "")
						count := 0
						for _, i := range longString {
							switch i {
							case 'a', 'e', 'i', 'o', 'u':
								count++
							case '\n', '\t':
								count += 100
							}
						}
						if count != 0 {
							t.Error("unexpected result", count)
						}
					})
					t.Run("Generate", func(t *testing.T) {
						t.Run("Basic", func(t *testing.T) {
							ctx := testt.Context(t)

							inputs := GenerateRandomStringSlice(512)
							count := &atomic.Int32{}
							out := Producer[string](func(ctx context.Context) (string, error) {
								count.Add(1)
								if int(count.Load()) > len(inputs) {
									return "", io.EOF
								}
								return inputs[rand.Intn(511)], nil
							}).GenerateParallel()
							sig := make(chan struct{})
							go func() {
								defer close(sig)
								vals, err := out.Slice(ctx)
								if err != nil {
									t.Error(err)
								}
								if len(vals) != len(inputs) {
									t.Error("unexpected result", count.Load(), len(vals), len(inputs))
								}
							}()
							<-sig
						})
						t.Run("GenerateParallel", func(t *testing.T) {
							ctx := testt.Context(t)

							inputs := GenerateRandomStringSlice(512)
							count := &atomic.Int32{}
							out := Producer[string](func(ctx context.Context) (string, error) {
								count.Add(1)
								if int(count.Load()) > len(inputs) {
									return "", io.EOF
								}
								return inputs[rand.Intn(511)], nil
							}).GenerateParallel(WorkerGroupConfNumWorkers(4))
							sig := make(chan struct{})
							go func() {
								defer close(sig)
								vals, err := out.Slice(ctx)
								if err != nil {
									t.Error(err)
								}
								// aborting may not happen at the same moment, given this locking model
								if len(vals)+16 < len(inputs) {
									t.Error("unexpected result", len(vals), len(inputs))
								}
							}()
							<-sig
						})
						t.Run("PanicSafety", func(t *testing.T) {
							ctx := testt.Context(t)
							out := Producer[string](func(ctx context.Context) (string, error) {
								panic("foo")
							}).GenerateParallel()

							if out.Next(ctx) {
								t.Fatal("should not iterate when panic")
							}

							err := out.Close()
							assert.ErrorIs(t, err, ErrRecoveredPanic)
							assert.Substring(t, err.Error(), "foo")
						})
						t.Run("ContinueOnPanic", func(t *testing.T) {
							ctx := testt.Context(t)

							count := 0
							out := Producer[string](
								func(ctx context.Context) (string, error) {
									count++
									if count == 3 {
										panic("foo")
									}

									if count == 5 {
										return "", io.EOF
									}
									return fmt.Sprint(count), nil
								},
							).GenerateParallel(WorkerGroupConfContinueOnPanic())
							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Log(err, output)
								t.Error(l)
							}
							if err == nil {
								t.Fatal("should have errored")
							}
							assert.Substring(t, err.Error(), "foo")
							assert.ErrorIs(t, err, ErrRecoveredPanic)
						})
						t.Run("ArbitraryErrorAborts", func(t *testing.T) {
							ctx := testt.Context(t)

							count := 0
							out := Producer[string](func(ctx context.Context) (string, error) {
								count++
								if count == 4 {
									return "", errors.New("beep")
								}
								return "foo", nil
							}).GenerateParallel()

							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Error(l)
							}
							if err == nil {
								t.Fatal("should have errored")
							}

							if err.Error() != "beep" {
								t.Log(len(ers.Unwind(err)))
								t.Fatalf("unexpected panic %q", err.Error())
							}
						})
						t.Run("ContinueOnError", func(t *testing.T) {
							ctx := testt.Context(t)
							count := 0
							out := Producer[string](func(ctx context.Context) (string, error) {
								count++
								if count == 3 {
									return "", errors.New("beep")
								}
								if count == 5 {
									return "", io.EOF
								}
								return "foo", nil
							}).GenerateParallel(WorkerGroupConfContinueOnError())

							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Error(l, output)
							}
							if err == nil {
								t.Fatal("should have errored")
							}
							if err.Error() != "beep" {
								t.Fatalf("unexpected error %q", err.Error())
							}
						})
					})

					t.Run("Reduce", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()
						seen := make(map[string]struct{}, len(elems))
						sum, err := iter.Reduce(func(in string, value string) (string, error) {
							seen[in] = struct{}{}
							return fmt.Sprint(value, in), nil
						})(ctx)

						if err != nil {
							t.Fatal(err)
						}
						CheckSeenMap(t, elems, seen)
						seenSum := 0
						for str := range seen {
							seenSum += len(str)
						}
						if seenSum != len(sum) {
							t.Errorf("incorrect seen %d, reduced %v", seenSum, sum)
						}
					})
					t.Run("ReduceError", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()

						seen := map[string]none{}

						count := 0
						_, err := iter.Reduce(
							func(in string, val string) (string, error) {
								check.Zero(t, val)
								seen[in] = none{}
								if len(seen) == 2 {
									return val, errors.New("boop")
								}
								count++
								return "", nil
							},
						)(ctx)
						check.Equal(t, count, 1)
						if err == nil {
							t.Fatal("expected error")
						}

						if e := err.Error(); e != "boop" {
							t.Error("unexpected error:", e)
						}
						if l := len(seen); l != 2 {
							t.Error("seen", l, seen)
						}
					})
					t.Run("ReduceSkip", func(t *testing.T) {
						elems := makeIntSlice(32)
						iter := SliceIterator(elems)
						count := 0
						sum, err := iter.Reduce(func(in int, value int) (int, error) {
							count++
							if count == 1 {
								return 42, nil
							}
							return value, ErrIteratorSkip
						})(testt.Context(t))
						assert.NotError(t, err)
						assert.Equal(t, sum, 42)
						assert.Equal(t, count, 32)
					})
					t.Run("ReduceEarlyExit", func(t *testing.T) {
						elems := makeIntSlice(32)
						iter := SliceIterator(elems)
						count := 0
						sum, err := iter.Reduce(func(in int, value int) (int, error) {
							count++
							if count == 16 {
								return 300, io.EOF
							}
							return 42, nil
						})(testt.Context(t))
						assert.NotError(t, err)
						assert.Equal(t, sum, 42)
						assert.Equal(t, count, 16)
					})

				})
			}
		})
	}
	t.Run("Collector", func(t *testing.T) {
		ec := &Collector{}
		check.Zero(t, ec.Len())
		check.NotError(t, ec.Resolve())
		check.True(t, !ec.HasErrors())
		const ErrCountMeOut ers.Error = "countm-me-out"
		op := func() error { return ErrCountMeOut }

		ec.Add(ers.Join(ers.Error("beep"), context.Canceled))
		check.True(t, ec.HasErrors())
		check.Equal(t, 1, ec.Len())
		ec.Add(op())
		check.Equal(t, 2, ec.Len())
		ec.Add(ers.Error("boop"))
		check.Equal(t, 3, ec.Len())

		ec.Add(nil)
		check.Equal(t, 3, ec.Len())
		check.True(t, ec.HasErrors())

		err := ec.Resolve()
		check.Error(t, err)
		check.Error(t, ers.FilterRemove(io.EOF, context.DeadlineExceeded)(err))
		check.NotError(t, ers.FilterRemove(context.Canceled)(err))

		check.NotError(t, ers.FilterRemove(ErrCountMeOut)(err))
		check.ErrorIs(t, err, ErrCountMeOut)
	})
	t.Run("ConverterOK", func(t *testing.T) {
		ctx := testt.Context(t)
		tfrm := ConverterOK(func(in string) (string, bool) { return in, true })
		out, err := tfrm(ctx, "hello")
		check.Equal(t, out, "hello")
		check.NotError(t, err)
		tfrm = ConverterOK(func(in string) (string, bool) { return in, false })
		out, err = tfrm(ctx, "bye")
		check.Error(t, err)
		check.ErrorIs(t, err, ErrIteratorSkip)
		check.Equal(t, out, "bye")
	})
	t.Run("Block", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			mpf := Transform[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), nil
			})

			out, err := mpf.Block()(42)
			check.Equal(t, "42", out)
			check.NotError(t, err)
			check.Equal(t, count, 1)
		})
		t.Run("Error", func(t *testing.T) {
			count := 0
			mpf := Transform[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), io.EOF
			})

			out, err := mpf.Block()(42)
			check.Equal(t, "42", out)
			check.Error(t, err)
			check.ErrorIs(t, err, io.EOF)
			check.Equal(t, count, 1)
		})
	})
	t.Run("BlockCheck", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			mpf := Transform[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), nil
			})

			out, ok := mpf.BlockCheck()(42)
			check.Equal(t, "42", out)
			check.True(t, ok)
			check.Equal(t, count, 1)
		})
		t.Run("Error", func(t *testing.T) {
			count := 0
			mpf := Transform[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), io.EOF
			})

			out, ok := mpf.BlockCheck()(42)
			check.Equal(t, "", out)
			check.True(t, !ok)
			check.Equal(t, count, 1)
		})
	})
	t.Run("Lock", func(t *testing.T) {
		count := &atomic.Int64{}

		mpf := Transform[int, string](func(ctx context.Context, in int) (string, error) {
			check.Equal(t, in, 42)
			count.Add(1)
			return fmt.Sprint(in), nil
		})
		mpf = mpf.Lock()
		// tempt the race detector
		wg := &WaitGroup{}
		ctx := testt.Context(t)
		wg.DoTimes(ctx, 128, func(ctx context.Context) {
			out, err := mpf(ctx, 42)
			check.Equal(t, out, "42")
			check.NotError(t, err)
		})

		check.Equal(t, count.Load(), 128)
	})
	t.Run("Pipe", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			var root error
			tfm := Transform[int, string](func(ctx context.Context, in int) (string, error) { return fmt.Sprint(in), root })
			proc, prod := tfm.Pipe()
			ctx := testt.Context(t)
			for i := 0; i < 100; i++ {
				assert.NotError(t, proc(ctx, i))
				assert.Equal(t, fmt.Sprint(i), ft.Must(prod(ctx)))
			}
			root = io.EOF
			for i := 0; i < 100; i++ {
				assert.NotError(t, proc(ctx, i))
				out, err := prod(ctx)
				assert.ErrorIs(t, err, io.EOF)
				assert.Equal(t, fmt.Sprint(i), out)
			}

		})
		t.Run("Parallel", func(t *testing.T) {
			var root error
			tfm := Transform[int, string](func(ctx context.Context, in int) (string, error) { return fmt.Sprint(in), root })
			proc, prod := tfm.Pipe()
			ctx := testt.Context(t)
			wg := &WaitGroup{}
			wg.DoTimes(ctx, 100, func(ctx context.Context) {
				assert.NotError(t, proc(ctx, 42))
				assert.Equal(t, fmt.Sprint(42), ft.Must(prod(ctx)))
			})
		})
	})
	t.Run("Worker", func(t *testing.T) {
		var tfmroot error
		var prodroot error
		var procroot error
		var tfmcount int
		var prodcount int
		var proccount int
		reset := func() { tfmcount, prodcount, proccount = 0, 0, 0; prodroot, procroot = nil, nil }
		prod := Producer[int](func(ctx context.Context) (int, error) { prodcount++; return 42, prodroot })
		proc := Processor[string](func(ctx context.Context, in string) error { proccount++; check.Equal(t, in, "42"); return procroot })
		tfm := Transform[int, string](func(ctx context.Context, in int) (string, error) { tfmcount++; return fmt.Sprint(in), tfmroot })
		worker := tfm.Worker(prod, proc)
		ctx := testt.Context(t)

		t.Run("HappyPath", func(t *testing.T) {
			defer reset()
			check.True(t, prodcount == proccount && prodcount == tfmcount && tfmcount == 0)
			check.NotError(t, worker(ctx))
			check.True(t, prodcount == proccount && prodcount == tfmcount && tfmcount == 1)
		})

		t.Run("ProdError", func(t *testing.T) {
			defer reset()
			prodroot = io.EOF
			check.ErrorIs(t, worker(ctx), io.EOF)
			check.True(t, proccount == tfmcount && tfmcount == 0)
			check.Equal(t, prodcount, 1)
			testt.Log(t, prodcount, proccount, tfmcount)
		})
		t.Run("ProcError", func(t *testing.T) {
			defer reset()

			procroot = io.EOF
			check.ErrorIs(t, worker(ctx), io.EOF)
			check.True(t, prodcount == proccount && prodcount == tfmcount && tfmcount == 1)
			testt.Log(t, prodcount, proccount, tfmcount)
		})
		t.Run("MapFails", func(t *testing.T) {
			defer reset()
			tfmroot = io.EOF
			check.ErrorIs(t, worker(ctx), io.EOF)
			check.True(t, prodcount == tfmcount && tfmcount == 1)
			check.Equal(t, proccount, 0)
			testt.Log(t, prodcount, proccount, tfmcount)
		})

	})

}
