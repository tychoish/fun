package wpa

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/opt"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &WorkerGroupConf{ExcludedErrors: []error{ers.ErrLimitExceeded}}
		newc := &WorkerGroupConf{ContinueOnPanic: true, ExcludedErrors: []error{ers.ErrRecoveredPanic, ers.ErrInvariantViolation}}
		eone := errors.New("cat")
		etwo := errors.New("3")

		err := WorkerGroupConfAddExcludeErrors(eone, etwo).Join(
			func(_ *WorkerGroupConf) error {
				return nil
			},
			WorkerGroupConfSet(newc),
		).Apply(conf)

		check.NotError(t, err)
		check.Equal(t, len(conf.ExcludedErrors), 2)
		check.True(t, conf.ContinueOnPanic)
		check.NotContains(t, conf.ExcludedErrors, eone)
		check.NotContains(t, conf.ExcludedErrors, etwo)
	})
	t.Run("SkippsNil", func(t *testing.T) {
		assert.NotError(t, WorkerGroupConfAddExcludeErrors(nil).Join(nil, nil, nil).Apply(&WorkerGroupConf{}))
	})
	t.Run("Error", func(t *testing.T) {
		of := WorkerGroupConfAddExcludeErrors(ers.ErrRecoveredPanic)
		opt := &WorkerGroupConf{}
		assert.Equal(t, 0, len(opt.ExcludedErrors))
		err := of(opt)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "cannot exclude recovered panics")
	})
	t.Run("Collector", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		check.Error(t, WorkerGroupConfWithErrorCollector(nil)(opt))
		check.NotError(t, WorkerGroupConfWithErrorCollector(&erc.Collector{})(opt))
	})
	t.Run("Build", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		n, err := WorkerGroupConfNumWorkers(42).Build(conf)
		assert.NotError(t, err)
		assert.True(t, n != nil)
		assert.Equal(t, n.NumWorkers, 42)

		n, err = opt.New(func(*WorkerGroupConf) error { return errors.New("hi") }).Build(conf)
		assert.Error(t, err)
		assert.True(t, n == nil)
	})
	t.Run("HandleErrorEdgecases", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		t.Run("NilError", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, opt.CanContinueOnError(nil))
			check.Equal(t, 0, opt.ErrorCollector.Len())
		})
		t.Run("Termintaing", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, ft.Not(opt.CanContinueOnError(io.EOF)))
		})

		t.Run("Continue", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, opt.CanContinueOnError(ers.ErrCurrentOpSkip))
			check.Equal(t, 0, opt.ErrorCollector.Len())
		})
	})
}

