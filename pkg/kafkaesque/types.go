package kafkaesque

import (
	"context"
)

// RecordHandlerFunc represents a function that processes a Kafka message/record.
type RecordHandlerFunc func(ctx context.Context, record Record) error

// Record represents a Kafka message/record.
type Record struct {
	Payload []byte
	Headers map[string]string
}

// Logger for the package's internal logging.
type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// noopLogger implements Logger as a no-op.
type noopLogger struct{}

func (n noopLogger) DebugContext(ctx context.Context, msg string, args ...any) {}
func (n noopLogger) InfoContext(ctx context.Context, msg string, args ...any)  {}
func (n noopLogger) ErrorContext(ctx context.Context, msg string, args ...any) {}
