package itertool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

type none struct{}

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

func GenerateRandomStringSlice(size int) []string {
	out := make([]string, size)
	for idx := range out {
		out[idx] = fmt.Sprint("value=", idx)
	}
	return out
}

type FixtureData[T any] struct {
	Name     string
	Elements []T
}

type FixtureIteratorConstuctors[T any] struct {
	Name        string
	Constructor func([]T) *fun.Iterator[T]
}

type FixtureIteratorFilter[T any] struct {
	Name   string
	Filter func(*fun.Iterator[T]) *fun.Iterator[T]
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
								Options{ContinueOnError: false},
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
								Options{
									ContinueOnError: true,
									ContinueOnPanic: false,
									NumWorkers:      2,
								},
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
								Options{
									ContinueOnError: true,
								},
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
								Options{
									ContinueOnPanic: true,
									NumWorkers:      1,
								},
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
								Options{
									NumWorkers:      1,
									ContinueOnError: false,
								},
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
								Options{
									NumWorkers:      4,
									ContinueOnError: true,
								},
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
							Options{},
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
							Options{
								NumWorkers: 4,
							},
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
								Options{},
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
								Options{
									NumWorkers: 4,
								},
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
								Options{},
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
								Options{
									ContinueOnPanic: true,
								},
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
								Options{},
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
								Options{
									ContinueOnError: true,
								},
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

func makeIntSlice(size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return out
}
