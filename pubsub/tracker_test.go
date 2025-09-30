package pubsub

import (
	"math"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestTracker(t *testing.T) {
	// mostly just confirmation/consistency testing
	var tracker queueLimitTracker

	t.Run("NoLimitIsUnlimted", func(t *testing.T) {
		tracker = &queueNoLimitTrackerImpl{}
		check.Equal(t, tracker.cap(), math.MaxInt)
		if t.Failed() {
			t.Log("unlimited should basically be unlimited")
		}
	})
	t.Run("LenCannotBeLessThanZero", func(t *testing.T) {
		t.Run("Unlimited", func(t *testing.T) {
			tracker = &queueNoLimitTrackerImpl{}
			assert.Zero(t, tracker.len())
			tracker.remove()
			assert.Zero(t, tracker.len())
		})
		t.Run("Unlimited", func(t *testing.T) {
			tracker = &queueHardLimitTracker{}
			if tracker.len() != 0 {
				t.Fatal("must start at zero")
			}
			tracker.remove()
			if tracker.len() != 0 {
				t.Fatal("must end at zero")
			}
		})
	})
}
