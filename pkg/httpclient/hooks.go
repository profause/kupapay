package httpclient

import "context"

// RequestInfo describes an outbound request as seen by hooks.
//
// Headers is the live, mutable map of per-request headers: a RequestHook may
// add, change, or delete entries and the modifications are applied to the
// request that is actually sent (including on every retry attempt, since the
// hook runs once per Do call).
type RequestInfo struct {
	Method  string
	URL     string
	Headers map[string]string
}

// RequestHook runs before a request is sent. Returning an error aborts the
// request before any network call (the error is returned from Do and does
// not satisfy errors.Is(err, ErrTransport)). Hooks run in registration order.
type RequestHook func(ctx context.Context, info *RequestInfo) error

// ResponseHook runs after the round-trip completes. It is also invoked on
// non-2xx responses and on transport failures — in the transport-failure
// case resp is nil and err is non-nil. Returning an error from a hook makes
// Do return that error (with resp still returned for inspection). Hooks run
// in registration order.
type ResponseHook func(ctx context.Context, info *RequestInfo, resp *Response, err error) error

// WithRequestHook registers one or more hooks invoked before each request.
// Common uses: request signing, dynamic auth tokens, correlation IDs.
func WithRequestHook(hooks ...RequestHook) Option {
	return func(c *Client) {
		c.requestHooks = append(c.requestHooks, hooks...)
	}
}

// WithResponseHook registers one or more hooks invoked after each request,
// including failures. Common uses: metrics, circuit breakers, audit logging.
func WithResponseHook(hooks ...ResponseHook) Option {
	return func(c *Client) {
		c.responseHooks = append(c.responseHooks, hooks...)
	}
}
