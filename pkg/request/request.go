package request

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"golang.org/x/time/rate"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type ClientOption func(*Client)

// Client represents an HTTP client with additional capabilities
type Client struct {
	client          *http.Client
	rateLimiter     *rate.Limiter
	headers         map[string]string
	maxRetries      int
	timeout         time.Duration
	skipTLSVerify   bool
	retryableStatus map[int]bool
	logger          zerolog.Logger
}

// WithMaxRetries sets the maximum number of retry attempts
func WithMaxRetries(retries int) ClientOption {
	return func(c *Client) {
		c.maxRetries = retries
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithRateLimiter sets a rate limiter
func WithRateLimiter(rl *rate.Limiter) ClientOption {
	return func(c *Client) {
		c.rateLimiter = rl
	}
}

// WithHeaders sets default headers
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		c.headers = headers
	}
}

func WithLogger(logger zerolog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithRetryableStatus adds status codes that should trigger a retry
func WithRetryableStatus(statusCodes ...int) ClientOption {
	return func(c *Client) {
		for _, code := range statusCodes {
			c.retryableStatus[code] = true
		}
	}
}

// doRequest performs a single HTTP request with rate limiting
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if c.rateLimiter != nil {
		err := c.rateLimiter.Wait(req.Context())
		if err != nil {
			return nil, fmt.Errorf("rate limiter wait: %w", err)
		}
	}

	return c.client.Do(req)
}

// Do performs an HTTP request with retries for certain status codes
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Save the request body for reuse in retries
	var bodyBytes []byte
	var err error

	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		req.Body.Close()
	}

	// Apply timeout to the request context if not already present
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx := req.Context()
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	backoff := time.Millisecond * 500
	var resp *http.Response

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Reset the request body if it exists
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Apply headers
		if c.headers != nil {
			for key, value := range c.headers {
				req.Header.Set(key, value)
			}
		}

		resp, err = c.doRequest(req)
		if err != nil {
			// Check if this is a network error that might be worth retrying
			if attempt < c.maxRetries {
				// Apply backoff with jitter
				jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
				sleepTime := backoff + jitter

				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(sleepTime):
					// Continue to next retry attempt
				}

				// Exponential backoff
				backoff *= 2
				continue
			}
			return nil, err
		}

		// Check if the status code is retryable
		if !c.retryableStatus[resp.StatusCode] || attempt == c.maxRetries {
			return resp, nil
		}

		// Close the response body before retrying
		resp.Body.Close()

		// Apply backoff with jitter
		jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
		sleepTime := backoff + jitter

		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(sleepTime):
			// Continue to next retry attempt
		}

		// Exponential backoff
		backoff *= 2
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// MakeRequest performs an HTTP request and returns the response body as bytes
func (c *Client) MakeRequest(req *http.Request) ([]byte, error) {
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d: %s", res.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}

// New creates a new HTTP client with the specified options
func New(options ...ClientOption) *Client {
	client := &Client{
		maxRetries:    3,
		skipTLSVerify: true,
		retryableStatus: map[int]bool{
			http.StatusTooManyRequests:     true,
			http.StatusInternalServerError: true,
			http.StatusBadGateway:          true,
			http.StatusServiceUnavailable:  true,
			http.StatusGatewayTimeout:      true,
		},
		logger: logger.NewLogger("request"),
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Create transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: client.skipTLSVerify,
		},
		Proxy: http.ProxyFromEnvironment,
	}

	// Create HTTP client
	client.client = &http.Client{
		Transport: transport,
		Timeout:   client.timeout,
	}

	return client
}
