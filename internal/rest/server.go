package rest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Server represents the http server of the app.
type Server struct {
	httpServer *http.Server
}

// NewServer returns a new [Server] instance.
func NewServer(baseContext context.Context, addr string, handler http.Handler) *Server {
	httpServer := &http.Server{
		BaseContext: func(_ net.Listener) context.Context { return baseContext },
		Addr:        addr,
		Handler:     handler,
		// Max time to read request headers. Defends against slowloris attacks.
		ReadHeaderTimeout: 5 * time.Second,
		// Max time from connection accept to full request body read.
		ReadTimeout: 5 * time.Second,
		// Max time from request header read to response write completion.
		WriteTimeout: 10 * time.Second,
		// Max time a keep-alive connection can sit idle between requests.
		IdleTimeout: 60 * time.Second,
		// Max size of request headers.
		MaxHeaderBytes: 8 * 1024, // 8 KB
	}

	return &Server{httpServer: httpServer}
}

// ListenAndServe simply calls ListenAndServe on the underlying http server. In case of an error,
// it swallows [http.ErrServerClosed], any other error is returned as is.
func (s *Server) ListenAndServe() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// No need for fmt.Errorf here.
		return err
	}

	return nil
}

// Close implements the Closer interface of the registry package.
func (s *Server) Close(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
