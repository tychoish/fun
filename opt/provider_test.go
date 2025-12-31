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