func TestWorkerGroupConfOptions(t *testing.T) {
	t.Run("WorkerGroupConfDefaults", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := WorkerGroupConfDefaults().Apply(conf)

		assert.NotError(t, err)
		assert.True(t, conf.ContinueOnError)
		assert.True(t, conf.NumWorkers > 0)
	})

	t.Run("WorkerGroupConfNumWorkers", func(t *testing.T) {
		tests := []struct {
			name     string
			input    int
			expected int
		}{
			{name: "PositiveValue", input: 10, expected: 10},
			{name: "Zero", input: 0, expected: 1},
			{name: "NegativeValue", input: -5, expected: 1},
			{name: "LargeValue", input: 1000, expected: 1000},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				conf := &WorkerGroupConf{}
				err := WorkerGroupConfNumWorkers(tt.input).Apply(conf)

				assert.NotError(t, err)
				assert.Equal(t, conf.NumWorkers, tt.expected)
			})
		}
	})

	t.Run("WorkerGroupConfContinueOnError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := WorkerGroupConfContinueOnError().Apply(conf)

		assert.NotError(t, err)
		assert.True(t, conf.ContinueOnError)
	})

	t.Run("WorkerGroupConfContinueOnPanic", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := WorkerGroupConfContinueOnPanic().Apply(conf)

		assert.NotError(t, err)
		assert.True(t, conf.ContinueOnPanic)
	})

	t.Run("WorkerGroupConfIncludeContextErrors", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := WorkerGroupConfIncludeContextErrors().Apply(conf)

		assert.NotError(t, err)
		assert.True(t, conf.IncludeContextExpirationErrors)
	})

	t.Run("WorkerGroupConfAddExcludeErrors", func(t *testing.T) {
		t.Run("ValidErrors", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			err1 := errors.New("error1")
			err2 := errors.New("error2")

			err := WorkerGroupConfAddExcludeErrors(err1, err2).Apply(conf)

			assert.NotError(t, err)
			assert.Equal(t, len(conf.ExcludedErrors), 2)
			assert.Contains(t, conf.ExcludedErrors, err1)
			assert.Contains(t, conf.ExcludedErrors, err2)
		})

		t.Run("RecoveredPanicError", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			err := WorkerGroupConfAddExcludeErrors(ers.ErrRecoveredPanic).Apply(conf)

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
			assert.Equal(t, len(conf.ExcludedErrors), 0)
		})

		t.Run("MixedWithRecoveredPanic", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			err1 := errors.New("valid")
			err := WorkerGroupConfAddExcludeErrors(err1, ers.ErrRecoveredPanic).Apply(conf)

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
		})

		t.Run("MultipleApplications", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			err1 := errors.New("first")
			err2 := errors.New("second")

			err := opt.Join(
				WorkerGroupConfAddExcludeErrors(err1),
				WorkerGroupConfAddExcludeErrors(err2),
			).Apply(conf)

			assert.NotError(t, err)
			assert.Equal(t, len(conf.ExcludedErrors), 2)
			assert.Contains(t, conf.ExcludedErrors, err1)
			assert.Contains(t, conf.ExcludedErrors, err2)
		})
	})

	t.Run("WorkerGroupConfWithErrorCollector", func(t *testing.T) {
		t.Run("ValidCollector", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			ec := &erc.Collector{}

			err := WorkerGroupConfWithErrorCollector(ec).Apply(conf)

			assert.NotError(t, err)
			assert.True(t, conf.ErrorCollector == ec)
		})

		t.Run("NilCollector", func(t *testing.T) {
			conf := &WorkerGroupConf{}
			err := WorkerGroupConfWithErrorCollector(nil).Apply(conf)

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
		})
	})

	t.Run("WorkerGroupConfSet", func(t *testing.T) {
		original := &WorkerGroupConf{NumWorkers: 5}
		replacement := &WorkerGroupConf{
			NumWorkers:      10,
			ContinueOnError: true,
			ContinueOnPanic: true,
		}

		err := WorkerGroupConfSet(replacement).Apply(original)

		assert.NotError(t, err)
		assert.Equal(t, original.NumWorkers, 10)
		assert.True(t, original.ContinueOnError)
		assert.True(t, original.ContinueOnPanic)
	})
}

func TestWorkerGroupConfJoinOptions(t *testing.T) {
	t.Run("EmptyOptions", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := opt.Join[*WorkerGroupConf]().Apply(conf)

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 1)      // because it's overridden by the validate method
		assert.NotNilPtr(t, conf.ErrorCollector) // set by validate method
		assert.True(t, ft.Not(conf.ContinueOnError))
		assert.True(t, ft.Not(conf.ContinueOnPanic))
		assert.True(t, ft.Not(conf.IncludeContextExpirationErrors))
		assert.Equal(t, len(conf.ExcludedErrors), 0)
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := opt.Join(
			WorkerGroupConfNumWorkers(8),
			WorkerGroupConfContinueOnError(),
			WorkerGroupConfContinueOnPanic(),
		).Apply(conf)

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 8)
		assert.True(t, conf.ContinueOnError)
		assert.True(t, conf.ContinueOnPanic)
	})

	t.Run("ConflictingOptions", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := opt.Join(
			WorkerGroupConfNumWorkers(5),
			WorkerGroupConfNumWorkers(10),
		).Apply(conf)

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 10)
	})

	t.Run("OptionsWithError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := opt.Join(
			WorkerGroupConfNumWorkers(8),
			WorkerGroupConfWithErrorCollector(nil),
			WorkerGroupConfContinueOnError(),
		).Apply(conf)

		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
	})

	t.Run("ComplexConfiguration", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		ec := &erc.Collector{}
		err1 := errors.New("exclude1")

		err := opt.Join(
			WorkerGroupConfNumWorkers(4),
			WorkerGroupConfContinueOnError(),
			WorkerGroupConfContinueOnPanic(),
			WorkerGroupConfIncludeContextErrors(),
			WorkerGroupConfAddExcludeErrors(err1),
			WorkerGroupConfWithErrorCollector(ec),
		).Apply(conf)

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 4)
		assert.True(t, conf.ContinueOnError)
		assert.True(t, conf.ContinueOnPanic)
		assert.True(t, conf.IncludeContextExpirationErrors)
		assert.Equal(t, len(conf.ExcludedErrors), 1)
		assert.True(t, conf.ErrorCollector == ec)
	})
}

