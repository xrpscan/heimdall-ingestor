package logger

import (
	"context"
	"log/slog"
)

type contextKey int

// ctxKey is used to put values into a context that are intended to be logged.
const ctxKey contextKey = iota

// ContextHandler is a custom slog.Handler implementation that logs the values present in the context.
type ContextHandler struct {
	slog.Handler
}

// Handle is supposed to be called by slog internally.
func (c ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(ctxKey).([]slog.Attr); ok {
		r.AddAttrs(attrs...)
	}
	return c.Handler.Handle(ctx, r)
}

// WithAttrs is supposed to be called by slog internally.
func (c ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{Handler: c.Handler.WithAttrs(attrs)}
}

// WithGroup is supposed to be called by slog internally.
func (c ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{Handler: c.Handler.WithGroup(name)}
}

// AddContextValue returns a new context that includes the given value.
//
// Any slog statements logged using the returned context will log this value.
func AddContextValue(parent context.Context, key string, value any) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	// Make slog.Attr from key-value.
	attr := slog.Any(key, value)

	// The child context will have the parent's slog attributes too.
	if v, ok := parent.Value(ctxKey).([]slog.Attr); ok {
		// Copy before mutating, so the parent context remains unchanged.
		vCopy := make([]slog.Attr, 0, len(v)+1)
		vCopy = append(vCopy, v...)
		vCopy = append(vCopy, attr)
		return context.WithValue(parent, ctxKey, vCopy)
	}

	return context.WithValue(parent, ctxKey, []slog.Attr{attr})
}

// GetContextValues returns all the key-value pairs that were put in the given context by the AddContextValue function.
func GetContextValues(ctx context.Context) map[string]slog.Value {
	v, ok := ctx.Value(ctxKey).([]slog.Attr)
	if !ok {
		return nil
	}

	m := map[string]slog.Value{}
	for _, entry := range v {
		m[entry.Key] = entry.Value
	}

	return m
}
