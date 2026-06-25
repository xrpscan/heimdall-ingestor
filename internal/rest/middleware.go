package rest

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/xrpscan/heimdall-ingestor/internal/logger"
	"github.com/xrpscan/heimdall-ingestor/pkg/httputils"

	"github.com/google/uuid"
)

const (
	headerCorrelationID = "X-Correlation-ID"

	// Keys to put values into context.
	ctxKeyRequestID     = "requestID"
	ctxKeyCorrelationID = "correlationID"

	// The browser will not send the actual request after preflight if the method is not allowed.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Allow-Methods
	corsAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	// The browser will not send the actual request after preflight if it requires headers outside of this list.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Allow-Headers
	corsAllowedHeaders = "Accept, Authorization, Content-Type, " + headerCorrelationID
	// The browser javascript will be able to read only these headers.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Expose-Headers
	corsExposedHeaders = headerCorrelationID
)

// recoveryMiddleware wraps the given http.Handler with a panic recover call. This makes sure that if the app panics
// while handling a request, the error gets logged, and a sanitized 5xx is returned.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			errAny := recover()
			if errAny == nil {
				return
			}

			// Stack for debugging.
			stack := string(debug.Stack())
			slog.ErrorContext(r.Context(), "panic during request execution", "error", errAny, "stack", stack)

			// Show 500 without revealing internal reason.
			httputils.WriteError(w, httputils.InternalServerError().WithReasonStr("unknown"))
		}()

		next.ServeHTTP(w, r)
	})
}

// accessLoggerMiddleware wraps the given http.Handler with a logger that logs http request-response details, like
// method, URL, execution time (latency), and response status code.
func accessLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// This will be used to calculate the total request execution time.
		start := time.Now()

		// Respect the correlation ID sent by the user.
		correlationID := r.Header.Get(headerCorrelationID)
		if correlationID == "" {
			// If not sent, generate own.
			correlationID = uuid.NewString()
		}

		// Logger should log these IDs when invoked with the new context:
		// 1. Correlation ID
		newCtx := logger.AddContextValue(ctx, ctxKeyCorrelationID, correlationID)
		// 2. Request ID
		newCtx = logger.AddContextValue(newCtx, ctxKeyRequestID, uuid.NewString())

		// Update the request context to the new one.
		*r = *r.WithContext(newCtx)

		// Persist status-code for logging.
		cw := &httputils.ResponseWriterWithCode{ResponseWriter: w}
		// Echo correlation ID back to the client.
		cw.Header().Set(headerCorrelationID, correlationID)

		// Request entry log.
		slog.InfoContext(newCtx, "request received", "url", r.URL.String(), "method", r.Method)
		// Release control to the next middleware or handler.
		next.ServeHTTP(cw, r)
		// Request exit log.
		slog.InfoContext(newCtx, "request completed", "latency", time.Since(start), "status", cw.StatusCode)
	})
}

// corsMiddleware wraps the given http.Handler to apply a strict, browser-correct CORS policy.
// It adds CORS headers (Access-Control-XXX-XXX) to the response for allowed origins only, short-circuits preflight
// requests, and leaves non-browser clients unaffected.
//
// TODO: Trim origin values?
func corsMiddleware(next http.Handler, origins []string, maxAgeSec int) http.Handler {
	// To easily handle cases where "*" is allowed.
	allowAllOrigins := slices.Contains(origins, "*")
	// For easy lookups.
	allowedOrigins := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowedOrigins[o] = struct{}{}
	}

	// Convenience function. Makes the code below much more readable.
	isOriginAllowed := func(origin string) bool {
		if allowAllOrigins {
			return true
		}

		_, allowed := allowedOrigins[origin]
		return allowed
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// If no origin is present, process the request, but don't add CORS headers (Access-Control-XXX-XXX) to the
		// response. This means that a browser will not allow the client javascript to read the response, but cURL,
		// Postman etc. will be able to read it.
		if origin == "" {
			// If it's a Preflight request, don't let it pass to the business logic.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
			return
		}

		// If origin is not allowed:
		if !isOriginAllowed(origin) {
			// If it's a Preflight request, respond without adding CORS headers (Access-Control-XXX-XXX).
			// This will result in the browser never sending the actual request.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Same as origin not being present.
			// Process the request, but don't add CORS headers (Access-Control-XXX-XXX) to the response, so the browser
			// will not allow the client javascript to read the response.
			next.ServeHTTP(w, r)
			return
		}

		// The origin is present and allowed. Add CORS headers (Access-Control-XXX-XXX), so the browser allows the
		// client javascript to read the response.
		w.Header().Set("Access-Control-Allow-Origin", origin)
		// Using .Add instead of .Set because "Vary" is additive and must not overwrite values set by other middleware.
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Expose-Headers", corsExposedHeaders)

		// The origin is present and allowed. Respond to the Preflight request with CORS headers. So, the browser
		// proceeds to send the actual request.
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAgeSec))
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Origin is present and allowed. Process the request normally. All CORS headers are already attached.
		next.ServeHTTP(w, r)
	})
}

// bodySizeLimitMiddleware wraps the given http.Handler to apply a max read limit on the request body.
func bodySizeLimitMiddleware(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}
