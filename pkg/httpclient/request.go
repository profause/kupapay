package httpclient

import (
	"encoding/json"
	"time"
)

// requestConfig accumulates per-request settings applied by RequestOption.
type requestConfig struct {
	headers    map[string]string
	query      map[string]string
	pathParams map[string]string
	cookies    map[string]string
	formData   map[string]string
	body       []byte
	encodeErr  error
	timeout    time.Duration
	userAgent  string
}

func newRequestConfig() *requestConfig {
	return &requestConfig{
		headers:    map[string]string{},
		query:      map[string]string{},
		pathParams: map[string]string{},
		cookies:    map[string]string{},
		formData:   map[string]string{},
	}
}

// RequestOption customizes a single request.
type RequestOption func(*requestConfig)

// Body sets the raw request body. Encoding is the caller's responsibility:
// marshal your payload (e.g. json.Marshal) and pass the resulting bytes here,
// and set the matching Content-Type yourself via Header — for example
// Header("Content-Type", "application/json").
func Body(raw []byte) RequestOption {
	return func(c *requestConfig) {
		c.body = raw
	}
}

// JSON marshals v as the request body and sets Content-Type: application/json.
// A marshal failure is surfaced by Do, so call sites stay one-liners. v is any
// for the same reason json.Marshal's argument is — concrete payloads flow in
// from typed callers.
func JSON(v any) RequestOption {
	return func(c *requestConfig) {
		raw, err := json.Marshal(v)
		if err != nil {
			c.encodeErr = err
			return
		}
		c.body = raw
		c.headers["Content-Type"] = "application/json"
	}
}

// FormData sets an application/x-www-form-urlencoded body.
func FormData(form map[string]string) RequestOption {
	return func(c *requestConfig) {
		for k, v := range form {
			c.formData[k] = v
		}
	}
}

// Header sets a single header for this request, overriding any client default.
func Header(key, value string) RequestOption {
	return func(c *requestConfig) { c.headers[key] = value }
}

// Headers sets multiple headers for this request, overriding client defaults.
func Headers(h map[string]string) RequestOption {
	return func(c *requestConfig) {
		for k, v := range h {
			c.headers[k] = v
		}
	}
}

// Query sets URL query parameters.
func Query(q map[string]string) RequestOption {
	return func(c *requestConfig) {
		for k, v := range q {
			c.query[k] = v
		}
	}
}

// QueryParam sets a single URL query parameter.
func QueryParam(key, value string) RequestOption {
	return func(c *requestConfig) { c.query[key] = value }
}

// PathParams substitutes :name placeholders in the URL path.
func PathParams(p map[string]string) RequestOption {
	return func(c *requestConfig) {
		for k, v := range p {
			c.pathParams[k] = v
		}
	}
}

// Cookies sets request cookies.
func Cookies(ck map[string]string) RequestOption {
	return func(c *requestConfig) {
		for k, v := range ck {
			c.cookies[k] = v
		}
	}
}

// Timeout overrides the client's default timeout for this request only.
func Timeout(d time.Duration) RequestOption {
	return func(c *requestConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// UserAgent overrides the User-Agent header for this request only.
func UserAgent(ua string) RequestOption {
	return func(c *requestConfig) { c.userAgent = ua }
}
