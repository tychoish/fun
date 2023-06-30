package internal

import "testing"

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
			if v := RoundToSmallestMultipe(35, 3); v != 33 {
				t.Error(v)
			}
			if v := RoundToSmallestMultipe(-35, 3); v != -36 {
				t.Error(v)
			}
			if v := RoundToSmallestMultipe(-31, 2); v != -32 {
				t.Error(v)
			}
			if v := RoundToSmallestMultipe(34, 5); v != 30 {
				t.Error(v)
			}
		})
		t.Run("Largest", func(t *testing.T) {
			if v := RoundToLargestMultipe(35, 3); v != 36 {
				t.Error(v)
			}
			if v := RoundToLargestMultipe(-35, 3); v != -33 {
				t.Error(v)
			}
			if v := RoundToLargestMultipe(-31, 2); v != -30 {
				t.Error(v)
			}
			if v := RoundToLargestMultipe(34, 5); v != 35 {
				t.Error(v)
			}
		})
		t.Run("Down", func(t *testing.T) {
			t.Run("Positive", func(t *testing.T) {
				if v := RoundDownToMultiple(32, 3); v != 30 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(31, 3); v != 30 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(31, 2); v != 30 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(31, 5); v != 30 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(34, 5); v != 30 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(35, 3); v != 33 {
					t.Error(v)
				}
			})
			t.Run("Negative", func(t *testing.T) {
				if v := RoundDownToMultiple(-32, 3); v != -33 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(-31, 3); v != -33 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(-31, 2); v != -32 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(-31, 5); v != -35 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(-34, 5); v != -35 {
					t.Error(v)
				}
				if v := RoundDownToMultiple(-35, 3); v != -36 {
					t.Error(v)
				}
			})
		})
		t.Run("Up", func(t *testing.T) {
			t.Run("Positive", func(t *testing.T) {
				if v := RoundUpToMultiple(32, 3); v != 33 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(31, 3); v != 33 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(31, 2); v != 32 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(31, 5); v != 35 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(34, 5); v != 35 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(35, 3); v != 36 {
					t.Error(v)
				}
			})
			t.Run("Negative", func(t *testing.T) {
				if v := RoundUpToMultiple(-32, 3); v != -30 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-31, 3); v != -30 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-31, 2); v != -30 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-29, 2); v != -28 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-31, 5); v != -30 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-36, 5); v != -35 {
					t.Error(v)
				}
				if v := RoundUpToMultiple(-35, 3); v != -33 {
					t.Error(v)
				}
			})

		})
	})
}
