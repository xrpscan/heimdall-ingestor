package httputils

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// WriteJson marshals the given body to JSON, and writes it as the http response.
// It also sets the given status code and headers.
//
// Since the response is assumed to be JSON, the content-type header is set to "application/json".
//
// Note that the role of this function is to make response writing convenient.
// So, in case of any error, this function fails silently. For example, if the given body is not
// a valid JSON, or if JSON marshalling fails due to any other reason, empty JSON "{}" is written
// to the writer with the given status code and headers.
func WriteJson(writer http.ResponseWriter, status int, headers map[string]string, body any) {
	// The content-type is application/json for all cases.
	writer.Header().Set("content-type", "application/json")
	// Setting the provided headers.
	for key, value := range headers {
		writer.Header().Set(key, value)
	}

	// Converting the provided body to a byte slice for writing.
	responseBytes, err := json.Marshal(body)
	if err != nil {
		responseBytes = []byte(`{}`)
		slog.Error("failed to marshal body", "error", err)
	}

	// Setting the content-length header.
	writer.Header().Set("content-length", fmt.Sprintf("%d", len(responseBytes)))

	// Setting the status code. No more headers can be set after this.
	writer.WriteHeader(status)
	// Writing the body to the response.
	_, _ = writer.Write(responseBytes)
}

// WriteError attempts to convert the given error to the Error type, and then writes it as JSON.
func WriteError(writer http.ResponseWriter, err error) {
	errHTTP := ToError(err)
	WriteJson(writer, errHTTP.StatusCode, nil, errHTTP)
}
