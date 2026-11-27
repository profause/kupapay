package httpclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/profause/kupapay/pkg/redact"
)

// logResult emits a single structured log line per request, shaped by
// LogOptions. It is a no-op unless a real logger was attached via WithLogger.
func (c *Client) logResult(
	_ context.Context,
	method, url string,
	rc *requestConfig,
	status int,
	latency time.Duration,
	resp *Response,
	reqErr error,
) {
	if !c.loggingOn {
		return
	}

	fields := []any{
		"method", method,
		"url", url,
	}
	if status > 0 {
		fields = append(fields, "status", status)
	}
	if c.logOpts.LogLatency {
		fields = append(fields, "latency_ms", latency.Milliseconds())
	}
	if c.logOpts.LogRequestHeaders {
		fields = append(fields, "request_headers", redactHeaders(rc.headers))
	}
	if c.logOpts.LogRequestBody {
		if body := requestBodyForLog(rc, c.logOpts.maxBodyBytes()); body != "" {
			fields = append(fields, "request_body", body)
		}
	}
	if c.logOpts.LogResponseHeaders && resp != nil {
		fields = append(fields, "response_headers", redactFlatHeaders(resp.Header))
	}
	if c.logOpts.LogResponseBody && resp != nil {
		fields = append(fields, "response_body", bodyForLog(resp.Body, c.logOpts.maxBodyBytes()))
	}

	if reqErr != nil {
		fields = append(fields, "error", reqErr)
		c.log.Errorw("external http request failed", fields...)
		return
	}

	switch {
	case status >= 500:
		c.log.Errorw("external http request returned server error", fields...)
	case status >= 400:
		c.log.Warnw("external http request returned client error", fields...)
	default:
		if c.logOpts.SuccessLevel == "debug" {
			c.log.Debugw("external http request completed", fields...)
			return
		}
		c.log.Infow("external http request completed", fields...)
	}
}

// redactHeaders redacts sensitive values in a flat header map.
func redactHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if redact.IsSensitiveKey(k) {
			out[k] = redact.Placeholder
		} else {
			out[k] = v
		}
	}
	return out
}

// redactFlatHeaders flattens an http.Header (map[string][]string) into a
// loggable, redacted map.
func redactFlatHeaders(h map[string][]string) map[string][]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, vals := range h {
		if redact.IsSensitiveKey(k) {
			out[k] = []string{redact.Placeholder}
			continue
		}
		out[k] = vals
	}
	return out
}

// requestBodyForLog renders the outbound body for logging, redacting
// JSON/form payloads where possible and truncating to max bytes.
func requestBodyForLog(rc *requestConfig, max int) string {
	if len(rc.body) > 0 {
		return bodyForLog(rc.body, max)
	}
	if len(rc.formData) > 0 {
		return truncate(string(mustJSON(redact.Value(rc.formData))), max)
	}
	return ""
}

// bodyForLog renders a response/raw body: redacted JSON when parseable,
// otherwise the truncated raw string.
func bodyForLog(body []byte, max int) string {
	if len(body) == 0 {
		return ""
	}
	var parsed any
	if json.Unmarshal(body, &parsed) == nil {
		if b, err := json.Marshal(redact.Value(parsed)); err == nil {
			return truncate(string(b), max)
		}
	}
	return truncate(string(body), max)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func truncate(s string, max int) string {
	if max > 0 && len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}