func TestWorkerGroupConfValidate(t *testing.T) {
	t.Run("ZeroWorkers", func(t *testing.T) {
		conf := &WorkerGroupConf{NumWorkers: 0}
		err := conf.Validate()

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 1)
	})

	t.Run("NegativeWorkers", func(t *testing.T) {
		conf := &WorkerGroupConf{NumWorkers: -10}
		err := conf.Validate()

		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 1)
	})

	t.Run("NilErrorCollector", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := conf.Validate()

		assert.NotError(t, err)
		assert.NotNilPtr(t, conf.ErrorCollector)
	})

	t.Run("ExistingErrorCollector", func(t *testing.T) {
		ec := &erc.Collector{}
		conf := &WorkerGroupConf{ErrorCollector: ec}
		err := conf.Validate()

		assert.NotError(t, err)
		assert.True(t, conf.ErrorCollector == ec)
	})
}

func TestWorkerGroupConfCanContinueOnError(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		conf.Validate()

		assert.True(t, conf.CanContinueOnError(nil))
		assert.Equal(t, conf.ErrorCollector.Len(), 0)
	})

	t.Run("ErrCurrentOpSkip", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		conf.Validate()

		assert.True(t, conf.CanContinueOnError(ers.ErrCurrentOpSkip))
		assert.Equal(t, conf.ErrorCollector.Len(), 0)
	})

	t.Run("TerminatingErrors", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
		}{
			{name: "io.EOF", err: io.EOF},
			{name: "ErrCurrentOpAbort", err: ers.ErrCurrentOpAbort},
			{name: "ErrContainerClosed", err: ers.ErrContainerClosed},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				conf := &WorkerGroupConf{}
				conf.Validate()

				assert.True(t, ft.Not(conf.CanContinueOnError(tt.err)))
				assert.Equal(t, conf.ErrorCollector.Len(), 0)
			})
		}
	})

	t.Run("RecoveredPanic", func(t *testing.T) {
		t.Run("ContinueOnPanicFalse", func(t *testing.T) {
			conf := &WorkerGroupConf{ContinueOnPanic: false}
			conf.Validate()

			assert.True(t, ft.Not(conf.CanContinueOnError(ers.ErrRecoveredPanic)))
			assert.Equal(t, conf.ErrorCollector.Len(), 1)
		})

		t.Run("ContinueOnPanicTrue", func(t *testing.T) {
			conf := &WorkerGroupConf{ContinueOnPanic: true}
			conf.Validate()

			assert.True(t, conf.CanContinueOnError(ers.ErrRecoveredPanic))
			assert.Equal(t, conf.ErrorCollector.Len(), 1)
		})
	})

	t.Run("ContextErrors", func(t *testing.T) {
		t.Run("IncludeContextErrorsFalse", func(t *testing.T) {
			conf := &WorkerGroupConf{IncludeContextExpirationErrors: false}
			conf.Validate()

			assert.True(t, ft.Not(conf.CanContinueOnError(errors.New("context canceled"))))
			assert.Equal(t, conf.ErrorCollector.Len(), 1)
		})

		t.Run("IncludeContextErrorsTrue", func(t *testing.T) {
			conf := &WorkerGroupConf{IncludeContextExpirationErrors: true}
			conf.Validate()

			// Context errors should still return false for continuation
			// but the error should not be collected
			assert.True(t, ft.Not(conf.CanContinueOnError(errors.New("context deadline exceeded"))))
		})
	})

	t.Run("ExpiredContextErrors", func(t *testing.T) {
		t.Run("AlwaysReturnsFalse", func(t *testing.T) {
			tests := []struct {
				name                           string
				err                            error
				includeContextExpirationErrors bool
			}{
				{name: "Canceled_IncludeFalse", err: context.Canceled, includeContextExpirationErrors: false},
				{name: "Canceled_IncludeTrue", err: context.Canceled, includeContextExpirationErrors: true},
				{name: "DeadlineExceeded_IncludeFalse", err: context.DeadlineExceeded, includeContextExpirationErrors: false},
				{name: "DeadlineExceeded_IncludeTrue", err: context.DeadlineExceeded, includeContextExpirationErrors: true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					conf := &WorkerGroupConf{
						IncludeContextExpirationErrors: tt.includeContextExpirationErrors,
					}
					conf.Validate()

					result := conf.CanContinueOnError(tt.err)
					assert.True(t, ft.Not(result))
				})
			}
		})

		t.Run("ErrorCollectorRespectsIncludeSetting", func(t *testing.T) {
			t.Run("DoesNotCollectWhenIncludeFalse", func(t *testing.T) {
				conf := &WorkerGroupConf{
					IncludeContextExpirationErrors: false,
				}
				conf.Validate()

				result := conf.CanContinueOnError(context.Canceled)

				assert.True(t, ft.Not(result))
				// Error should NOT be collected when include is false
				assert.Equal(t, conf.ErrorCollector.Len(), 0)
			})

			t.Run("CollectsWhenIncludeTrue", func(t *testing.T) {
				conf := &WorkerGroupConf{
					IncludeContextExpirationErrors: true,
				}
				conf.Validate()

				result := conf.CanContinueOnError(context.DeadlineExceeded)

				assert.True(t, ft.Not(result))
				// Error should be collected when include is true
				assert.Equal(t, conf.ErrorCollector.Len(), 1)
			})

			t.Run("MultipleContextErrors", func(t *testing.T) {
				t.Run("IncludeFalse", func(t *testing.T) {
					conf := &WorkerGroupConf{
						IncludeContextExpirationErrors: false,
					}
					conf.Validate()

					conf.CanContinueOnError(context.Canceled)
					conf.CanContinueOnError(context.DeadlineExceeded)
					conf.CanContinueOnError(context.Canceled)

					// No errors should be collected
					assert.Equal(t, conf.ErrorCollector.Len(), 0)
				})

				t.Run("IncludeTrue", func(t *testing.T) {
					conf := &WorkerGroupConf{
						IncludeContextExpirationErrors: true,
					}
					conf.Validate()

					conf.CanContinueOnError(context.Canceled)
					conf.CanContinueOnError(context.DeadlineExceeded)
					conf.CanContinueOnError(context.Canceled)

					// All errors should be collected
					assert.Equal(t, conf.ErrorCollector.Len(), 3)
				})
			})
		})
	})

	t.Run("RegularError", func(t *testing.T) {
		t.Run("ContinueOnErrorFalse", func(t *testing.T) {
			conf := &WorkerGroupConf{ContinueOnError: false}
			conf.Validate()

			testErr := errors.New("test error")
			assert.True(t, ft.Not(conf.CanContinueOnError(testErr)))
			assert.Equal(t, conf.ErrorCollector.Len(), 1)
		})

		t.Run("ContinueOnErrorTrue", func(t *testing.T) {
			conf := &WorkerGroupConf{ContinueOnError: true}
			conf.Validate()

			testErr := errors.New("test error")
			assert.True(t, conf.CanContinueOnError(testErr))
			assert.Equal(t, conf.ErrorCollector.Len(), 1)
		})
	})
}

