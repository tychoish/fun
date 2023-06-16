package itertool

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

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/risky"
	"github.com/tychoish/fun/testt"
)

func getConstructors[T comparable](t *testing.T) []FixtureIteratorConstuctors[T] {
	return []FixtureIteratorConstuctors[T]{
		{
			Name: "SlicifyIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.SliceIterator(elems)
			},
		},
		{
			Name: "Slice",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.SliceIterator(elems)
			},
		},
		{
			Name: "VariadicIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.VariadicIterator(elems...)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return fun.Blocking(vals).Iterator()
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

func TestTools(t *testing.T) {
	t.Parallel()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
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

				output := fun.Blocking(pipe).Iterator().Channel(ctx)
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
					case <-time.After(10 * time.Millisecond):
						break CONSUME
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})
		})
	}
	t.Run("MapWorkerSendingBlocking", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string, 1)
		output := make(chan int)
		catcher := &erc.Collector{}
		wg := &fun.WaitGroup{}
		pipe <- t.Name()

		var mf mapper[string, int] = func(ctx context.Context, in string) (int, error) { return 53, nil }
		mf.Safe().Processor(fun.Blocking(output).Send().Write, fun.WorkerGroupOptions{ErrorObserver: catcher.Add, ErrorResolver: catcher.Resolve}).
			ReadAll(fun.Blocking(pipe).Receive().Producer()).
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

func TestParallelForEach(t *testing.T) {
	ctx := testt.Context(t)

	t.Run("Basic", func(t *testing.T) {
		for i := int64(-1); i <= 12; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				elems := makeIntSlice(200)

				seen := &adt.Map[int, none]{}

				err := Process(ctx,
					fun.SliceIterator(elems),
					func(ctx context.Context, in int) error {
						abs := int64(math.Abs(float64(i)))

						jitter := time.Duration(rand.Int63n(1 + abs*int64(time.Millisecond)))
						time.Sleep(time.Millisecond + jitter)
						seen.Ensure(in)
						return nil
					},
					fun.NumWorkers(int(i)),
				)
				if err != nil {
					t.Fatal(err)
				}
				out := risky.Force(seen.Keys().Slice(ctx))

				testt.Log(t, "output", out)
				testt.Log(t, "input", elems)

				if len(out) != len(elems) {
					t.Error("unequal length slices")
				}

				matches := 0
				for idx := range out {
					if out[idx] == elems[idx] {
						matches++
					}
				}
				if i >= 2 && matches == len(out) {
					t.Error("should not all match", matches, len(out))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		seen := &adt.Map[int, none]{}
		count := &atomic.Int64{}
		errCount := &atomic.Int64{}
		ec := &erc.Collector{}
		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(200)),
			func(ctx context.Context, in int) error {
				count.Add(1)
				seen.Ensure(in)
				runtime.Gosched()
				if in >= 100 {
					errCount.Add(1)
					panic("error")
				}
				return nil
			},
			fun.NumWorkers(3),
			fun.ContinueOnPanic(),
			fun.ErrorCollector(ec),
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}

		if errCount.Load() != 100 {
			t.Error(errCount.Load())
		}
		if count.Load() != 200 {
			t.Error(count.Load())
		}
		check.Equal(t, 200, fun.CountWraps(err))
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seen := &adt.Map[int, none]{}

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				if in == 8 {
					// make sure something else
					// has a chance to run before
					// the event.
					runtime.Gosched()
					panic("gotcha")
				} else {
					seen.Ensure(in)
				}

				<-ctx.Done()
				return nil
			},
			fun.NumWorkers(10),
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		if seen.Len() < 1 {
			t.Error("should have only seen one", seen.Len())
		}
		errs := fun.Unwind(err)
		if len(errs) != 2 {
			// panic + expected
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				if in == 4 {
					cancel()
					panic("gotcha")
				}
				return nil
			},
			fun.NumWorkers(8),
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
	})
	t.Run("CollectAllContinuedErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := &atomic.Int64{}

		err := ParallelForEach(ctx,
			fun.SliceIterator(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				count.Add(1)
				return fmt.Errorf("errored=%d", in)
			},
			fun.NumWorkers(4),
			fun.ContinueOnError(),
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
		testt.Log(t, err)
		check.Equal(t, 10, count.Load())

		errs := fun.Unwind(err)
		if len(errs) != 10 {
			t.Error(len(errs), "!= 10", errs)
		}

	})
	t.Run("CollectAllErrors/Double", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(100)),
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			fun.NumWorkers(2),
		)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := fun.Unwind(err)
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > 2 {
			t.Error(len(errs))
		}
	})
	t.Run("CollectAllErrors/Cores", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(100)),
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			fun.WorkerPerCPU(),
		)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := fun.Unwind(err)
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

			err := Process(ctx,
				fun.SliceIterator(makeIntSlice(2)),
				func(ctx context.Context, in int) error {
					return context.Canceled
				},
			)
			check.NotError(t, err)
			if err != nil {
				t.Error("should have skipped all errors", err)
			}
		})
		t.Run("WithErrors", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := Process(ctx,
				fun.SliceIterator(makeIntSlice(2)),
				func(ctx context.Context, in int) error {
					return context.Canceled
				},
				fun.IncludeContextErrors(),
			)
			check.Error(t, err)
			check.ErrorIs(t, err, context.Canceled)
		})
	})
}

func TestWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := &atomic.Int64{}
	err := Worker(ctx, fun.SliceIterator([]fun.Operation{
		func(context.Context) { count.Add(1) },
		func(context.Context) { count.Add(1) },
		func(context.Context) { count.Add(1) },
	}))
	assert.NotError(t, err)
	assert.Equal(t, count.Load(), 3)
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

					builder := func() *fun.Iterator[T] { return baseBuilder(elems) }

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
							check.ErrorIs(t, err, fun.ErrRecoveredPanic)
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
								fun.NumWorkers(2),
								fun.ContinueOnError(),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error")
							}

							assert.ErrorIs(t, err, fun.ErrRecoveredPanic)

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
								fun.ContinueOnError(),
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
								fun.ContinueOnPanic(),
								fun.NumWorkers(1),
							).Slice(ctx)

							testt.Log(t, elems)

							if err == nil {
								t.Error("expected error", err)
							}
							check.ErrorIs(t, err, fun.ErrRecoveredPanic)
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
								fun.NumWorkers(1),
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
								fun.NumWorkers(4),
								fun.ContinueOnError(),
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

					builder := func() *fun.Iterator[string] { return (baseBuilder(elems)) }
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
							fun.MergeIterators(builder(), builder(), builder()),
							func(ctx context.Context, str string) (string, error) {
								for _, c := range []string{"a", "e", "i", "o", "u"} {
									str = strings.ReplaceAll(str, c, "")
								}
								return strings.TrimSpace(str), nil
							},
							fun.NumWorkers(4),
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
							out := Generate(
								func(ctx context.Context) (string, error) {
									count.Add(1)
									if int(count.Load()) > len(inputs) {
										return "", io.EOF
									}
									return inputs[rand.Intn(511)], nil
								},
							)
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
							out := Generate(
								func(ctx context.Context) (string, error) {
									count.Add(1)
									if int(count.Load()) > len(inputs) {
										return "", io.EOF
									}
									return inputs[rand.Intn(511)], nil
								},
								fun.NumWorkers(4),
							)
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

							out := Generate(
								func(ctx context.Context) (string, error) {
									panic("foo")
								},
							)
							if out.Next(ctx) {
								t.Fatal("should not iterate when panic")
							}

							err := out.Close()
							assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
							assert.Substring(t, err.Error(), "foo")
						})
						t.Run("ContinueOnPanic", func(t *testing.T) {
							ctx := testt.Context(t)

							count := 0
							out := Generate(
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
								fun.ContinueOnPanic(),
							)
							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Log(err, output)
								t.Error(l)
							}
							if err == nil {
								t.Fatal("should have errored")
							}
							assert.Substring(t, err.Error(), "foo")
							assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
						})
						t.Run("ArbitraryErrorAborts", func(t *testing.T) {
							ctx := testt.Context(t)

							count := 0
							out := Generate(
								func(ctx context.Context) (string, error) {
									count++
									if count == 4 {
										return "", errors.New("beep")
									}
									return "foo", nil
								},
							)
							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Error(l)
							}
							if err == nil {
								t.Fatal("should have errored")
							}

							if err.Error() != "beep" {
								t.Log(len(fun.Unwind(err)))
								t.Fatalf("unexpected panic %q", err.Error())
							}
						})
						t.Run("ContinueOnError", func(t *testing.T) {
							ctx := testt.Context(t)
							count := 0
							out := Generate(
								func(ctx context.Context) (string, error) {
									count++
									if count == 3 {
										return "", errors.New("beep")
									}
									if count == 5 {
										return "", io.EOF
									}
									return "foo", nil
								},
								fun.ContinueOnError(),
							)
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
						sum, err := Reduce(ctx, iter,
							func(in string, val int) (int, error) {
								seen[in] = struct{}{}
								val += len(in)
								return val, nil
							},
							0,
						)
						if err != nil {
							t.Fatal(err)
						}
						CheckSeenMap(t, elems, seen)
						seenSum := 0
						for str := range seen {
							seenSum += len(str)
						}
						if seenSum != sum {
							t.Errorf("incorrect seen %d, reduced %d", seenSum, sum)
						}
					})
					t.Run("ReduceError", func(t *testing.T) {
						ctx := testt.Context(t)

						iter := builder()

						seen := map[string]none{}

						total, err := Reduce(ctx, iter,
							func(in string, val int) (int, error) {
								seen[in] = none{}
								val++
								if len(seen) == 2 {
									return val, errors.New("boop")
								}
								return val, nil
							},
							0,
						)

						if err == nil {
							t.Fatal("expected error")
						}

						if e := err.Error(); e != "boop" {
							t.Error("unexpected error:", e)
						}
						if l := len(seen); l != 2 {
							t.Error("seen", l, seen)
						}
						if total != 2 {
							t.Error("unexpected total value", total)
						}
					})

				})
			}
		})
	}
}
