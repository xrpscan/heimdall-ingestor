package xrpld

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Interface to represent an xrpld client abstraction.
type Interface interface {
	GetManifest(ctx context.Context, validatorPublicKey string) (ManifestDetails, error)
}

// Client implements [Interface].
type Client struct {
	httpClient *http.Client

	addr   string
	logger Logger
}

// NewClient returns a new instance of Client.
func NewClient(addr string, logger Logger) *Client {
	if logger == nil {
		logger = noopLogger{}
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	return &Client{httpClient: httpClient, addr: addr, logger: logger}
}

func (c *Client) GetManifest(
	ctx context.Context, validatorPublicKey string,
) (ManifestDetails, error) {
	command := commandRequest{
		Method: "manifest",
		Params: []map[string]any{{"public_key": validatorPublicKey}},
	}

	// Marshal the body for the http request.
	body, err := json.Marshal(command)
	if err != nil {
		return ManifestDetails{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Form the http request.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr, bytes.NewReader(body))
	if err != nil {
		return ManifestDetails{}, fmt.Errorf("failed to create http request: %w", err)
	}
	request.Header.Add("Content-Type", "application/json")

	// Execute request.
	response, err := c.httpClient.Do(request)
	if err != nil {
		return ManifestDetails{}, fmt.Errorf("failed to execute http request: %w", err)
	}

	// Cleanup.
	defer func() { _ = response.Body.Close() }()

	// Read the body as we'll have to unmarshal it multiple times.
	// Here, io.LimitReader ensures that a maliciously large response
	// body is not loaded into memory. 1<<20 represents 1 MB.
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return ManifestDetails{}, fmt.Errorf("failed to read response body: %w", err)
	}
	responseBodyStr := string(responseBody)

	// If not 2xx code.
	if response.StatusCode/100 != 2 {
		c.logger.ErrorContext(ctx, "xrpld returned non-2xx status code",
			"code", response.StatusCode, "body", stringUpto(responseBodyStr, 1024))

		return ManifestDetails{},
			fmt.Errorf("xrpld returned non-2xx status code: %d", response.StatusCode)
	}

	// Decode the body.
	result, err := processManifestCommandResponse(responseBody)
	if err != nil {
		// Log the response body to help with debugging.
		// 1<<10 represents 1 KB.
		c.logger.ErrorContext(ctx, "failed to process response body",
			"body", stringUpto(responseBodyStr, 1<<10))

		return ManifestDetails{}, fmt.Errorf("failed to process response body: %w", err)
	}

	return result, nil
}

// processManifestCommandResponse judges if the given body represents a success or error response
// and decodes it accordingly.
func processManifestCommandResponse(body []byte) (ManifestDetails, error) {
	var onlyStatus struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}

	// Get just the status to judge whether the response represents success or error.
	// Unmarshalling will be different for both.
	if err := json.Unmarshal(body, &onlyStatus); err != nil {
		return ManifestDetails{}, fmt.Errorf("failed to read status: %w", err)
	}

	switch onlyStatus.Result.Status {
	case "success":
		var resp manifestCommandResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return ManifestDetails{},
				fmt.Errorf("failed to decode success response body: %w", err)
		}
		return resp.Result.Details, nil
	case "error":
		var resp commandErrorResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return ManifestDetails{}, fmt.Errorf("failed to decode error response body: %w", err)
		}

		result := resp.Result
		return ManifestDetails{},
			fmt.Errorf("xrpld returned error: %s, %s", result.Error, result.ErrorMessage)
	default:
		return ManifestDetails{}, fmt.Errorf("unknown response status: %s", onlyStatus.Result.Status)
	}
}
