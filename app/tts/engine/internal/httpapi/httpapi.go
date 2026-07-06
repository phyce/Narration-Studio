// Package httpapi centralizes the JSON-over-HTTP request scaffolding shared by
// the API-backed TTS engines (ElevenLabs, OpenAI, Gemini). Each engine used to
// hand-roll the same marshal/new-request/set-headers/do/close/status-check/read
// sequence; Do collapses that into one place with a shared, timeout-bounded
// client.
package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nstudio/app/common/response"
)

// DefaultTimeout bounds each HTTP call so a hung endpoint cannot block a pooled
// engine instance forever.
const DefaultTimeout = 30 * time.Second

var client = &http.Client{Timeout: DefaultTimeout}

// Request describes a single HTTP request with an optional JSON body.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	// Body, when non-nil, is JSON-encoded and sent as the request body. The
	// caller is responsible for setting the Content-Type header.
	Body any
}

// Do performs req and returns the response body on a 2xx status. On a non-2xx
// status it returns an error containing the status code and response body. All
// returned errors are already routed through response.Err (logged), so callers
// can return them directly without re-wrapping.
func Do(req Request) ([]byte, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		encoded, err := json.Marshal(req.Body)
		if err != nil {
			return nil, response.Err(fmt.Errorf("failed to marshal request body: %w", err))
		}
		bodyReader = bytes.NewReader(encoded)
	}

	httpRequest, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to create HTTP request: %w", err))
	}

	for key, value := range req.Headers {
		httpRequest.Header.Set(key, value)
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to send HTTP request: %w", err))
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to read response body: %w", err))
	}

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, response.Err(fmt.Errorf("request failed with status %d: %s", httpResponse.StatusCode, string(body)))
	}

	return body, nil
}
