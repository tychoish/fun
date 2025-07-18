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
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

func Map[T any, O any](it *Stream[T], mpf Converter[T, O], optp ...OptionProvider[*WorkerGroupConf]) *Stream[O] {
	return mpf.Parallel(it, optp...)
}

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

type FixtureStreamConstructors[T any] struct {
	Name        string
	Constructor func([]T) *Stream[T]
}

type FixtureStreamFilter[T any] struct {
	Name   string
	Filter func(*Stream[T]) *Stream[T]
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
	for i := 0; i < 4; i++ {
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

				output := Blocking(pipe).Stream().Channel(ctx)
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
					case <-time.After(100 * time.Millisecond):
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

		var mf Converter[string, int] = func(_ context.Context, _ string) (int, error) { return 53, nil }
		mf.WithRecover().mapPullProcess(Blocking(output).Send().Write, &WorkerGroupConf{}).
			ReadAll(Blocking(pipe).Receive().Stream()).
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

func getConstructors[T comparable]() []FixtureStreamConstructors[T] {
	return []FixtureStreamConstructors[T]{
		{
			Name: "Slice",
			Constructor: func(elems []T) *Stream[T] {
				return SliceStream(elems)
			},
		},
		{
			Name: "VariadicStream",
			Constructor: func(elems []T) *Stream[T] {
				return VariadicStream(elems...)
			},
		},
		{
			Name: "ChannelStream",
			Constructor: func(elems []T) *Stream[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return Blocking(vals).Stream()
			},
		},
	}

}

func TestStreamImplementations(t *testing.T) {
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
		RunStreamImplementationTests(t, elems, getConstructors[string]())
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunStreamStringAlgoTests(t, elems, getConstructors[string]())
	})
}

func TestStreamAlgoInts(t *testing.T) {
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
		RunStreamImplementationTests(t, elems, getConstructors[int]())
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunStreamIntegerAlgoTests(t, elems, getConstructors[int]())
	})
}

func TestParallelForEach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Basic", func(t *testing.T) {
		for i := int64(-1); i <= 12; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				i := i
				t.Parallel()
				elems := makeIntSlice(200)
				seen := &atomic.Int64{}
				count := &atomic.Int64{}

				err := SliceStream(elems).Parallel(
					func(ctx context.Context, in int) error {
						abs := int64(math.Abs(float64(i)))

						count.Add(1)
						time.Sleep(time.Duration(rand.Int63n(2 + abs*int64(time.Millisecond))))

						if err := ctx.Err(); err != nil {
							return err
						}

						seen.Add(int64(in))
						return nil
					},
					WorkerGroupConfNumWorkers(int(i))).Run(ctx)

				check.NotError(t, err)

				check.Equal(t, int(seen.Load()), sum(elems))

				if int(count.Load()) != len(elems) {
					t.Error("unequal length slices", count.Load(), len(elems))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		count := &atomic.Int64{}
		errCount := &atomic.Int64{}
		err := SliceStream(makeIntSlice(200)).Parallel(
			func(_ context.Context, in int) error {
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
		).Run(ctx)
		if err == nil {
			t.Error("should have errored", err)
		}
		if errCount.Load() != 100 {
			t.Error(errCount.Load())
		}
		if count.Load() != 200 {
			t.Error(count.Load())
		}
		// panic recovery errors plus panics themselves
		check.Equal(t, 200, len(ers.Unwind(err)))
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seenCount := &atomic.Int64{}
		paned := &atomic.Bool{}

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					if in == 8 {
						paned.Store(true)
						// make sure something else
						// has a chance to run before
						// the event.
						runtime.Gosched()
						panic("gotcha")
					}

					seenCount.Add(1)
					return nil
				},
				WorkerGroupConfNumWorkers(4),
			).Run(ctx)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		check.True(t, paned.Load())
		if seenCount.Load() != 9 {
			t.Error("should have seen", 9, "saw", seenCount.Load())
		}
		errs := ers.Unwind(err)
		if len(errs) != 2 {
			// panic + expected
			t.Log(errs)
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					if in == 4 {
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

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					count.Add(1)
					return fmt.Errorf("errored=%d", in)
				},
				WorkerGroupConfNumWorkers(4),
				WorkerGroupConfContinueOnError(),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		check.Equal(t, 10, count.Load())

		errs := ers.Unwind(err)
		if len(errs) != 10 {
			t.Log(errs)
			t.Error(len(errs), "!= 10", errs)
		}

	})
	t.Run("CollectAllErrors/Double", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(100)).
			Parallel(
				func(_ context.Context, in int) error {
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
			t.Log("errs", errs)
			t.Error("num-errors", len(errs))
		}
	})
	t.Run("CollectAllErrors/Cores", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(100)).Parallel(
			func(_ context.Context, in int) error {
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
			t.Log("num-workers", runtime.NumCPU())
			t.Error("num-errors", len(errs))
		}
	})
	t.Run("IncludeContextErrors", func(t *testing.T) {
		t.Run("SuppressErrorsByDefault", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := SliceStream(makeIntSlice(2)).Parallel(
				func(_ context.Context, _ int) error {
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

			err := SliceStream(makeIntSlice(2)).Parallel(
				func(_ context.Context, _ int) error {
					return context.Canceled
				},
				WorkerGroupConfIncludeContextErrors(),
			).Run(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, context.Canceled)
		})
	})
}

