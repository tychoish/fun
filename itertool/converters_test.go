package itertool

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestConverters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Map", func(t *testing.T) {
		in := map[string]string{
			"hi":  "there",
			"how": "are you doing",
		}
		iter := FromMap(in)
		seen := 0
		for iter.Next(ctx) {
			item := iter.Value()
			switch {
			case item.Key == "hi":
				check.Equal(t, item.Value, "there")
			case item.Key == "how":
				check.Equal(t, item.Value, "are you doing")
			default:
				t.Errorf("unexpected value: %s", item)
			}
			seen++
		}
		assert.Equal(t, seen, len(in))
	})

}
