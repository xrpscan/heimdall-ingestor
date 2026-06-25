package httputils

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_ErrorMethod(t *testing.T) {
	t.Parallel()

	t.Run("returns reason when set", func(t *testing.T) {
		t.Parallel()
		e := BadRequest().WithReasonStr("invalid email")
		require.Equal(t, "invalid email", e.Error())
	})

	t.Run("returns status when reason is empty", func(t *testing.T) {
		t.Parallel()
		e := BadRequest()
		require.Equal(t, "Bad Request", e.Error())
	})
}

func TestError_WithReasonStr(t *testing.T) {
	t.Parallel()

	e := BadRequest()
	ret := e.WithReasonStr("bad input")

	require.Equal(t, "bad input", e.Reason)
	require.Same(t, e, ret)
}

func TestError_WithReasonErr(t *testing.T) {
	t.Parallel()

	e := BadRequest()
	ret := e.WithReasonErr(errors.New("something broke"))

	require.Equal(t, "something broke", e.Reason)
	require.Same(t, e, ret)
}

func TestToError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		input              any
		expectedStatusCode int
		expectedReason     string
	}{
		{
			name:               "*Error passthrough",
			input:              NotFound().WithReasonStr("no such user"),
			expectedStatusCode: http.StatusNotFound,
			expectedReason:     "no such user",
		},
		{
			name:               "Error value",
			input:              Error{StatusCode: http.StatusConflict, Status: "Conflict", Reason: "duplicate"},
			expectedStatusCode: http.StatusConflict,
			expectedReason:     "duplicate",
		},
		{
			name:               "error wrapping *Error",
			input:              fmt.Errorf("wrapped: %w", Forbidden().WithReasonStr("denied")),
			expectedStatusCode: http.StatusForbidden,
			expectedReason:     "denied",
		},
		{
			name:               "plain error",
			input:              errors.New("something failed"),
			expectedStatusCode: http.StatusInternalServerError,
			expectedReason:     "something failed",
		},
		{
			name:               "string",
			input:              "bad things happened",
			expectedStatusCode: http.StatusInternalServerError,
			expectedReason:     "bad things happened",
		},
		{
			name:               "unknown type",
			input:              42,
			expectedStatusCode: http.StatusInternalServerError,
			expectedReason:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := ToError(tc.input)
			require.Equal(t, tc.expectedStatusCode, result.StatusCode)
			require.Equal(t, tc.expectedReason, result.Reason)
		})
	}
}

func TestNewError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code           int
		expectedStatus string
	}{
		{http.StatusBadRequest, "Bad Request"},
		{http.StatusNotFound, "Not Found"},
		{http.StatusInternalServerError, "Internal Server Error"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedStatus, func(t *testing.T) {
			t.Parallel()

			e := NewError(tc.code)
			require.Equal(t, tc.code, e.StatusCode)
			require.Equal(t, tc.expectedStatus, e.Status)
			require.Empty(t, e.Reason)
		})
	}
}
