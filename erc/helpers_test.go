package erc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/tychoish/fun"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

func TestCollections(t *testing.T) {
	t.Run("Merge", func(t *testing.T) {
		e1 := &errorTest{val: 100}
		e2 := &errorTest{val: 200}

		err := Merge(e1, e2)

		if err == nil {
			t.Fatal("should be an error")
		}
		if !errors.Is(err, e1) {
			t.Error("shold be er1", err, e1)
		}

		if !errors.Is(err, e2) {
			t.Error("shold be er2", err, e2)
		}
		cp := &errorTest{}
		if !errors.As(err, &cp) {
			t.Error("should err as", err, cp)
		}
		if cp.val != e1.val {
			t.Error(cp.val)
		}
	})
	t.Run("Collapse", func(t *testing.T) {
		t.Run("From", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				ec := &Collector{}
				CollapseFrom(ec, nil)
				if err := ec.Resolve(); err != nil {
					t.Error("should be nil", err)
				}
				ec = &Collector{}
				CollapseFrom(ec, []error{})
				if err := ec.Resolve(); err != nil {
					t.Error("should be nil", err)
				}
			})
			t.Run("One", func(t *testing.T) {
				ec := &Collector{}
				e := errors.New("fourty-two")
				CollapseFrom(ec, []error{e})
				err := ec.Resolve()
				if !errors.Is(err, e) {
					t.Error(err, e)
				}
			})
			t.Run("Many", func(t *testing.T) {
				ec := &Collector{}
				e0 := errors.New("fourty-two")
				e1 := errors.New("fourty-three")
				CollapseFrom(ec, []error{e0, e1})
				err := ec.Resolve()
				if !errors.Is(err, e1) {
					t.Error(err, e1)
				}
				if !errors.Is(err, e0) {
					t.Error(err, e0)
				}
				errs := Unwind(err)
				if len(errs) != 2 {
					t.Error(errs)
				}
			})
		})
		t.Run("Into", func(t *testing.T) {
			t.Run("From", func(t *testing.T) {
				t.Run("Empty", func(t *testing.T) {
					ec := &Collector{}
					CollapseInto(ec, nil)
					if err := ec.Resolve(); err != nil {
						t.Error("should be nil", err)
					}
					ec = &Collector{}
					CollapseInto(ec)
					if err := ec.Resolve(); err != nil {
						t.Error("should be nil", err)
					}
				})
				t.Run("One", func(t *testing.T) {
					ec := &Collector{}
					e := errors.New("fourty-two")
					CollapseInto(ec, e)
					err := ec.Resolve()
					if !errors.Is(err, e) {
						t.Error(err, e)
					}
				})
				t.Run("Many", func(t *testing.T) {
					ec := &Collector{}
					e0 := errors.New("fourty-two")
					e1 := errors.New("fourty-three")
					CollapseInto(ec, e0, e1)
					err := ec.Resolve()
					if !errors.Is(err, e1) {
						t.Error(err, e1)
					}
					if !errors.Is(err, e0) {
						t.Error(err, e0)
					}
					errs := Unwind(err)
					if len(errs) != 2 {
						t.Error(errs)
					}
				})
			})
		})
		t.Run("Base", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				if err := Collapse(); err != nil {
					t.Error("should be nil", err)
				}
			})
			t.Run("One", func(t *testing.T) {
				e := errors.New("fourty-two")
				err := Collapse(e)
				if !errors.Is(err, e) {
					t.Error(err, e)
				}
			})
			t.Run("Many", func(t *testing.T) {
				e0 := errors.New("fourty-two")
				e1 := errors.New("fourty-three")
				err := Collapse(e0, e1)
				if !errors.Is(err, e1) {
					t.Error(err, e1)
				}
				if !errors.Is(err, e0) {
					t.Error(err, e0)
				}
				errs := Unwind(err)
				if len(errs) != 2 {
					t.Error(errs)
				}
			})
		})
	})
	t.Run("Stream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("Base", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				ch := make(chan error)
				close(ch)
				err := Stream(ctx, ch)
				if err != nil {
					t.Error("nil expected", err)
				}
			})

			t.Run("One", func(t *testing.T) {
				ch := getPopulatedErrChan(1)
				close(ch)
				err := Stream(ctx, ch)
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 1 {
					t.Error(errs)
				}
			})
			t.Run("Many", func(t *testing.T) {
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				err := Stream(ctx, ch)
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(errs)
				}
			})
		})
		t.Run("Into", func(t *testing.T) {
			t.Run("Many", func(t *testing.T) {
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				ec := &Collector{}
				StreamInto(ctx, ec, ch)
				err := ec.Resolve()
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(errs)
				}
			})
			t.Run("CanceledWithContent", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				ec := &Collector{}
				StreamInto(ctx, ec, ch)
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})
			t.Run("Canceled", func(t *testing.T) {
				// the real test here is that it
				// doesn't deadlock
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch := getPopulatedErrChan(0)
				ec := &Collector{}
				StreamInto(ctx, ec, ch)
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})
		})
		t.Run("Process", func(t *testing.T) {
			t.Run("Many", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				fun.Invariant(ctx.Err() == nil, ctx.Err())
				wg := &sync.WaitGroup{}
				ec := &Collector{}
				StreamProcess(ctx, wg, ec, ch)
				wg.Wait()
				err := ec.Resolve()
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(len(errs), errs, err)
				}
			})
			t.Run("Canceled", func(t *testing.T) {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				wg := &sync.WaitGroup{}
				ec := &Collector{}
				StreamProcess(ctx, wg, ec, ch)
				wg.Wait()
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})

		})
	})
}

func getPopulatedErrChan(size int) chan error {
	out := make(chan error, size)

	for i := 0; i < size; i++ {
		out <- fmt.Errorf("mock err %d", i)
	}
	return out
}
