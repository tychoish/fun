package fun

import (
	"context"
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
		t.Fatal("collected values are not equal")
	}
	for idx := range in {
		if in[idx] != out[idx] {
			t.Error("mismatch values at index", idx)
		}
	}
}

func TestIteratorImplementations(t *testing.T) {
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
			for name, builder := range map[string]func() Iterator[string]{
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
			} {
				t.Run(name, func(t *testing.T) {
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
							err := ForEach(ctx, builder(), func(str string) error {
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
							ch := Channel(ctx, iter)
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
							vals, err := Collect(ctx, iter)
							if err != nil {
								t.Fatal(err)
							}
							slicesAreEqual(t, elems, vals)
						})
						t.Run("Map", func(t *testing.T) {
							iter := builder()
							out := Map(ctx, iter,
								func(ctx context.Context, str string) (string, error) {
									return str, nil
								},
							)

							vals, err := Collect(ctx, out)
							if err != nil {
								t.Fatal(err)
							}
							slicesAreEqual(t, elems, vals)
						})
						t.Run("Reduce", func(t *testing.T) {
							iter := builder()
							seen := make(map[string]struct{}, len(elems))
							sum, err := Reduce(ctx, iter,
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
}
