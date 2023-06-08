package ers

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	err := errors.New("ERRNO=42")
	t.Run("Constructor", func(t *testing.T) {
		if GetTime(WithTime(err)).IsZero() {
			t.Fatal("not timestamped")
		}
	})
	t.Run("FormattingPassthrough", func(t *testing.T) {
		terr := WithTime(err)
		if err.Error() != terr.Error() {
			t.Fatal("unexpected error value:", terr)
		}
	})
	t.Run("ConstructoPopulates", func(t *testing.T) {
		terr := WithTime(err).(*timestamped)
		if terr.ts.IsZero() {
			t.Error("timestamp should always be populated")
		}
		if !terr.ts.Equal(terr.Time()) {
			t.Error("unexpected time value difference")
		}
	})
	t.Run("IsPassthrough", func(t *testing.T) {
		terr := WithTime(err)
		if !errors.Is(terr, err) {
			t.Error("wrapping does not reveal root error")
		}
	})
	t.Run("GetTime", func(t *testing.T) {
		t.Run("NonError", func(t *testing.T) {
			if !GetTime(err).IsZero() {
				t.Error("GetTime should be zero for arbitrary errors")
			}
		})
		t.Run("Wrapped", func(t *testing.T) {
			now := time.Now()

			time.Sleep(time.Millisecond)

			terr := fmt.Errorf("outer: %w", WithTime(err))
			ts := GetTime(terr)

			if ts.IsZero() {
				t.Error("GetTIme should resolve wrapped errors")
			}

			if !now.Before(ts) {
				t.Errorf("times should be closer now=%q, ts=%q", now, ts)
			}
		})
		t.Run("Expected", func(t *testing.T) {
			terr := WithTime(err)
			verr := terr.(*timestamped)
			if !GetTime(terr).Equal(verr.ts) {
				t.Error("times should be the same")
			}
		})
	})
	t.Run("NilErrors", func(t *testing.T) {
		if WithTime(nil) != nil {
			t.Error("timestamp shouldn't zero")
		}
	})

	t.Run("Formatting", func(t *testing.T) {
		now := time.Now()
		exp := WithTime(err)

		t.Run("Expanded+vShouldHaveTime", func(t *testing.T) {
			if str := fmt.Sprintf("%+v", exp); !strings.Contains(str, fmt.Sprint(now.Year())) {
				t.Error("unexpected fmt", str)
			}
		})
		t.Run("NormalVShouldNot", func(t *testing.T) {
			if str := fmt.Sprintf("%v", exp); strings.Contains(str, fmt.Sprint(now.Year())) {
				t.Error("unexpected fmt", str)
			}
		})
		t.Run("Passthrough", func(t *testing.T) {
			if fmt.Sprintf("%q", exp) != fmt.Sprintf("%q", err) {
				t.Error("quote formatting differs")
			}
			if fmt.Sprintf("%v", exp) != fmt.Sprintf("%v", err) {
				t.Error("quote formatting differs")
			}
			if fmt.Sprintf("%s", exp) != fmt.Sprintf("%s", err) {
				t.Error("quote formatting differs")
			}
		})
	})
	t.Run("As", func(t *testing.T) {
		exp := WithTime(err)
		var sincer interface{ Since(time.Time) time.Duration }
		if errors.As(exp, &sincer) {
			t.Error("shouldn't cast self to time")
		}
		tse := &timestamped{}
		if !errors.As(exp, &tse) {
			t.Error("should use as to cast to self")
		}
	})
}
