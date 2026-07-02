package xrpld

import (
	"context"
)

// commandRequest is the schema of the request that xrpld accepts for a command execution.
type commandRequest struct {
	Method string           `json:"method"`
	Params []map[string]any `json:"params"`
}

// commandErrorResponse is the schema of an error response for an xrpld command execution.
type commandErrorResponse struct {
	Result struct {
		Error        string `json:"error"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		Status       string `json:"status"`
	} `json:"result"`
}

// manifestCommandResponse is the schema of a success response for an xrpld "manifest" command.
type manifestCommandResponse struct {
	Result struct {
		Details   ManifestDetails `json:"details"`
		Manifest  string          `json:"manifest"`
		Requested string          `json:"requested"`
		Status    string          `json:"status"`
	} `json:"result"`
}

// ManifestDetails is the schema of the "details" field in the "manifest" command result of xrpld.
type ManifestDetails struct {
	Domain       string `json:"domain"`
	EphemeralKey string `json:"ephemeral_key"`
	MasterKey    string `json:"master_key"`
	Seq          int    `json:"seq"`
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
