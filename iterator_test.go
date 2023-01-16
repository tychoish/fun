package fun

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func checkSeenMap(t *testing.T, elems []string, seen map[string]struct{}) {
	t.Helper()
	if len(seen) != len(elems) {
		t.Fatal("all elements not iterated", len(seen), "vs", len(elems))
	}
	for idx, val := range elems {
		if _, ok := seen[val]; !ok {
			t.Error("element a not observed", idx, val)
		}
	}
}

func slicesAreEqual[T comparable](t *testing.T, in []T, out []T) {
	t.Helper()
	if len(in) != len(out) {
		t.Fatalf("collected values are not equal, in=%d, out=%d", len(in), len(out))
	}
	for idx := range in {
		if in[idx] != out[idx] {
			t.Error("mismatch values at index", idx)
		}
	}
}

func TestIteratorAlgoInts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	elems := func() []int {
		e := make([]int, 100)
		for idx := range e {
			e[idx] = idx
		}
		return e
	}
	for name, baseBuilder := range map[string]func() Iterator[int]{
		"SliceIterator": func() Iterator[int] {
			return SliceIterator(elems())
		},
		"ChannelIterator": func() Iterator[int] {
			e := elems()
			vals := make(chan int, len(e))
			for idx := range e {
				vals <- e[idx]
			}
			close(vals)
			return ChannelIterator(vals)
		},
		"SetIterator": func() Iterator[int] {
			e := elems()
			set := MakeSet[int](len(e))
			for idx := range e {
				set.Add(e[idx])
			}

			return set.Iterator(ctx)
		},
		"OrderedSetIterator": func() Iterator[int] {
			e := elems()
			set := MakeOrderedSet[int](len(e))
			for idx := range e {
				set.Add(e[idx])
			}

			return set.Iterator(ctx)
		},
	} {
		t.Run(name, func(t *testing.T) {
			for wrapperName, wrapper := range map[string]func(Iterator[int]) Iterator[int]{
				"UnSynchronized": func(in Iterator[int]) Iterator[int] { return in },
				"Synchronized": func(in Iterator[int]) Iterator[int] {
					return MakeSynchronizedIterator(in)
				},
			} {
				t.Run(wrapperName, func(t *testing.T) {
					t.Run("Filter", func(t *testing.T) {
						t.Run("Evens", func(t *testing.T) {
							outIter := IteratorFilter(
								ctx,
								wrapper(baseBuilder()),
								func(ctx context.Context, input int) (int, bool, error) {
									if input%2 == 0 {
										return input, true, nil
									}
									return 0, false, nil
								},
							)
							out, err := IteratorCollect(ctx, outIter)
							if err != nil {
								t.Fatal(err)
							}
							if len(out) != 50 { // half 100
								t.Fatal("output had", len(out))
							}
						})
						t.Run("PanicSafety", func(t *testing.T) {
							outIter := IteratorFilter(
								ctx,
								wrapper(baseBuilder()),
								func(ctx context.Context, input int) (int, bool, error) {
									panic("whoop")
								},
							)
							out, err := IteratorCollect(ctx, outIter)
							if err == nil {
								t.Fatal("expectged error")
							}
							if err.Error() != "panic: whoop" {
								t.Fatal(err)
							}
							if len(out) != 0 {
								t.Fatal("unexpected output", out)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							outIter := IteratorFilter(
								ctx,
								wrapper(baseBuilder()),
								func(ctx context.Context, input int) (int, bool, error) {
									if input < 10 {
										return input, true, nil
									}
									// include value is ignored for error cases
									return 0, true, errors.New("abort")
								},
							)
							out, err := IteratorCollect(ctx, outIter)
							if err == nil {
								t.Fatal("expectged error", err)
							}
							if err.Error() != "abort" {
								t.Fatal(err)
							}
							// the default set uses a map, which has randomized
							// iteration order. Skip this implementation.
							if name != "SetIterator" {
								if len(out) != 10 {
									t.Fatal("unexpected output", out)
								}
							}
						})
					})
					t.Run("ForEach", func(t *testing.T) {
						t.Run("PanicSafety", func(t *testing.T) {
							err := IteratorForEach(
								ctx,
								wrapper(baseBuilder()),
								func(ctx context.Context, input int) error {
									panic("whoop")
								},
							)

							if err == nil {
								t.Fatal("expectged error")
							}
							if err.Error() != "panic: whoop" {
								t.Fatal(err)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							var count int
							seen := NewSet[int]()
							err := IteratorForEach(
								ctx,
								wrapper(baseBuilder()),
								func(ctx context.Context, in int) error {
									count++
									seen.Add(in)
									if count >= 60 {
										return errors.New("whoop")
									}

									return nil
								})

							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "whoop" {
								t.Error(err)
							}
							if count != 60 {
								t.Error("count should have been 60, but was", count)
							}
							if count != seen.Len() {
								t.Error("impossible", count, seen.Len())
							}
						})
					})
					t.Run("Map", func(t *testing.T) {
						t.Run("PanicSafety", func(t *testing.T) {
							out, err := IteratorCollect(ctx,
								IteratorMap(ctx,
									IteratorMapOptions{ContinueOnError: false},
									wrapper(baseBuilder()),
									func(ctx context.Context, input int) (int, error) {
										panic("whoop")
									},
								),
							)
							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "panic: whoop" {
								t.Fatal(err)
							}
							if len(out) != 0 {
								t.Fatal("unexpected output", out)
							}
						})
						t.Run("ErrorDoesNotAbort", func(t *testing.T) {
							out, err := IteratorCollect(ctx,
								IteratorMap(ctx,
									IteratorMapOptions{
										ContinueOnError: true,
									},
									wrapper(baseBuilder()),
									func(ctx context.Context, input int) (int, error) {
										if input == 42 {
											return 0, errors.New("whoop")
										}
										return input, nil
									},
								),
							)
							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "whoop" {
								t.Fatal(err)
							}
							if len(out) != 99 {
								t.Fatal("unexpected output", len(out), "->", out)
							}
						})
						t.Run("PanicDoesNotAbort", func(t *testing.T) {
							out, err := IteratorCollect(ctx,
								IteratorMap(ctx,
									IteratorMapOptions{
										ContinueOnPanic: true,
										NumWorkers:      1,
									},
									wrapper(baseBuilder()),
									func(ctx context.Context, input int) (int, error) {
										if input == 42 {
											panic("whoop")
										}
										return input, nil
									},
								),
							)
							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "panic: whoop" {
								t.Fatal(err)
							}
							if len(out) != 99 {
								t.Fatal("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ParallelErrorDoesNotAbort", func(t *testing.T) {
							expectedErr := errors.New("whoop")
							out, err := IteratorCollect(ctx,
								IteratorMap(ctx,
									IteratorMapOptions{
										NumWorkers:      4,
										ContinueOnError: true,
									},
									wrapper(baseBuilder()),
									func(ctx context.Context, input int) (int, error) {
										if input == 42 {
											return 0, expectedErr
										}
										return input, nil
									},
								),
							)
							if err == nil {
								t.Error("expected error")
							}
							if !errors.Is(err, expectedErr) {
								t.Error(err)
							}
							if len(out) != 99 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							out, err := IteratorCollect(ctx,
								IteratorMap(ctx,
									IteratorMapOptions{
										NumWorkers:      1,
										ContinueOnError: false,
									},
									wrapper(baseBuilder()),
									func(ctx context.Context, input int) (int, error) {
										if input >= 10 {
											return 0, errors.New("whoop")
										}
										return input, nil
									},
								),
							)
							if err == nil {
								t.Fatal("expected error")
							}
							if err.Error() != "whoop" {
								t.Fatal(err)
							}
							// we should abort, but there's some asynchronicity.
							if len(out) > 10 {
								t.Fatal("unexpected output", len(out), "->", out)
							}
						})
					})
				})
			}
		})
	}
}

func TestIteratorImplementations(t *testing.T) {
	t.Parallel()
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
			for name, baseBuilder := range map[string]func() Iterator[string]{
				"SliceIterator": func() Iterator[string] {
					return SliceIterator(elems)
				},
				"ChannelIterator": func() Iterator[string] {
					vals := make(chan string, len(elems))
					for idx := range elems {
						vals <- elems[idx]
					}
					close(vals)
					return ChannelIterator(vals)
				},
				"SetIterator": func() Iterator[string] {
					set := MakeSet[string](len(elems))
					for idx := range elems {
						set.Add(elems[idx])
					}

					return set.Iterator(ctx)
				},
				"OrderedSetIterator": func() Iterator[string] {
					set := MakeOrderedSet[string](len(elems))
					for idx := range elems {
						set.Add(elems[idx])
					}

					return set.Iterator(ctx)
				},
			} {
				t.Run(name, func(t *testing.T) {
					elems := elems
					t.Parallel()

					for filterName, filter := range map[string]func(Iterator[string]) Iterator[string]{
						"UnSynchronized": func(in Iterator[string]) Iterator[string] { return in },
						"Synchronized": func(in Iterator[string]) Iterator[string] {
							return MakeSynchronizedIterator(in)
						},
					} {
						t.Run(filterName, func(t *testing.T) {
							builder := func() Iterator[string] { return filter(baseBuilder()) }
							t.Run("Single", func(t *testing.T) {
								seen := make(map[string]struct{}, len(elems))
								iter := builder()

								for iter.Next(ctx) {
									seen[iter.Value()] = struct{}{}
								}

								checkSeenMap(t, elems, seen)
								if err := iter.Close(ctx); err != nil {
									t.Fatal(err)
								}
							})
							t.Run("Merged", func(t *testing.T) {
								iter := MergeIterators(ctx, builder(), builder(), builder())
								seen := make(map[string]struct{}, len(elems))
								var count int
								for iter.Next(ctx) {
									count++
									seen[iter.Value()] = struct{}{}
								}
								if count != 3*len(elems) {
									t.Fatal("did not iterate enough", count, 3*len(elems))
								}

								checkSeenMap(t, elems, seen)

								if err := iter.Close(ctx); err != nil {
									t.Fatal(err)
								}
							})
							t.Run("Canceled", func(t *testing.T) {
								iter := builder()
								ctx, cancel := context.WithCancel(ctx)
								cancel()
								var count int

								for iter.Next(ctx) {
									count++
								}
								if count != 0 {
									t.Fatal("should not have iterated", count)
								}
							})
							t.Run("Algo", func(t *testing.T) {
								t.Run("ForEach", func(t *testing.T) {
									var count int
									seen := make(map[string]struct{}, len(elems))
									err := IteratorForEach(
										ctx,
										builder(),
										func(ctx context.Context, str string) error {
											count++
											seen[str] = struct{}{}
											return nil
										})
									if err != nil {
										t.Fatal(err)
									}

									checkSeenMap(t, elems, seen)
								})
								t.Run("Channel", func(t *testing.T) {
									seen := make(map[string]struct{}, len(elems))
									iter := builder()
									ch := IteratorChannel(ctx, iter)
									for str := range ch {
										seen[str] = struct{}{}
									}
									checkSeenMap(t, elems, seen)
									if err := iter.Close(ctx); err != nil {
										t.Fatal(err)
									}
								})
								t.Run("Collect", func(t *testing.T) {
									iter := builder()
									vals, err := IteratorCollect(ctx, iter)
									if err != nil {
										t.Fatal(err)
									}
									// skip implementation with random order
									if name != "SetIterator" {
										slicesAreEqual(t, elems, vals)
									}
								})
								t.Run("Map", func(t *testing.T) {
									iter := builder()
									out := IteratorMap(
										ctx,
										IteratorMapOptions{},
										iter,
										func(ctx context.Context, str string) (string, error) {
											return str, nil
										},
									)

									vals, err := IteratorCollect(ctx, out)
									if err != nil {
										t.Fatal(err)

									}
									// skip implementation with random order
									if name != "SetIterator" {
										slicesAreEqual(t, elems, vals)
									}
								})
								t.Run("Reduce", func(t *testing.T) {
									iter := builder()
									seen := make(map[string]struct{}, len(elems))
									sum, err := IteratorReduce(ctx, iter,
										func(ctx context.Context, in string, val int) (int, error) {
											seen[in] = struct{}{}
											val += len(in)
											return val, nil
										},
										0,
									)
									if err != nil {
										t.Fatal(err)
									}
									checkSeenMap(t, elems, seen)
									seenSum := 0
									for str := range seen {
										seenSum += len(str)
									}
									if seenSum != sum {
										t.Errorf("incorrect seen %d, reduced %d", seenSum, sum)
									}
								})
							})
						})
					}
				})
			}
		})
	}
}