func TestWorkerGroupConfFilter(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		conf.Validate()

		assert.NotError(t, conf.Filter(nil))
	})

	t.Run("ErrorThatContinues", func(t *testing.T) {
		conf := &WorkerGroupConf{ContinueOnError: true}
		conf.Validate()

		testErr := errors.New("test")
		filtered := conf.Filter(testErr)

		assert.ErrorIs(t, filtered, ers.ErrCurrentOpSkip)
	})

	t.Run("ErrorThatDoesNotContinue", func(t *testing.T) {
		conf := &WorkerGroupConf{ContinueOnError: false}
		conf.Validate()

		testErr := errors.New("test")
		filtered := conf.Filter(testErr)

		assert.ErrorIs(t, filtered, testErr)
	})

	t.Run("SkipError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		conf.Validate()

		filtered := conf.Filter(ers.ErrCurrentOpSkip)
		assert.ErrorIs(t, filtered, ers.ErrCurrentOpSkip)
	})

	t.Run("TerminatingError", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		conf.Validate()

		filtered := conf.Filter(io.EOF)
		assert.ErrorIs(t, filtered, io.EOF)
	})

	t.Run("RecoveredPanicWithContinue", func(t *testing.T) {
		conf := &WorkerGroupConf{ContinueOnPanic: true}
		conf.Validate()

		filtered := conf.Filter(ers.ErrRecoveredPanic)
		assert.ErrorIs(t, filtered, ers.ErrCurrentOpSkip)
		assert.Equal(t, conf.ErrorCollector.Len(), 1)
	})

	t.Run("RecoveredPanicWithoutContinue", func(t *testing.T) {
		conf := &WorkerGroupConf{ContinueOnPanic: false}
		conf.Validate()

		filtered := conf.Filter(ers.ErrRecoveredPanic)
		assert.ErrorIs(t, filtered, ers.ErrRecoveredPanic)
		assert.Equal(t, conf.ErrorCollector.Len(), 1)
	})
}

