package httpclient

import (
	"time"

	"github.com/gofiber/fiber/v3/client"
	"github.com/profause/kupapay/pkg/logger"
)

const (
	// DefaultTimeout is the per-request timeout applied when none is configured.
	DefaultTimeout = 60 * time.Second

	// DefaultMaxBodyLogBytes caps how many bytes of a request/response body are
	// written to logs when body logging is enabled.
	DefaultMaxBodyLogBytes = 4096
)

// LogOptions tunes the verbosity of per-request logging. It does not turn
// logging on or off: logging is enabled solely by attaching a logger via
// WithLogger, and disabled when no logger is set (regardless of these flags).
//
// With a logger attached and the zero-value LogOptions, every request still
// emits one line containing the method, URL, the status (when a response was
// received) and a level reflecting the outcome (2xx -> info/debug,
// 4xx -> warn, 5xx or transport failure -> error). Latency, request/response
// bodies and request/response headers are each opt-in via the corresponding
// field below, because they are noisier or potentially sensitive. Header
// values for sensitive keys (Authorization, API keys, tokens, ...) and
// redactable body fields are redacted automatically.
type LogOptions struct {
	// LogRequestBody includes the (possibly truncated) request body.
	LogRequestBody bool

	// LogResponseBody includes the (possibly truncated) response body.
	LogResponseBody bool

	// LogRequestHeaders includes outbound request headers (redacted).
	LogRequestHeaders bool

	// LogResponseHeaders includes inbound response headers (redacted).
	LogResponseHeaders bool

	// LogLatency includes the round-trip duration in milliseconds.
	// Latency is always measured; this only controls whether it is emitted.
	LogLatency bool

	// MaxBodyLogBytes caps logged body size. <= 0 uses DefaultMaxBodyLogBytes.
	MaxBodyLogBytes int

	// SuccessLevel is the level used for 2xx responses. Defaults to "info".
	// One of: "debug", "info". Non-2xx responses log at "warn" and transport
	// failures at "error" regardless of this setting.
	SuccessLevel string
}

func (o LogOptions) maxBodyBytes() int {
	if o.MaxBodyLogBytes <= 0 {
		return DefaultMaxBodyLogBytes
	}
	return o.MaxBodyLogBytes
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithBaseURL sets a base URL that is prepended to request paths, so callers
// can pass relative paths like "/v1/users".
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = baseURL }
}

// WithTimeout sets the default per-request timeout. A non-positive value is
// ignored. The timeout can still be overridden per request with Timeout().
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithHeaders sets default headers sent on every request. They can be
// overridden per request via Header/Headers request options.
func WithHeaders(h map[string]string) Option {
	return func(c *Client) {
		for k, v := range h {
			c.headers[k] = v
		}
	}
}

// WithHeader sets a single default header sent on every request.
func WithHeader(key, value string) Option {
	return func(c *Client) { c.headers[key] = value }
}

// WithUserAgent sets the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithLogger attaches a logger and turns on per-request logging. Without it
// the client logs nothing. An optional LogOptions tunes verbosity (latency,
// bodies, headers); omit it for the baseline method/URL/status line. If more
// than one LogOptions is passed the last one wins. Passing a nil logger is a
// no-op (logging stays off).
func WithLogger(l *logger.Logger, opts ...LogOptions) Option {
	return func(c *Client) {
		if l == nil {
			return
		}
		c.log = l.WithComponent("httpclient")
		c.loggingOn = true
		if len(opts) > 0 {
			c.logOpts = opts[len(opts)-1]
		}
	}
}

// RetryConfig is re-exported so callers do not need to import the fiber client
// package directly.
type RetryConfig = client.RetryConfig

// WithRetry enables automatic retries with exponential backoff for idempotent
// failures, using fiber's retry addon. Pass a zero RetryConfig for defaults.
func WithRetry(cfg RetryConfig) Option {
	return func(c *Client) { c.retry = &cfg }
}

// WithProxy routes all requests through the given proxy URL.
func WithProxy(proxyURL string) Option {
	return func(c *Client) { c.proxyURL = proxyURL }
}

// WithInsecureSkipVerify disables TLS certificate verification.
// Intended for local/test environments only — never use in production.
func WithInsecureSkipVerify() Option {
	return func(c *Client) { c.insecureSkipVerify = true }
}

// WithFiberClient supplies a pre-configured fiber client instead of building a
// fresh one. Useful for sharing a connection pool or injecting a test double.
func WithFiberClient(fc *client.Client) Option {
	return func(c *Client) {
		if fc != nil {
			c.fiber = fc
		}
	}
}
