// Package srv provides a framework for service orchestration.
package srv

import "context"

type ctxKey struct{}

// WithContext attaches an orchestrator, if one is not already
// attached to the context and returns a replacement context.
func WithContext(ctx context.Context, or *Orchestrator) context.Context {
	if existing, ok := ctx.Value(ctxKey{}).(*Orchestrator); ok {
		if or == existing {
			return ctx
		}
		panic("multiple orchestrators")
	}
	return context.WithValue(ctx, ctxKey{}, or)
}

// FromContext resolves the Orchestrator attached to the context or
// panics if there is no orchestrator attached to the context.
func FromContext(ctx context.Context) *Orchestrator {
	if or, ok := ctx.Value(ctxKey{}).(*Orchestrator); ok {
		return or
	}

	panic("nil orchestrator")
}
