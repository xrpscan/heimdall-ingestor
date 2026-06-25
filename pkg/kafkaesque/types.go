package kafkaesque

// Logger for the package's internal logging.
type Logger interface {
	Info(msg string, args ...any)
}

// noopLogger implements Logger as a no-op.
type noopLogger struct{}

func (n noopLogger) Info(msg string, args ...any) {}
