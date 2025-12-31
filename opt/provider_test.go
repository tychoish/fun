package opt

import (
	"errors"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

type testConfig struct {
	Value     int
	Name      string
	Enabled   bool
	validated bool
}

func (t *testConfig) Validate() error {
	t.validated = true
	if t.Value < 0 {
		return errors.New("value must be non-negative")
	}
	return nil
}

func TestProvider(t *testing.T) {
	t.Run("Apply", func(t *testing.T) {
		t.Run("WithoutValidate", func(t *testing.T) {
			type simpleConfig struct {
				Value int
			}

			conf := &simpleConfig{}
			provider := Provider[*simpleConfig](func(c *simpleConfig) error {
				c.Value = 42
				return nil
			})

			err := provider.Apply(conf)
			check.NotError(t, err)
			check.Equal(t, conf.Value, 42)
		})

		t.Run("WithValidate", func(t *testing.T) {
			conf := &testConfig{}
			provider := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 10
				return nil
			})

			err := provider.Apply(conf)
			check.NotError(t, err)
			check.Equal(t, conf.Value, 10)
			check.True(t, conf.validated)
		})

		t.Run("ValidateReturnsError", func(t *testing.T) {
			conf := &testConfig{}
			provider := Provider[*testConfig](func(c *testConfig) error {
				c.Value = -5
				return nil
			})

			err := provider.Apply(conf)
			check.Error(t, err)
			check.Substring(t, err.Error(), "value must be non-negative")
		})

		t.Run("ProviderReturnsError", func(t *testing.T) {
			conf := &testConfig{}
			expectedErr := errors.New("provider error")
			provider := Provider[*testConfig](func(c *testConfig) error {
				return expectedErr
			})

			err := provider.Apply(conf)
			check.Error(t, err)
			check.ErrorIs(t, err, expectedErr)
		})
	})

	t.Run("Build", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			conf := &testConfig{}
			provider := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 42
				c.Name = "test"
				return nil
			})

			result, err := provider.Build(conf)
			check.NotError(t, err)
			check.True(t, result != nil)
			check.Equal(t, result.Value, 42)
			check.Equal(t, result.Name, "test")
		})

		t.Run("Error", func(t *testing.T) {
			conf := &testConfig{}
			expectedErr := errors.New("build error")
			provider := Provider[*testConfig](func(c *testConfig) error {
				return expectedErr
			})

			result, err := provider.Build(conf)
			check.Error(t, err)
			check.ErrorIs(t, err, expectedErr)
			check.True(t, result == nil)
		})
	})

	t.Run("Join", func(t *testing.T) {
		t.Run("MultipleProviders", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 10
				return nil
			})
			p2 := Provider[*testConfig](func(c *testConfig) error {
				c.Name = "joined"
				return nil
			})
			p3 := Provider[*testConfig](func(c *testConfig) error {
				c.Enabled = true
				return nil
			})
			testt.Logf(t, "before %+v", conf)

			combined := p1.Join(p2, p3)
			err := combined.Apply(conf)

			testt.Logf(t, "after %+v", conf)

			check.NotError(t, err)
			check.Equal(t, conf.Value, 10)
			check.Equal(t, conf.Name, "joined")
			check.True(t, conf.Enabled)
		})

		t.Run("SkipsNilProviders", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 5
				return nil
			})

			combined := p1.Join(nil, nil, nil)
			err := combined.Apply(conf)

			check.NotError(t, err)
			check.Equal(t, conf.Value, 5)
		})

		t.Run("ErrorInFirstProvider", func(t *testing.T) {
			conf := &testConfig{}
			expectedErr := errors.New("first error")

			p1 := Provider[*testConfig](func(c *testConfig) error {
				return expectedErr
			})
			p2 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 10
				return nil
			})

			combined := p1.Join(p2)
			err := combined.Apply(conf)

			check.Error(t, err)
			check.ErrorIs(t, err, expectedErr)
		})

		t.Run("ErrorInSecondProvider", func(t *testing.T) {
			conf := &testConfig{}
			expectedErr := errors.New("second error")

			p1 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 10
				return nil
			})
			p2 := Provider[*testConfig](func(c *testConfig) error {
				return expectedErr
			})

			combined := p1.Join(p2)
			err := combined.Apply(conf)

			check.Error(t, err)
			check.ErrorIs(t, err, expectedErr)
		})

		t.Run("PanicRecovery", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				panic("test panic")
			})

			p2 := Provider[*testConfig](func(c *testConfig) error {
				panic("test panic2")
			})

			check.NotPanic(t, func() {
				err := p1.Join(p2).Apply(conf)
				check.Error(t, err)
			})
		})

		t.Run("PanicRecovery", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				panic("test panic")
			})

			check.NotPanic(t, func() {
				err := p1.Apply(conf)
				check.Error(t, err)
			})
		})
		t.Run("OverridingValues", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 10
				return nil
			})
			p2 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 20
				return nil
			})

			combined := p1.Join(p2)
			err := combined.Apply(conf)

			check.NotError(t, err)
			// Last provider wins
			check.Equal(t, conf.Value, 20)
		})
	})

	t.Run("JoinFunction", func(t *testing.T) {
		t.Run("EmptyProviders", func(t *testing.T) {
			conf := &testConfig{}
			combined := Join[*testConfig]()
			err := combined.Apply(conf)

			check.NotError(t, err)
			// Config should remain unchanged
			check.Equal(t, conf.Value, 0)
			check.Equal(t, conf.Name, "")
			check.True(t, !conf.Enabled)
		})

		t.Run("MultipleProviders", func(t *testing.T) {
			conf := &testConfig{}

			p1 := Provider[*testConfig](func(c *testConfig) error {
				c.Value = 15
				return nil
			})
			p2 := Provider[*testConfig](func(c *testConfig) error {
				c.Name = "combined"
				return nil
			})

			combined := Join(p1, p2)
			err := combined.Apply(conf)

			check.NotError(t, err)
			check.Equal(t, conf.Value, 15)
			check.Equal(t, conf.Name, "combined")
		})
	})
}

