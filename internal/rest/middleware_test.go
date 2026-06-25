package rest

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/xrpscan/heimdall-ingestor/internal/logger"
	"github.com/xrpscan/heimdall-ingestor/pkg/httputils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMiddleware(t *testing.T) {
	// Mock next handler that panics.
	mockNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("mock error"))
	})

	// Mock request, response.
	request := httptest.NewRequest(http.MethodGet, "https://ingestor.shivansh.io", nil)
	recorder := httptest.NewRecorder()

	// Invoke the middleware.
	handler := recoveryMiddleware(mockNext)
	handler.ServeHTTP(recorder, request)

	// Verify response status code to be 5xx.
	require.Equal(t, httputils.InternalServerError().StatusCode, recorder.Code)

	// Decode the response body for verification.
	responseBody := map[string]any{}
	err := json.NewDecoder(recorder.Body).Decode(&responseBody)
	require.NoError(t, err)

	// Verify response body.
	require.Equal(t, httputils.InternalServerError().Status, responseBody["status"])
	require.Equal(t, "unknown", responseBody["reason"])
}

func TestAccessLoggerMiddleware(t *testing.T) {
	// This test cannot run in parallel because it relies on the global logger object.

	// Temporary file details for testing.
	tempDir, fileName := t.TempDir(), uuid.NewString()+".log"
	filePath := filepath.Join(tempDir, fileName)

	closer := logger.Init(filePath, "info", true)
	defer func() { _ = closer() }()

	// Mock next handler.
	expectedStatusCode := http.StatusOK
	mockNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	})

	// Mock request, response.
	request := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	recorder := httptest.NewRecorder()

	// Invoke the middleware.
	handler := accessLoggerMiddleware(mockNext)
	handler.ServeHTTP(recorder, request)

	// Fetch context-info to verify if it was set correctly by the middleware.
	ctxInfo := logger.GetContextValues(request.Context())

	// Verify if the request ID was initialized.
	requestID, exists := ctxInfo[ctxKeyRequestID]
	require.True(t, exists)

	// Request ID should be valid uuid.
	_, err := uuid.Parse(requestID.String())
	require.NoError(t, err)

	// Expect the correct response code.
	require.Equal(t, expectedStatusCode, recorder.Code)

	// Open the file again for verification.
	tmpFileOpened, err := os.Open(filePath)
	require.NoError(t, err)
	defer func() { require.NoError(t, tmpFileOpened.Close()) }()

	// Count number of log statements printed.
	var actualLogCount int
	for scanner := bufio.NewScanner(tmpFileOpened); scanner.Scan(); {
		actualLogCount++
	}

	// Verify number of log statements.
	require.Equal(t, 2, actualLogCount)
}

func TestCorsMiddleware(t *testing.T) {
	mockOrigin := "https://ingestor.shivansh.io"
	mockMaxAgeSec := 86400

	testCases := []struct {
		name string

		allowedOrigins     []string
		inputRequestOrigin string
		inputRequestMethod string

		expectNextCall          bool
		expectedResponseCode    int
		expectedResponseHeaders map[string][]string
	}{
		{
			name:                    "Preflight request, no origin",
			allowedOrigins:          []string{mockOrigin},
			inputRequestOrigin:      "",
			inputRequestMethod:      http.MethodOptions,
			expectNextCall:          false,
			expectedResponseCode:    http.StatusNoContent,
			expectedResponseHeaders: map[string][]string{},
		},
		{
			name:                    "Preflight request, unknown origin",
			allowedOrigins:          []string{mockOrigin},
			inputRequestOrigin:      mockOrigin + ".something",
			inputRequestMethod:      http.MethodOptions,
			expectNextCall:          false,
			expectedResponseCode:    http.StatusNoContent,
			expectedResponseHeaders: map[string][]string{},
		},
		{
			name:                 "Preflight request, known origin",
			allowedOrigins:       []string{mockOrigin},
			inputRequestOrigin:   mockOrigin,
			inputRequestMethod:   http.MethodOptions,
			expectNextCall:       false,
			expectedResponseCode: http.StatusNoContent,
			expectedResponseHeaders: map[string][]string{
				"Access-Control-Allow-Origin":   {mockOrigin},
				"Vary":                          {"Origin"},
				"Access-Control-Expose-Headers": {corsExposedHeaders},
				"Access-Control-Allow-Methods":  {corsAllowedMethods},
				"Access-Control-Allow-Headers":  {corsAllowedHeaders},
				"Access-Control-Max-Age":        {strconv.Itoa(mockMaxAgeSec)},
			},
		},
		{
			name:                 "Preflight request, all origins allowed",
			allowedOrigins:       []string{"*"},
			inputRequestOrigin:   mockOrigin,
			inputRequestMethod:   http.MethodOptions,
			expectNextCall:       false,
			expectedResponseCode: http.StatusNoContent,
			expectedResponseHeaders: map[string][]string{
				"Access-Control-Allow-Origin":   {mockOrigin},
				"Vary":                          {"Origin"},
				"Access-Control-Expose-Headers": {corsExposedHeaders},
				"Access-Control-Allow-Methods":  {corsAllowedMethods},
				"Access-Control-Allow-Headers":  {corsAllowedHeaders},
				"Access-Control-Max-Age":        {strconv.Itoa(mockMaxAgeSec)},
			},
		},
		{
			name:                    "Actual request, no origin",
			allowedOrigins:          []string{mockOrigin},
			inputRequestOrigin:      "",
			inputRequestMethod:      http.MethodGet,
			expectNextCall:          true,
			expectedResponseCode:    http.StatusOK,
			expectedResponseHeaders: map[string][]string{},
		},
		{
			name:                    "Actual request, unknown origin",
			allowedOrigins:          []string{mockOrigin},
			inputRequestOrigin:      mockOrigin + ".something",
			inputRequestMethod:      http.MethodGet,
			expectNextCall:          true,
			expectedResponseCode:    http.StatusOK,
			expectedResponseHeaders: map[string][]string{},
		},
		{
			name:                 "Actual request, known origin",
			allowedOrigins:       []string{mockOrigin},
			inputRequestOrigin:   mockOrigin,
			inputRequestMethod:   http.MethodGet,
			expectNextCall:       true,
			expectedResponseCode: http.StatusOK,
			expectedResponseHeaders: map[string][]string{
				"Access-Control-Allow-Origin":   {mockOrigin},
				"Vary":                          {"Origin"},
				"Access-Control-Expose-Headers": {corsExposedHeaders},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Track if the next handler was called - for verification.
			var nextCalled bool
			mockNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Mock request, response.
			request := httptest.NewRequest(tc.inputRequestMethod, "https://ingestor.shivansh.io", nil)
			request.Header.Set("Origin", tc.inputRequestOrigin)
			recorder := httptest.NewRecorder()

			// Invoke the middleware.
			handler := corsMiddleware(mockNext, tc.allowedOrigins, mockMaxAgeSec)
			handler.ServeHTTP(recorder, request)

			// Verify flow and response.
			require.Equal(t, tc.expectNextCall, nextCalled)
			require.Equal(t, tc.expectedResponseCode, recorder.Code)
			require.Equal(t, http.Header(tc.expectedResponseHeaders), recorder.Result().Header)
		})
	}
}
