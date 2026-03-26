package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPTool provides dynamic HTTP request capabilities
// It accepts connection parameters at runtime and executes requests
type HTTPTool struct{}

// NewHTTPTool creates a new HTTP tool
func NewHTTPTool() *HTTPTool {
	return &HTTPTool{}
}

func (t *HTTPTool) Name() string {
	return "http_request"
}

func (t *HTTPTool) Description() string {
	return `Execute HTTP requests to query business data from various APIs.

Supported methods: GET, POST, PUT, DELETE, PATCH

Examples:
- GET request to fetch data
- POST request to create data
- PUT request to update data
- DELETE request to remove data

Authentication types:
- none: No authentication
- basic: Basic authentication (username/password)
- bearer: Bearer token authentication
- api_key: API key in header`
}

func (t *HTTPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method: GET, POST, PUT, DELETE, PATCH",
				"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Full URL to request",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Base URL (used with path)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Request path (appended to base_url)",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "HTTP headers to send",
			},
			"query_params": map[string]any{
				"type":        "object",
				"description": "Query parameters",
			},
			"body": map[string]any{
				"description": "Request body (for POST, PUT, PATCH)",
			},
			"content_type": map[string]any{
				"type":        "string",
				"description": "Content type (default: application/json)",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Request timeout in seconds (default: 30)",
			},
			"auth": map[string]any{
				"type":        "object",
				"description": "Authentication configuration",
				"properties": map[string]any{
					"type": map[string]any{
						"type":        "string",
						"description": "Auth type: none, basic, bearer, api_key",
					},
					"username": map[string]any{
						"type":        "string",
						"description": "Username for basic auth",
					},
					"password": map[string]any{
						"type":        "string",
						"description": "Password for basic auth",
					},
					"token": map[string]any{
						"type":        "string",
						"description": "Token for bearer auth",
					},
					"api_key_name": map[string]any{
						"type":        "string",
						"description": "Header name for API key",
					},
					"api_key_value": map[string]any{
						"type":        "string",
						"description": "API key value",
					},
				},
			},
		},
		"required": []string{"method"},
	}
}

// Execute runs an HTTP request with the given parameters
func (t *HTTPTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Extract parameters
	method, _ := args["method"].(string)
	fullURL, _ := args["url"].(string)
	baseURL, _ := args["base_url"].(string)
	path, _ := args["path"].(string)
	headers, _ := args["headers"].(map[string]any)
	queryParams, _ := args["query_params"].(map[string]any)
	body := args["body"]
	contentType, _ := args["content_type"].(string)
	timeout, _ := args["timeout"].(float64)
	authArg, _ := args["auth"].(map[string]any)

	// Validate required parameters
	if method == "" {
		return ErrorResult("method is required (GET, POST, PUT, DELETE, PATCH)")
	}

	// Build URL
	if fullURL == "" && baseURL == "" {
		return ErrorResult("url or base_url is required")
	}

	if fullURL == "" {
		fullURL = baseURL + path
	}

	// Add query parameters
	if len(queryParams) > 0 {
		q := url.Values{}
		for k, v := range queryParams {
			q.Add(k, fmt.Sprintf("%v", v))
		}
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + q.Encode()
		} else {
			fullURL += "?" + q.Encode()
		}
	}

	// Create request body
	var reqBody io.Reader
	if body != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		ct := contentType
		if ct == "" {
			ct = "application/json"
		}

		var bodyBytes []byte
		switch b := body.(type) {
		case string:
			bodyBytes = []byte(b)
		case []byte:
			bodyBytes = b
		default:
			var err error
			bodyBytes, err = json.Marshal(b)
			if err != nil {
				return ErrorResult(fmt.Sprintf("failed to marshal body: %v", err))
			}
		}
		reqBody = strings.NewReader(string(bodyBytes))

		// Add content type to headers
		if headers == nil {
			headers = make(map[string]any)
		}
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = ct
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	// Add headers
	for k, v := range headers {
		httpReq.Header.Set(k, fmt.Sprintf("%v", v))
	}

	// Add authentication
	if authArg != nil {
		addAuth(httpReq, authArg)
	}

	// Set timeout
	client := &http.Client{}
	if timeout > 0 {
		client.Timeout = time.Duration(timeout) * time.Second
	} else {
		client.Timeout = 30 * time.Second
	}

	// Execute request
	resp, err := client.Do(httpReq)
	if err != nil {
		return ErrorResult(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read response: %v", err))
	}

	// Parse response headers
	respHeaders := make(map[string]any)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	// Try to parse JSON body
	var parsedBody any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsedBody); err != nil {
			// If not JSON, return as string
			parsedBody = string(respBody)
		}
	}

	// Format result
	result := map[string]any{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     respHeaders,
		"body":        parsedBody,
	}

	jsonRes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to marshal result: %v", err))
	}

	return &ToolResult{
		ForLLM: string(jsonRes),
	}
}

// addAuth adds authentication to the request
func addAuth(req *http.Request, auth map[string]any) {
	authType, _ := auth["type"].(string)

	switch authType {
	case "basic":
		username, _ := auth["username"].(string)
		password, _ := auth["password"].(string)
		req.SetBasicAuth(username, password)
	case "bearer":
		token, _ := auth["token"].(string)
		req.Header.Set("Authorization", "Bearer "+token)
	case "api_key":
		apiKeyName, _ := auth["api_key_name"].(string)
		apiKeyValue, _ := auth["api_key_value"].(string)
		if apiKeyName != "" && apiKeyValue != "" {
			req.Header.Set(apiKeyName, apiKeyValue)
		}
	}
}
