package router

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/set"
)

func TestMessageIDs(t *testing.T) {
	t.Run("Parse", func(t *testing.T) {
		t.Run("GeneratedIDs", func(t *testing.T) {
			defer resetCounter()
			for i := 0; i < 32; i++ {
				id := GenerateID()
				ts, host, count, rs, err := id.Parse()
				check.NotError(t, err)
				check.NotZero(t, ts)
				check.NotZero(t, host)
				check.NotZero(t, count)
				check.NotZero(t, rs)
				check.True(t, len(string(id)) == 32)
				if t.Failed() {
					t.Log("failure", id)
					break
				}
			}
		})
		t.Run("Empty", func(t *testing.T) {
			id := MessageID("")
			ts, host, count, rs, err := id.Parse()
			check.Error(t, err)
			check.Zero(t, ts)
			check.Zero(t, host)
			check.Zero(t, count)
			check.Zero(t, rs)
		})
		t.Run("Incorrect", func(t *testing.T) {
			for _, str := range []string{
				"a-a-a-a",
				"1-a-a-a",
			} {
				id := MessageID(str)
				ts, host, count, rs, err := id.Parse()
				check.Error(t, err)
				check.Zero(t, ts)
				check.Zero(t, host)
				check.Zero(t, count)
				check.Zero(t, rs)
			}
		})
	})
	t.Run("StableLength", func(t *testing.T) {
		defer resetCounter()

		const iters = 100000
		seen := set.MakeUnordered[MessageID](iters)
		for i := 0; i < iters; i++ {
			id := GenerateID()
			seen.Add(id)
			size := len(id)
			check.Equal(t, size, 32)
			if t.Failed() {
				t.Logf("iter=%d, id=%q", i, id)
				break
			}
		}
		assert.Equal(t, seen.Len(), iters)
	})
	t.Run("PopulateHelper", func(t *testing.T) {
		t.Run("NonZero", func(t *testing.T) {
			for _, str := range []string{"foo", "kip", "merlin", "cat"} {
				assert.Equal(t, str, populateID(str))
			}
		})
		t.Run("Set", func(t *testing.T) {
			id := MessageID(populateID(""))
			ts, host, count, rs, err := id.Parse()
			check.NotError(t, err)
			check.NotZero(t, ts)
			check.NotZero(t, host)
			check.NotZero(t, count)
			check.NotZero(t, rs)
		})
	})
}
func TestRandomInterval(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprint("Case", i), func(t *testing.T) {

		})
	}
}
