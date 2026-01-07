package stw

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"testing"
)

func TestMath(t *testing.T) {
	t.Run("Abs", func(t *testing.T) {
		if v := Abs(-100); v != 100 {
			t.Error(v)
		}
		if v := Abs(100); v != 100 {
			t.Error(v)
		}
	})
	t.Run("Rounding", func(t *testing.T) {
		t.Run("Smallest", func(t *testing.T) {
			if v := RoundToSmallestMultiple(35, 3); v != 33 {
				t.Error(v)
			}
			if v := RoundToSmallestMultiple(-35, 3); v != -36 {
				t.Error(v)
			}
			if v := RoundToSmallestMultiple(-31, 2); v != -32 {
				t.Error(v)
			}
			if v := RoundToSmallestMultiple(34, 5); v != 30 {
				t.Error(v)
			}
		})
		t.Run("Largest", func(t *testing.T) {
			if v := RoundToLargestMultiple(35, 3); v != 36 {
				t.Error(v)
			}
			if v := RoundToLargestMultiple(-35, 3); v != -33 {
				t.Error(v)
			}
			if v := RoundToLargestMultiple(-31, 2); v != -30 {
				t.Error(v)
			}
			if v := RoundToLargestMultiple(34, 5); v != 35 {
				t.Error(v)
			}
		})
		t.Run("TowardZero", func(t *testing.T) {
			t.Run("Positive", func(t *testing.T) {
				if v := RoundToMultipleTowardZero(32, 3); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(31, 3); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(31, 2); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(31, 5); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(34, 5); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(35, 3); v != 33 {
					t.Error(v)
				}
			})
			t.Run("Negative", func(t *testing.T) {
				if v := RoundToMultipleTowardZero(-32, 3); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-31, 3); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-31, -3); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-31, 2); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(2, -31); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-2, 31); v != 30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-36, 5); v != -35 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-34, 5); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-35, 3); v != -33 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(35, -3); v != 33 {
					t.Error(v)
				}
				if v := RoundToMultipleTowardZero(-35, -3); v != -33 {
					t.Error(v)
				}
			})
		})
		t.Run("Millis", func(t *testing.T) {
			if v := Millis[int](1.23); v != 1230 {
				t.Error(v)
			}
			if v := FloatMillis(1230); v != 1.23 {
				t.Error(v)
			}
			if v := Millis[int64](1); v != 1000 {
				t.Error(v)
			}
			if v := Millis[int32](300); v != 300_000 {
				t.Error(v)
			}
			if v := Millis[int16](0.1234567); v != 123 {
				t.Error(v)
			}
			if v := Millis[int](0.25); v != 250 {
				t.Error(v)
			}
			if v := FloatMillis(250); v != 0.25 {
				t.Error(v)
			}

			func() {
				defer func() {
					if p := recover(); p == nil {
						t.Error("should have been panic")
					}
				}()

				Millis[int64](math.MaxInt64)
			}()
		})
		t.Run("AwayFromZero", func(t *testing.T) {
			t.Run("Positive", func(t *testing.T) {
				if v := RoundToMultipleAwayFromZero(32, 3); v != 33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(31, 3); v != 33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(31, 2); v != 32 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(31, 5); v != 35 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(34, 5); v != 35 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(35, 3); v != 36 {
					t.Error(v)
				}
			})
			t.Run("Negative", func(t *testing.T) {
				if v := RoundToMultipleAwayFromZero(32, -3); v != 33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-32, -3); v != -33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-32, 3); v != -33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-31, 3); v != -33 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-31, 2); v != -32 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-29, 2); v != -30 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-31, 5); v != -35 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-36, 5); v != -40 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-35, 3); v != -36 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(-3, 35); v != 36 {
					t.Error(v)
				}
				if v := RoundToMultipleAwayFromZero(3, -35); v != -36 {
					t.Error(v)
				}
			})
		})
	})
	t.Run("Diff", func(t *testing.T) {
		if v := Diff(1, -1); v != 2 {
			t.Error(v)
		}
		if v := Diff(-1, 1); v != 2 {
			t.Error(v)
		}
		if v := Diff(-1, -1); v != 0 {
			t.Error(v)
		}
		if v := Diff(0, -0); v != 0 {
			t.Error(v)
		}
		if v := Diff(10, -10); v != 20 {
			t.Error(v)
		}
		if v := Diff(10, 100); v != 90 {
			t.Error(v)
		}
		if v := Diff(100, 10); v != 90 {
			t.Error(v)
		}
		if v := Diff(90, -10); v != 100 {
			t.Error(v)
		}
	})
	t.Run("Ranges", func(t *testing.T) {
		for _, tt := range []struct {
			Name        string
			One         int
			Two         int
			ExpectedMin int
			ExpectedMax int
			Operation   func(a, b int) (int, int)
		}{
			////////////////////////////////
			//
			// BOUNDS
			//
			{
				Name:        "ZeroNormal",
				One:         0,
				Two:         2,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   Bounds[int],
			},
			{
				Name:        "ZeroFlip",
				One:         2,
				Two:         0,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   Bounds[int],
			},
			{
				Name:        "NegativeNormal",
				One:         -64,
				Two:         128,
				ExpectedMin: 0,
				ExpectedMax: 128,
				Operation:   Bounds[int],
			},
			{
				Name:        "NegativeFlip",
				One:         128,
				Two:         -128,
				ExpectedMin: 0,
				ExpectedMax: 128,
				Operation:   Bounds[int],
			},
			{
				Name:        "PositiveNormal",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   Bounds[int],
			},
			{
				Name:        "PositiveFlip",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   Bounds[int],
			},
			////////////////////////////////
			//
			// RANGE
			//
			{
				Name:        "ZeroNormal",
				One:         0,
				Two:         2,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   Range[int],
			},
			{
				Name:        "ZeroFlip",
				One:         2,
				Two:         0,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   Range[int],
			},
			{
				Name:        "NegativeNormal",
				One:         -64,
				Two:         128,
				ExpectedMin: -64,
				ExpectedMax: 128,
				Operation:   Range[int],
			},
			{
				Name:        "NegativeFlip",
				One:         128,
				Two:         -128,
				ExpectedMin: -128,
				ExpectedMax: 128,
				Operation:   Range[int],
			},
			{
				Name:        "PositiveNormal",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   Range[int],
			},
			{
				Name:        "PositiveFlip",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   Range[int],
			},
			////////////////////////////////
			//
			// ABSBOUNDS
			//
			{
				Name:        "ZeroNormal",
				One:         0,
				Two:         2,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "ZeroFlip",
				One:         2,
				Two:         0,
				ExpectedMin: 0,
				ExpectedMax: 2,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "NegativeNormal",
				One:         -64,
				Two:         128,
				ExpectedMin: 64,
				ExpectedMax: 128,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "BothNegative",
				One:         -256,
				Two:         -128,
				ExpectedMin: 128,
				ExpectedMax: 256,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "BothNegativeFlip",
				One:         -32,
				Two:         -64,
				ExpectedMin: 32,
				ExpectedMax: 64,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "NegativeFlip",
				One:         128,
				Two:         -128,
				ExpectedMin: 128,
				ExpectedMax: 128,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "PositiveNormal",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   AbsBounds[int],
			},
			{
				Name:        "PositiveFlip",
				One:         512,
				Two:         1024,
				ExpectedMin: 512,
				ExpectedMax: 1024,
				Operation:   AbsBounds[int],
			},
		} {
			id := fmt.Sprintf("%s/%s/From/%d_%d/To/%d_%d",
				runtime.FuncForPC(reflect.ValueOf(tt.Operation).Pointer()).Name(),
				tt.Name, tt.One, tt.Two, tt.ExpectedMin, tt.ExpectedMax,
			)
			t.Run(id, func(t *testing.T) {
				outMin, outMax := tt.Operation(tt.One, tt.Two)
				if outMin != tt.ExpectedMin {
					t.Errorf("got %d rather than %d", outMin, tt.ExpectedMin)
				}
				if outMax != tt.ExpectedMax {
					t.Errorf("got %d rather than %d", outMin, tt.ExpectedMax)
				}
			})
		}
	})
}