func TestJoinTableDriven(t *testing.T) {
	tests := []struct {
		name          string
		providerCount int
		includeNils   bool
		expectError   bool
		errorProvider int // -1 for none, otherwise index of provider that errors
	}{
		{
			name:          "ZeroProviders",
			providerCount: 0,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "SingleProvider",
			providerCount: 1,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "TwoProviders",
			providerCount: 2,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "ThreeProviders",
			providerCount: 3,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "FiveProviders",
			providerCount: 5,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "TenProviders",
			providerCount: 10,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "TwoProvidersWithNils",
			providerCount: 2,
			includeNils:   true,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "FiveProvidersWithNils",
			providerCount: 5,
			includeNils:   true,
			expectError:   false,
			errorProvider: -1,
		},
		{
			name:          "ThreeProvidersErrorInFirst",
			providerCount: 3,
			expectError:   true,
			errorProvider: 0,
		},
		{
			name:          "ThreeProvidersErrorInMiddle",
			providerCount: 3,
			expectError:   true,
			errorProvider: 1,
		},
		{
			name:          "ThreeProvidersErrorInLast",
			providerCount: 3,
			expectError:   true,
			errorProvider: 2,
		},
		{
			name:          "FiveProvidersErrorInMiddle",
			providerCount: 5,
			expectError:   true,
			errorProvider: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &testConfig{}

			// Track call counts and order
			callCounts := make([]int, tt.providerCount)
			callOrder := []int{}
			expectedErr := errors.New("expected error")

			// Build providers
			providers := make([]Provider[*testConfig], 0, tt.providerCount*2)
			for i := 0; i < tt.providerCount; i++ {
				idx := i // capture for closure
				provider := Provider[*testConfig](func(c *testConfig) error {
					callCounts[idx]++
					callOrder = append(callOrder, idx)
					c.Value += idx + 1 // Each provider adds a unique value

					if tt.errorProvider == idx {
						return expectedErr
					}
					return nil
				})
				providers = append(providers, provider)

				// Optionally insert nil providers
				if tt.includeNils && i < tt.providerCount-1 {
					providers = append(providers, nil)
				}
			}

			// Call Join with the providers
			combined := Join(providers...)
			err := combined.Apply(conf)

			// Verify error expectation
			if tt.expectError {
				check.Error(t, err)
				check.ErrorIs(t, err, expectedErr)
			} else {
				check.NotError(t, err)
			}

			// Verify all providers were called exactly once
			for i := 0; i < tt.providerCount; i++ {
				if callCounts[i] != 1 {
					t.Errorf("provider %d: expected 1 call, got %d", i, callCounts[i])
				}
			}

			// Verify providers were called in the correct sequence
			if len(callOrder) != tt.providerCount {
				t.Fatalf("expected %d calls, got %d", tt.providerCount, len(callOrder))
			}
			for i := 0; i < tt.providerCount; i++ {
				if callOrder[i] != i {
					t.Errorf("call %d: expected provider %d, got provider %d", i, i, callOrder[i])
				}
			}

			// Verify the accumulated value if no error occurred
			if !tt.expectError {
				expectedValue := 0
				for i := 1; i <= tt.providerCount; i++ {
					expectedValue += i
				}
				if conf.Value != expectedValue {
					t.Errorf("expected Value=%d, got %d", expectedValue, conf.Value)
				}
			}

			// Verify config was validated (testConfig has Validate method)
			if !tt.expectError && tt.providerCount > 0 {
				check.True(t, conf.validated)
			}
		})
	}
}

