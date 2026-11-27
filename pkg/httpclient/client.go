// Package httpclient is the single entry point for outbound HTTP calls in
// kupapay. Every service that needs to talk to an external URL should
// instantiate a Client here rather than reaching for net/http directly, so
// that timeouts, default headers, retries, TLS and structured logging are
// configured consistently in one place.
//
// It is built on Fiber's fasthttp-backed client and exposes a small,
// allocation-friendly API. Responses are fully buffered and the underlying
// pooled response is released before returning, so a *Response is safe to
// retain and share across goroutines.
//
// Callers own request encoding: marshal your payload and pass the bytes to
// Body, setting the matching Content-Type via Header.
//
// Basic usage:
//
//	c, err := httpclient.New(
//	    httpclient.WithBaseURL("https://api.example.com"),
//	    httpclient.WithTimeout(15*time.Second),
//	    httpclient.WithHeaders(map[string]string{"X-App": "atlas"}),
//	    httpclient.WithLogger(log, httpclient.LogOptions{
//	        LogLatency:      true,
//	        LogResponseBody: true,
//	    }),
//	)
//	if err != nil {
//	    return fmt.Errorf("init http client: %w", err)
//	}
//
//	body, err := json.Marshal(payload)
//	if err != nil {
//	    return err
//	}
//	resp, err := c.Post(ctx, "/v1/payouts",
//	    httpclient.Body(body),
//	    httpclient.Header("Content-Type", "application/json"),
//	    httpclient.Header("Idempotency-Key", key),
//	)
//	if err != nil {
//	    return err // transport failure (errors.Is(err, httpclient.ErrTransport))
//	}
//	if !resp.IsSuccess() {
//	    return fmt.Errorf("payouts: unexpected status %d", resp.StatusCode)
//	}
//	var out PayoutResult
//	return resp.JSON(&out)
package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	fiberclient "github.com/gofiber/fiber/v3/client"
	"github.com/profause/kupapay/pkg/logger"
)

// Client is a reusable, concurrency-safe HTTP client for external calls.
//
// All per-request state is passed through to the underlying fiber client as
// request-scoped config, so a single Client may be shared by many goroutines.
type Client struct {
	fiber     *fiberclient.Client
	log       *logger.Logger
	logOpts   LogOptions
	loggingOn bool

	baseURL            string
	userAgent          string
	timeout            time.Duration
	headers            map[string]string
	retry              *RetryConfig
	proxyURL           string
	insecureSkipVerify bool

	requestHooks  []RequestHook
	responseHooks []ResponseHook

	// mock, when set, answers every request in place of the network — see
	// WithMockResponder.
	mock MockResponder
}

// New creates a Client. With no options it issues plain requests with a
// 30s timeout and no logging.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		log:     logger.NewNop(),
		timeout: DefaultTimeout,
		headers: map[string]string{},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	if c.fiber == nil {
		c.fiber = fiberclient.New()
	}

	c.fiber.SetTimeout(c.timeout)
	if c.baseURL != "" {
		c.fiber.SetBaseURL(c.baseURL)
	}
	if c.userAgent != "" {
		c.fiber.SetUserAgent(c.userAgent)
	}
	if len(c.headers) > 0 {
		c.fiber.SetHeaders(c.headers)
	}
	if c.retry != nil {
		c.fiber.SetRetryConfig(c.retry)
	}
	if c.insecureSkipVerify {
		c.fiber.SetTLSConfig(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // opt-in for non-prod use only
	}
	if c.proxyURL != "" {
		if err := c.fiber.SetProxyURL(c.proxyURL); err != nil {
			return nil, fmt.Errorf("httpclient: invalid proxy URL: %w", err)
		}
	}

	return c, nil
}

// Get issues a GET request.
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, opts...)
}

// Post issues a POST request.
func (c *Client) Post(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, opts...)
}

// Put issues a PUT request.
func (c *Client) Put(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodPut, url, opts...)
}

// Patch issues a PATCH request.
func (c *Client) Patch(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodPatch, url, opts...)
}

// Delete issues a DELETE request.
func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodDelete, url, opts...)
}

// Head issues a HEAD request.
func (c *Client) Head(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodHead, url, opts...)
}

// Do issues a request with an arbitrary HTTP method. It returns a non-nil
// error only on transport failure (the request never produced a response);
// a non-2xx response is returned as a *Response with a nil error so callers
// can inspect StatusCode and Body. Transport errors satisfy
// errors.Is(err, ErrTransport).
func (c *Client) Do(ctx context.Context, method, url string, opts ...RequestOption) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	rc := newRequestConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(rc)
		}
	}
	if rc.encodeErr != nil {
		return nil, fmt.Errorf("httpclient: encode request body: %w", rc.encodeErr)
	}

	timeout := c.timeout
	if rc.timeout > 0 {
		timeout = rc.timeout
	}

	info := &RequestInfo{Method: method, URL: url, Headers: rc.headers}
	for _, hook := range c.requestHooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx, info); err != nil {
			return nil, fmt.Errorf("httpclient: request hook: %w", err)
		}
	}

	var resp *Response
	var outErr error
	mocked := false

	start := time.Now()
	if c.mock != nil {
		// Sandbox (or a test) path: no fiber config to build, no socket ever
		// opened. The responder owns producing a *Response shaped exactly
		// like a real one, non-2xx included — unless it declines via
		// ErrPassThrough, in which case this request falls through to the
		// real round trip below like any unmocked client.
		resp, outErr = c.mock(ctx, info, rc.encodedBody())
		mocked = !errors.Is(outErr, ErrPassThrough)
	}
	if !mocked {
		// Discard whatever the responder set (ErrPassThrough, on this path) —
		// this request is now a normal, unmocked round trip.
		resp, outErr = nil, nil

		cfg := fiberclient.Config{
			Ctx:       ctx,
			Header:    rc.headers,
			Param:     rc.query,
			Cookie:    rc.cookies,
			PathParam: rc.pathParams,
			Timeout:   timeout,
			UserAgent: rc.userAgent,
		}
		// The body is already-encoded bytes supplied by the caller (see Body).
		// It takes precedence over FormData.
		switch {
		case rc.body != nil:
			cfg.Body = rc.body
		case len(rc.formData) > 0:
			cfg.FormData = rc.formData
		}

		fr, err := c.fiber.Custom(url, method, cfg)
		if err != nil {
			outErr = newTransportError(method, url, err)
		} else {
			resp = newResponse(fr)
			fr.Close()
		}
	}
	latency := time.Since(start)

	if hookErr := c.runResponseHooks(ctx, info, resp, outErr); hookErr != nil {
		outErr = hookErr
	}

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	c.logResult(ctx, method, url, rc, status, latency, resp, outErr)

	return resp, outErr
}

// runResponseHooks invokes response hooks in order, returning the first
// non-nil hook error. transportErr (if any) is passed through to hooks.
func (c *Client) runResponseHooks(ctx context.Context, info *RequestInfo, resp *Response, transportErr error) error {
	for _, hook := range c.responseHooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx, info, resp, transportErr); err != nil {
			return fmt.Errorf("httpclient: response hook: %w", err)
		}
	}
	return nil
}

// Fiber exposes the underlying fiber client for advanced use cases
// (streaming, cookie jars, custom transports) not covered by this wrapper.
func (c *Client) Fiber() *fiberclient.Client {
	return c.fiber
}