func RunStreamImplementationTests[T comparable](
	t *testing.T,
	elements []FixtureData[T],
	builders []FixtureStreamConstructors[T],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					// name := builder.Name
					t.Parallel()

					builder := func() *Stream[T] { return baseBuilder(elems) }

					t.Run("Single", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

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
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map[T](
								baseBuilder(elems),
								func(_ context.Context, _ T) (T, error) {
									panic("whoop")
								},
							).Slice(ctx)

							check.Error(t, err)
							check.ErrorIs(t, err, ers.ErrRecoveredPanic)

							if len(out) != 0 {
								t.Error("unexpected output", out)
							}
						})
						t.Run("ParallelMeap", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, _ T) (T, error) {
									panic("whoop")
								},
								WorkerGroupConfNumWorkers(2),
								WorkerGroupConfContinueOnError(),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error")
							}

							assert.ErrorIs(t, err, ers.ErrRecoveredPanic)

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

func RunStreamIntegerAlgoTests(
	t *testing.T,
	elements []FixtureData[int],
	builders []FixtureStreamConstructors[int],
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input == elems[2] {
										return 0, errors.New("whoop")
									}
									return input, nil
								},
								WorkerGroupConfContinueOnError(),
							).Slice(ctx)
							if err == nil {
								t.Fatal("expected error", out)
							}
							if err.Error() != "whoop" {
								t.Error(err)
							}

							if len(out) != len(elems)-1 {
								t.Fatal("unexpected output", len(out), "->", out, len(elems)-1)
							}
						})

						t.Run("PanicDoesNotAbort", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input == elems[3] {
										panic("whoops")
									}
									return input, nil
								},
								WorkerGroupConfContinueOnPanic(),
								WorkerGroupConfNumWorkers(1),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error", err)
							}
							check.ErrorIs(t, err, ers.ErrRecoveredPanic)
							if len(out) != len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()
							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
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