func TestJoinCallOrderAndCount(t *testing.T) {
	t.Run("ComplexCallSequence", func(t *testing.T) {
		type callRecord struct {
			providerID int
			callIndex  int
		}

		conf := &testConfig{}
		callLog := []callRecord{}
		callCounter := 0

		// Create 7 providers that log their calls
		providers := make([]Provider[*testConfig], 7)
		for i := 0; i < 7; i++ {
			idx := i
			providers[i] = Provider[*testConfig](func(c *testConfig) error {
				callCounter++
				callLog = append(callLog, callRecord{
					providerID: idx,
					callIndex:  callCounter,
				})
				return nil
			})
		}

		// Mix in some nil providers
		combined := Join(
			providers[0],
			nil,
			providers[1],
			providers[2],
			nil,
			nil,
			providers[3],
			providers[4],
			providers[5],
			nil,
			providers[6],
		)

		err := combined.Apply(conf)
		check.NotError(t, err)

		// Verify exactly 7 calls were made
		if len(callLog) != 7 {
			t.Fatalf("expected 7 calls, got %d", len(callLog))
		}

		// Verify the sequence: 0, 1, 2, 3, 4, 5, 6
		for i := 0; i < 7; i++ {
			if callLog[i].providerID != i {
				t.Errorf("call %d: expected provider %d, got %d",
					i, i, callLog[i].providerID)
			}
			if callLog[i].callIndex != i+1 {
				t.Errorf("call %d: expected callIndex %d, got %d",
					i, i+1, callLog[i].callIndex)
			}
		}
	})

	t.Run("MultipleApplicationsSameProvider", func(t *testing.T) {
		callCount := 0

		p1 := Provider[*testConfig](func(c *testConfig) error {
			callCount++
			c.Value = 10
			return nil
		})
		p2 := Provider[*testConfig](func(c *testConfig) error {
			callCount++
			c.Value = 20
			return nil
		})

		combined := Join(p1, p2)

		// Apply multiple times
		conf1 := &testConfig{}
		err := combined.Apply(conf1)
		check.NotError(t, err)
		check.Equal(t, conf1.Value, 20)

		conf2 := &testConfig{}
		err = combined.Apply(conf2)
		check.NotError(t, err)
		check.Equal(t, conf2.Value, 20)

		conf3 := &testConfig{}
		err = combined.Apply(conf3)
		check.NotError(t, err)
		check.Equal(t, conf3.Value, 20)

		// Each application should call both providers once
		// So total calls = 3 applications * 2 providers = 6
		if callCount != 6 {
			t.Errorf("expected 6 total calls, got %d", callCount)
		}
	})

	t.Run("AllNilProviders", func(t *testing.T) {
		conf := &testConfig{}
		combined := Join[*testConfig](nil, nil, nil, nil)
		err := combined.Apply(conf)

		check.NotError(t, err)
		check.Equal(t, conf.Value, 0)
		check.Equal(t, conf.Name, "")
	})

	t.Run("ErrorDoesNotStopExecution", func(t *testing.T) {
		// In opt.Provider.Join, all providers are called even if one errors
		conf := &testConfig{}
		callCount := 0
		expectedErr := errors.New("middle error")

		p1 := Provider[*testConfig](func(c *testConfig) error {
			callCount++
			c.Value = 1
			return nil
		})
		p2 := Provider[*testConfig](func(c *testConfig) error {
			callCount++
			c.Value = 2
			return expectedErr
		})
		p3 := Provider[*testConfig](func(c *testConfig) error {
			callCount++
			c.Value = 3
			return nil
		})

		combined := Join(p1, p2, p3)
		err := combined.Apply(conf)

		check.Error(t, err)
		check.ErrorIs(t, err, expectedErr)

		// All three providers should have been called
		if callCount != 3 {
			t.Errorf("expected 3 calls, got %d", callCount)
		}

		// Last provider should have set the value
		check.Equal(t, conf.Value, 3)
	})
}
