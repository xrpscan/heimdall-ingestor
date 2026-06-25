package httputils

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ResponseWriterWithCode is a wrapper for http.ResponseWriter that also persists statusCode.
type ResponseWriterWithCode struct {
	http.ResponseWriter
	StatusCode int
}

func (r *ResponseWriterWithCode) Write(b []byte) (int, error) {
	if r.StatusCode == 0 {
		r.StatusCode = http.StatusOK
	}

	return r.ResponseWriter.Write(b)
}

func (r *ResponseWriterWithCode) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Flush forwards http.Flusher when supported (important for SSE/streaming).
func (r *ResponseWriterWithCode) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack method belongs to the http.Hijacker interface. It is necessary when working with websockets.
func (r *ResponseWriterWithCode) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// Get the underlying hijacker interface.
	hijacker, asserted := r.ResponseWriter.(http.Hijacker)
	if !asserted {
		return nil, nil, errors.New("hijack not supported")
	}

	// Call the underlying hijacker.
	conn, readWriter, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, fmt.Errorf("error in wrapped hijacker: %w", err)
	}

	return conn, readWriter, nil
}
