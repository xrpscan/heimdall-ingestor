package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Init creates a new slog logger and sets it as the default one.
//
// Inputs:
//  1. filePath:	Path to the log file. If empty, it logs to stdout.
//  2. level:		Logger level. Should be one of "debug", "info", "warn" and "error".
//  3. pretty:		If true, logs will follow key=value format, otherwise JSON format.
//
// Outputs:
//  1. closer:		A function that closes the file descriptor if file-logging was enabled.
func Init(filePath, level string, pretty bool) func() error {
	var slogLevel slog.Level

	// Convert the given log-level to slog.Level case-insensitively.
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		panic("unknown log level provided: " + level)
	}

	// Set logger level.
	options := &slog.HandlerOptions{AddSource: true, Level: slogLevel}

	// Get destination (file or stdout)
	destination := getDestination(filePath)

	var handler slog.Handler
	if pretty {
		handler = slog.NewTextHandler(destination, options)
	} else {
		handler = slog.NewJSONHandler(destination, options)
	}

	handler = ContextHandler{Handler: handler}
	slog.SetDefault(slog.New(handler))

	return func() error { return destination.Close() }
}

func getDestination(filePath string) io.WriteCloser {
	if strings.TrimSpace(filePath) == "" {
		// Defend os.Stdout from being closed.
		return nopCloser{os.Stdout}
	}

	return &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    50,    // Megabytes.
		MaxBackups: 0,     // Never delete old log files based on number.
		MaxAge:     0,     // Never delete old log files based on age.
		Compress:   false, // Don't compress old log files.
	}
}

// nopCloser for an io.Writer.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
