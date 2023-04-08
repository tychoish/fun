package router

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/testt"
)

func makeTestDispatcher(t *testing.T) *Dispatcher {
	baseNum := runtime.NumCPU()

	d := NewDispatcher(Config{Name: t.Name(), Workers: baseNum, Buffer: baseNum * baseNum})
	ctx := testt.ContextWithTimeout(t, time.Second)
	srv := d.Service()
	assert.NotError(t, srv.Start(ctx))
	d.Ready()(ctx)
	t.Cleanup(func() { srv.Close() })

	return d
}

func TestDispater(t *testing.T) {
	t.Run("DispatcherConstructorDoesNotPanic", func(t *testing.T) {
		assert.NotPanic(t, func() { NewDispatcher(Config{}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1, Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 100, Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1, Buffer: 100}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 100, Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1, Buffer: 100}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -100, Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1, Buffer: -100}) })
	})
	t.Run("RegisteringMethods", func(t *testing.T) {
		d := makeTestDispatcher(t)
		ok := d.RegisterHandler(Protocol{}, func(context.Context, Message) (Response, error) { panic("never called") })
		assert.True(t, !ok) // empty protocol
		ok = d.RegisterHandler(Protocol{Schema: "hi"}, func(context.Context, Message) (Response, error) { panic("never called") })
		assert.True(t, ok)
		ok = d.RegisterHandler(Protocol{Schema: "hi"}, func(context.Context, Message) (Response, error) { panic("really never called") })
		assert.True(t, !ok) // already registered

		d.OverrideHandler(Protocol{Schema: "hi"}, func(context.Context, Message) (Response, error) { return Response{ID: "kip"}, nil })
		hfn := d.handlers.Get(Protocol{Schema: "hi"})
		resp, err := hfn(nil, Message{})
		assert.NotError(t, err)
		assert.Equal(t, resp.ID, "kip")
	})

}
