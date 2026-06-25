package registry

import (
	"context"
)

// Closer represents any service/process that can be closed.
type Closer interface {
	Close(ctx context.Context) error
}

// CloserFunc allows types to easily implement the Closer interface.
type CloserFunc func(ctx context.Context) error

func (c CloserFunc) Close(ctx context.Context) error { return c(ctx) }

// Logger for the package's internal logging.
type Logger interface {
	Info(msg string, args ...any)
}

// noopLogger implements Logger as a no-op.
type noopLogger struct{}

func (n noopLogger) Info(msg string, args ...any) {}
