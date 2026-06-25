package httputils

import (
	"errors"
	"net/http"
)

// Error represents http errors, and implements the error interface.
type Error struct {
	StatusCode int    `json:"-"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

// Error provides the reason behind the error, which is usually human-readable.
// If the reason is absent, it provides the error status (example: CONFLICT) instead.
func (e *Error) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	// Returning status if reason is empty.
	return e.Status
}

// WithReasonStr is a chainable method to set the reason of the Error.
//
// This accepts the reason as a string.
func (e *Error) WithReasonStr(reason string) *Error {
	e.Reason = reason
	return e
}

// WithReasonErr is a chainable method to set the reason of the Error.
//
// This accepts the reason as an error.
func (e *Error) WithReasonErr(reason error) *Error {
	e.Reason = reason.Error()
	return e
}

// ToError converts any value to an appropriate Error.
func ToError(err any) *Error {
	switch asserted := err.(type) {
	case *Error:
		return asserted
	case Error:
		return &asserted
	case error:
		var errHTTP *Error
		if errors.As(asserted, &errHTTP) {
			return errHTTP
		}
		return InternalServerError().WithReasonErr(asserted)
	case string:
		return InternalServerError().WithReasonStr(asserted)
	default:
		return InternalServerError()
	}
}

// NewError returns the Error instance for the code. It derives the Status value from the code too.
func NewError(code int) *Error {
	statusText := http.StatusText(code) // Example: 400 -> Bad Request
	return &Error{StatusCode: code, Status: statusText}
}

func BadRequest() *Error          { return NewError(http.StatusBadRequest) }
func Unauthorized() *Error        { return NewError(http.StatusUnauthorized) }
func PaymentRequired() *Error     { return NewError(http.StatusPaymentRequired) }
func Forbidden() *Error           { return NewError(http.StatusForbidden) }
func NotFound() *Error            { return NewError(http.StatusNotFound) }
func RequestTimeout() *Error      { return NewError(http.StatusRequestTimeout) }
func Conflict() *Error            { return NewError(http.StatusConflict) }
func PreconditionFailed() *Error  { return NewError(http.StatusPreconditionFailed) }
func InternalServerError() *Error { return NewError(http.StatusInternalServerError) }
func ServiceUnavailable() *Error  { return NewError(http.StatusServiceUnavailable) }
