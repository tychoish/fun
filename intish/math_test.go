package intish

import (
	"math"
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
	t.Run("Max", func(t *testing.T) {
		if v := Max(0, 100); v != 100 {
			t.Error(v)
		}
		if v := Max(-10, 100); v != 100 {
			t.Error(v)
		}

		if v := Max(100, 0); v != 100 {
			t.Error(v)
		}
		if v := Max(100, -10); v != 100 {
			t.Error(v)
		}
	})
	t.Run("Min", func(t *testing.T) {
		if v := Min(-100, -10); v != -100 {
			t.Error(v)
		}
		if v := Min(-100, 0); v != -100 {
			t.Error(v)
		}
		if v := Min(-100, 100); v != -100 {
			t.Error(v)
		}

		if v := Min(-10, -100); v != -100 {
			t.Error(v)
		}
		if v := Min(0, -100); v != -100 {
			t.Error(v)
		}
		if v := Min(100, -100); v != -100 {
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
}
