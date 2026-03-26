package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPProvider implements HTTPProvider
type HTTPProvider struct {
	client  *http.Client
	baseURL string
	headers map[string]string
	auth    *AuthConfig
}

// NewHTTPProvider creates a new HTTP provider
func NewHTTPProvider() *HTTPProvider {
	return &HTTPProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Connect connects to the HTTP server with the given configuration
func (p *HTTPProvider) Connect(ctx context.Context, config *HTTPConfig) error {
	if config == nil {
		return fmt.Errorf("HTTP config is required")
	}

	p.baseURL = config.BaseURL
	p.headers = config.Headers
	p.auth = config.Auth

	if config.Timeout > 0 {
		p.client.Timeout = time.Duration(config.Timeout) * time.Second
	}

	return nil
}

// Close closes the HTTP client
func (p *HTTPProvider) Close() error {
	return nil
}

// IsConnected returns true if the client is configured
func (p *HTTPProvider) IsConnected() bool {
	return p.baseURL != ""
}

// Execute executes an HTTP request
func (p *HTTPProvider) Execute(ctx context.Context, req *Request) (*QueryResult, error) {
	start := time.Now()

	// Build URL
	uri := p.baseURL + req.Path
	if len(req.QueryParams) > 0 {
		q := url.Values{}
		for k, v := range req.QueryParams {
			q.Add(k, v)
		}
		if strings.Contains(uri, "?") {
			uri += "&" + q.Encode()
		} else {
			uri += "?" + q.Encode()
		}
	}

	// Create request body
	var body io.Reader
	if req.Body != nil {
		contentType := req.ContentType
		if contentType == "" {
			contentType = "application/json"
		}

		var bodyBytes []byte
		switch b := req.Body.(type) {
		case string:
			bodyBytes = []byte(b)
		case []byte:
			bodyBytes = b
		default:
			var err error
			bodyBytes, err = json.Marshal(b)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
		}
		body = bytes.NewReader(bodyBytes)

		// Add content type header
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		if _, ok := req.Headers["Content-Type"]; !ok {
			req.Headers["Content-Type"] = contentType
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, uri, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add default headers
	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	// Add request-specific headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Add authentication
	p.addAuth(httpReq)

	// Execute request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response headers
	headers := make(map[string]any)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
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

	duration := time.Since(start).Milliseconds()

	return &QueryResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    headers,
		Body:       parsedBody,
		Duration:   duration,
	}, nil
}

// addAuth adds authentication to the request
func (p *HTTPProvider) addAuth(req *http.Request) {
	if p.auth == nil {
		return
	}

	switch p.auth.Type {
	case "basic":
		req.SetBasicAuth(p.auth.Username, p.auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+p.auth.Token)
	case "api_key":
		if p.auth.APIKeyName != "" && p.auth.APIKeyValue != "" {
			req.Header.Set(p.auth.APIKeyName, p.auth.APIKeyValue)
		}
	}
}

// HTTPConfig holds the configuration for HTTP server connections
type HTTPConfig struct {
	// BaseURL is the base URL for all requests
	BaseURL string `json:"base_url"`
	// Headers are the default headers to send with each request
	Headers map[string]string `json:"headers,omitempty"`
	// Auth is the authentication configuration
	Auth *AuthConfig `json:"auth,omitempty"`
	// Timeout is the request timeout in seconds
	Timeout int `json:"timeout"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Type is the authentication type: none, basic, bearer, api_key
	Type string `json:"type"`
	// Username for basic auth
	Username string `json:"username,omitempty"`
	// Password for basic auth
	Password string `json:"password,omitempty"`
	// Token for bearer auth
	Token string `json:"token,omitempty"`
	// APIKeyName is the header name for API key auth
	APIKeyName string `json:"api_key_name,omitempty"`
	// APIKeyValue is the API key value
	APIKeyValue string `json:"api_key_value,omitempty"`
}

// Request represents an HTTP request
type Request struct {
	// Method is the HTTP method: GET, POST, PUT, DELETE, PATCH
	Method string `json:"method"`
	// Path is the request path (appended to BaseURL)
	Path string `json:"path"`
	// Headers are additional headers for this request
	Headers map[string]string `json:"headers,omitempty"`
	// QueryParams are query parameters
	QueryParams map[string]string `json:"query_params,omitempty"`
	// Body is the request body (for POST, PUT, PATCH)
	Body any `json:"body,omitempty"`
	// ContentType is the content type of the request
	ContentType string `json:"content_type,omitempty"`
}

// QueryResult represents the result of an HTTP request
type QueryResult struct {
	StatusCode int             `json:"status_code"`
	Status     string         `json:"status"`
	Headers    map[string]any `json:"headers"`
	Body       any            `json:"body"`
	Duration   int64          `json:"duration_ms"`
}

// ServerConfig holds command line configuration
type ServerConfig struct {
	BaseURL string            `json:"base_url"`
	Headers map[string]string `json:"headers"`
	Auth    *AuthConfig      `json:"auth"`
	Timeout int              `json:"timeout"`
}

// HTTPProviderFactory is a function that creates a new HTTPProvider
type HTTPProviderFactory func() *HTTPProvider

// NewDefaultHTTPProvider creates a default HTTP provider
func NewDefaultHTTPProvider() *HTTPProvider {
	return NewHTTPProvider()
}

// ConnectProvider creates a provider and connects to the HTTP server
func ConnectProvider(ctx context.Context, config *HTTPConfig) (*HTTPProvider, error) {
	provider := NewHTTPProvider()
	if err := provider.Connect(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to connect to HTTP server: %w", err)
	}
	return provider, nil
}

// HTTPProviderInterface defines the interface for HTTP server operations
type HTTPProviderInterface interface {
	Execute(ctx context.Context, req *Request) (*QueryResult, error)
	Close() error
	IsConnected() bool
}

var _ HTTPProviderInterface = (*HTTPProvider)(nil)