func TestCustomValidators(t *testing.T) {
	t.Run("ValidatorReturnsError", func(t *testing.T) {
		expectedErr := errors.New("custom validation failed")
		validator := func(conf *WorkerGroupConf) error {
			return expectedErr
		}

		conf := &WorkerGroupConf{}
		// Apply automatically calls Validate, so we expect the error here
		err := WorkerGroupConfCustomValidatorAppend(validator).Apply(conf)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), expectedErr.Error())
		assert.Equal(t, len(conf.CustomValidators), 1)
	})

	t.Run("ValidatorReturnsNil", func(t *testing.T) {
		validator := func(conf *WorkerGroupConf) error {
			return nil
		}

		conf := &WorkerGroupConf{}
		err := WorkerGroupConfCustomValidatorAppend(validator).Apply(conf)
		assert.NotError(t, err)

		// Validate should not return error
		err = conf.Validate()
		assert.NotError(t, err)
	})

	t.Run("MultipleValidators", func(t *testing.T) {
		err1 := errors.New("validation error 1")
		err2 := errors.New("validation error 2")

		validator1 := func(conf *WorkerGroupConf) error {
			if conf.NumWorkers < 2 {
				return err1
			}
			return nil
		}
		validator2 := func(conf *WorkerGroupConf) error {
			if conf.NumWorkers > 10 {
				return err2
			}
			return nil
		}

		t.Run("BothPass", func(t *testing.T) {
			conf := &WorkerGroupConf{NumWorkers: 5}
			err := opt.Join(
				WorkerGroupConfCustomValidatorAppend(validator1),
				WorkerGroupConfCustomValidatorAppend(validator2),
			).Apply(conf)
			assert.NotError(t, err)

			err = conf.Validate()
			assert.NotError(t, err)
		})

		t.Run("FirstFails", func(t *testing.T) {
			conf := &WorkerGroupConf{NumWorkers: 1}
			// Apply calls Validate, so we get the error here
			err := opt.Join(
				WorkerGroupConfCustomValidatorAppend(validator1),
				WorkerGroupConfCustomValidatorAppend(validator2),
			).Apply(conf)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), err1.Error())
		})

		t.Run("SecondFails", func(t *testing.T) {
			conf := &WorkerGroupConf{NumWorkers: 15}
			// Apply calls Validate, so we get the error here
			err := opt.Join(
				WorkerGroupConfCustomValidatorAppend(validator1),
				WorkerGroupConfCustomValidatorAppend(validator2),
			).Apply(conf)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), err2.Error())
		})

		t.Run("BothFail", func(t *testing.T) {
			// NumWorkers will be set to 1 by Validate (from 0)
			// Then validator1 will fail
			conf := &WorkerGroupConf{NumWorkers: 0}
			// Apply calls Validate, so we get the error here
			err := opt.Join(
				WorkerGroupConfCustomValidatorAppend(validator1),
				WorkerGroupConfCustomValidatorAppend(validator2),
			).Apply(conf)
			assert.Error(t, err)
			// Both validator errors should be present
			assert.Substring(t, err.Error(), err1.Error())
		})
	})

	t.Run("ValidatorReset", func(t *testing.T) {
		expectedErr := errors.New("should not be called")
		validator := func(conf *WorkerGroupConf) error {
			return expectedErr
		}

		conf := &WorkerGroupConf{}
		err := opt.Join(
			WorkerGroupConfCustomValidatorAppend(validator),
			WorkerGroupConfCustomValidatorReset(),
		).Apply(conf)
		assert.NotError(t, err)
		assert.Equal(t, len(conf.CustomValidators), 0)

		// Validate should not return error since validators were reset
		err = conf.Validate()
		assert.NotError(t, err)
	})

	t.Run("ValidatorResetThenAppend", func(t *testing.T) {
		err1 := errors.New("first validator")
		err2 := errors.New("second validator")

		validator1 := func(conf *WorkerGroupConf) error {
			return err1
		}
		validator2 := func(conf *WorkerGroupConf) error {
			return err2
		}

		conf := &WorkerGroupConf{}
		// Apply calls Validate, which will run validator2 (validator1 was reset)
		err := opt.Join(
			WorkerGroupConfCustomValidatorAppend(validator1),
			WorkerGroupConfCustomValidatorReset(),
			WorkerGroupConfCustomValidatorAppend(validator2),
		).Apply(conf)
		assert.Error(t, err)
		assert.Equal(t, len(conf.CustomValidators), 1)
		// Should only get err2, not err1
		assert.Substring(t, err.Error(), err2.Error())
		assert.True(t, ft.Not(errors.Is(err, err1)))
	})

	t.Run("ValidatorWithConfModification", func(t *testing.T) {
		validator := func(conf *WorkerGroupConf) error {
			// Validator can also modify the config
			if conf.NumWorkers > 100 {
				conf.NumWorkers = 100
			}
			return nil
		}

		conf := &WorkerGroupConf{NumWorkers: 200}
		err := WorkerGroupConfCustomValidatorAppend(validator).Apply(conf)
		assert.NotError(t, err)

		err = conf.Validate()
		assert.NotError(t, err)
		assert.Equal(t, conf.NumWorkers, 100)
	})

	t.Run("EmptyValidatorsList", func(t *testing.T) {
		conf := &WorkerGroupConf{}
		err := conf.Validate()
		assert.NotError(t, err)
	})

	t.Run("ValidatorOrderMatters", func(t *testing.T) {
		var callOrder []int

		validator1 := func(conf *WorkerGroupConf) error {
			callOrder = append(callOrder, 1)
			return nil
		}
		validator2 := func(conf *WorkerGroupConf) error {
			callOrder = append(callOrder, 2)
			return nil
		}
		validator3 := func(conf *WorkerGroupConf) error {
			callOrder = append(callOrder, 3)
			return nil
		}

		conf := &WorkerGroupConf{}
		err := opt.Join(
			WorkerGroupConfCustomValidatorAppend(validator1),
			WorkerGroupConfCustomValidatorAppend(validator2),
			WorkerGroupConfCustomValidatorAppend(validator3),
		).Apply(conf)
		assert.NotError(t, err)

		err = conf.Validate()
		assert.NotError(t, err)

		// Validators should be called in order
		assert.True(t, len(callOrder) >= 3)

		// Find first occurrence of each validator
		first1, first2, first3 := -1, -1, -1
		for i, v := range callOrder {
			if v == 1 && first1 == -1 {
				first1 = i
			}
			if v == 2 && first2 == -1 {
				first2 = i
			}
			if v == 3 && first3 == -1 {
				first3 = i
			}
		}

		// Check order is preserved
		assert.True(t, first1 < first2)
		assert.True(t, first2 < first3)
	})
}
