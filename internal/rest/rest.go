package rest

import (
	"net/http"

	"github.com/xrpscan/heimdall-ingestor/internal/config"
	"github.com/xrpscan/heimdall-ingestor/pkg/httputils"
)

// maxBodyReadBytes is the max size that a request body is allowed to have.
const maxBodyReadBytes = 16 * 1024 // 16 KB

// Handler encapsulates all REST API handlers.
//
// It implements the http.Handler interface for convenient usage with an http.Server.
type Handler struct {
	underlying http.Handler
}

// NewHandler returns a new Handler instance.
func NewHandler(conf config.Config) *Handler {
	handler := &Handler{}

	handler.addRoutes()
	handler.addMiddleware(conf)
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.underlying.ServeHTTP(w, r)
}

// addRoutes instantiates the underlying handler and attaches all REST routes to it.
func (h *Handler) addRoutes() {
	// A ServeMux will act as the underlying http.Handler.
	mux := http.NewServeMux()
	h.underlying = mux

	// Status check API.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httputils.WriteJson(w, http.StatusOK, nil, map[string]any{"code": "OK"})
	})
}

// addMiddleware wraps the underlying handler with all the middleware.
func (h *Handler) addMiddleware(conf config.Config) {
	// Middleware attachments. This order is opposite to the execution order.
	next := bodySizeLimitMiddleware(h.underlying, maxBodyReadBytes)
	next = corsMiddleware(next, conf.HttpServer.AllowedOrigins, conf.HttpServer.CorsMaxAgeSec)
	next = accessLoggerMiddleware(next)
	next = recoveryMiddleware(next) // <- This will execute first.

	h.underlying = next
}