func RunStreamStringAlgoTests(
	t *testing.T,
	elements []FixtureData[string],
	builders []FixtureStreamConstructors[string],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					t.Parallel()

					builder := func() *Stream[string] { return (baseBuilder(elems)) }
					t.Run("Channel", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

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
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						vals, err := iter.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}
						check.EqualItems(t, elems, vals)
						if err := iter.Close(); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("Map", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						out := Map[string, string](
							iter,
							func(_ context.Context, str string) (string, error) {
								return str, nil
							},
						)

						vals, err := out.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}

						check.EqualItems(t, elems, vals)
					})
					t.Run("ParallelMap", func(t *testing.T) {
						vals, err := Map(
							MergeStreams(VariadicStream(builder(), builder(), builder())),
							func(_ context.Context, str string) (string, error) {
								for _, c := range []string{"a", "e", "i", "o", "u"} {
									str = strings.ReplaceAll(str, c, "")
								}
								return strings.TrimSpace(str), nil
							},
							WorkerGroupConfNumWorkers(4),
						).Slice(t.Context())

						if err != nil {
							t.Error(len(vals), " >>:", err)
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							inputs := GenerateRandomStringSlice(512)
							count := &atomic.Int32{}
							out := Generator[string](func(_ context.Context) (string, error) {
								count.Add(1)
								if int(count.Load()) > len(inputs) {
									return "", io.EOF
								}
								return inputs[rand.Intn(511)], nil
							}).Parallel().Stream()
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							inputs := GenerateRandomStringSlice(512)
							count := &atomic.Int32{}
							out := Generator[string](func(_ context.Context) (string, error) {
								count.Add(1)
								if int(count.Load()) > len(inputs) {
									return "", io.EOF
								}
								return inputs[rand.Intn(511)], nil
							}).Parallel(WorkerGroupConfNumWorkers(4)).Stream()
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
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()
							out := MakeGenerator(func() (string, error) {
								panic("foo")
							}).Parallel().Stream()

							if out.Next(ctx) {
								t.Fatal("should not iterate when panic")
							}

							err := out.Close()

							assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
							assert.Substring(t, err.Error(), "foo")
						})
						t.Run("ContinueOnPanic", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							count := &atomic.Int64{}
							out := Generator[string](
								func(_ context.Context) (string, error) {
									count.Add(1)

									if count.Load()%3 == 0 {
										panic("foo")
									}

									if count.Load()%5 == 0 {
										return "", ers.ErrCurrentOpAbort
									}
									return fmt.Sprint(count.Load()), nil
								},
							).Parallel(WorkerGroupConfContinueOnPanic()).Stream()
							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Log(err, output)
								t.Error(l)
							}
							if err == nil {
								t.Fatal("should have errored", count.Load())
							}
							assert.Substring(t, err.Error(), "foo")
							assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
						})
						t.Run("ArbitraryErrorAborts", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							count := &atomic.Int64{}
							expectedErr := errors.New("beep")
							out := Generator[string](func(_ context.Context) (string, error) {
								count.Add(1)
								if count.Load() > 5 {
									return "", expectedErr
								}
								return "foo", nil
							}).Parallel().Stream()

							output, err := out.Slice(ctx)
							if l := len(output); l != 5 {
								t.Error(l, output)
							}

							check.Error(t, err)
							t.Log(err, out.Close())

							// because it's parallel we collect both
							if !ers.Is(err, expectedErr) && !ers.Is(err, io.EOF) {
								t.Log(len(ers.Unwind(err)))
								t.Errorf("unexpected panic '%v'", err)
							}
						})
						t.Run("ContinueOnError", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()
							count := &atomic.Int64{}
							expectedErr := errors.New("beep")
							out := Generator[string](func(_ context.Context) (string, error) {
								count.Add(1)
								if count.Load() == 3 {
									return "", expectedErr
								}
								if count.Load() >= 5 {
									return "", io.EOF
								}
								return "foo", nil
							}).Parallel(WorkerGroupConfContinueOnError()).Stream()

							output, err := out.Slice(ctx)
							if l := len(output); l != 3 {
								t.Error(l, output)
							}
							if err == nil {
								t.Error("should have errored")
							}
							// because it's parallel we collect both
							if !ers.Is(err, expectedErr) && !ers.Is(err, io.EOF) {
								t.Errorf("unexpected error '%v'", err)
							}
						})
					})

					t.Run("Reduce", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						seen := make(map[string]struct{}, len(elems))
						sum, err := iter.Reduce(func(in string, value string) (string, error) {
							seen[in] = struct{}{}
							return fmt.Sprint(value, in), nil
						}).Read(ctx)

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
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

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
						).Read(ctx)
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
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						elems := makeIntSlice(32)
						iter := SliceStream(elems)
						count := 0
						sum, err := iter.Reduce(func(_ int, value int) (int, error) {
							count++
							if count == 1 {
								return 42, nil
							}
							return value, ErrStreamContinue
						}).Read(ctx)
						assert.NotError(t, err)
						assert.Equal(t, sum, 42)
						assert.Equal(t, count, 32)
					})
					t.Run("ReduceEarlyExit", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						elems := makeIntSlice(32)
						iter := SliceStream(elems)
						count := 0
						sum, err := iter.Reduce(func(_ int, _ int) (int, error) {
							count++
							if count == 16 {
								return 300, io.EOF
							}
							return 42, nil
						}).Read(ctx)
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

		ec.Add(erc.Join(ers.Error("beep"), context.Canceled))
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
		check.Error(t, erc.NewFilter().Without(io.EOF, context.DeadlineExceeded).Apply(err))
		check.NotError(t, erc.NewFilter().WithoutContext().Apply(err))

		check.NotError(t, erc.NewFilter().Without(ErrCountMeOut).Apply(err))
		check.ErrorIs(t, err, ErrCountMeOut)
	})
	t.Run("ConverterOK", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tfrm := MakeCovnerterOk(func(in string) (string, bool) { return in, true })
		out, err := tfrm(ctx, "hello")
		check.Equal(t, out, "hello")
		check.NotError(t, err)
		tfrm = MakeCovnerterOk(func(in string) (string, bool) { return in, false })
		out, err = tfrm(ctx, "bye")
		check.Error(t, err)
		check.ErrorIs(t, err, ErrStreamContinue)
		check.Equal(t, out, "bye")
	})
	t.Run("Block", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			mpf := Converter[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), nil
			})

			out, err := mpf.Wait(42)
			check.Equal(t, "42", out)
			check.NotError(t, err)
			check.Equal(t, count, 1)
		})
		t.Run("Error", func(t *testing.T) {
			count := 0
			mpf := Converter[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), io.EOF
			})

			out, err := mpf.Wait(42)
			check.Equal(t, "42", out)
			check.Error(t, err)
			check.ErrorIs(t, err, io.EOF)
			check.Equal(t, count, 1)
		})
	})
	t.Run("Lock", func(t *testing.T) {
		count := &atomic.Int64{}

		mpf := Converter[int, string](func(_ context.Context, in int) (string, error) {
			check.Equal(t, in, 42)
			count.Add(1)
			return fmt.Sprint(in), nil
		})
		mpf = mpf.Lock()
		// tempt the race detector
		wg := &WaitGroup{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Group(128, func(ctx context.Context) {
			out, err := mpf(ctx, 42)
			check.Equal(t, out, "42")
			check.NotError(t, err)
		}).Run(ctx)
		wg.Wait(ctx)
		check.Equal(t, count.Load(), 128)
	})
	t.Run("ParallelProccesingInvalidConfig", func(t *testing.T) {
		counter := 0
		out, err := MakeConverter(func(in int) string { counter++; return fmt.Sprint(in) }).Parallel(
			SliceStream([]int{42, 84, 21}),
			WorkerGroupConfWithErrorCollector(nil),
		).Slice(t.Context())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.Zero(t, counter)
		assert.Nil(t, out)
	})
}
