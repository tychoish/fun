package pubsub

import (
	"math"
	"testing"
)

func TestTracker(t *testing.T) {
	// mostly just confirmation/consistency testing
	var tracker queueLimitTracker

	t.Run("NoLimitIsUnlimted", func(t *testing.T) {
		tracker = &queueNoLimitTrackerImpl{}
		if tracker.cap() != math.MaxInt {
			t.Fatal("unlimited should basically be unlimited")
		}
	})
	t.Run("LenCannotBeLessThanZero", func(t *testing.T) {
		t.Run("Unlimited", func(t *testing.T) {
			tracker = &queueNoLimitTrackerImpl{}
			if tracker.len() != 0 {
				t.Fatal("must start at zero")
			}
			tracker.remove()
			if tracker.len() != 0 {
				t.Fatal("must end at zero")
			}
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
