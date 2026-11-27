package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	fiberclient "github.com/gofiber/fiber/v3/client"
)

// Response is a fully buffered, pool-safe view of an HTTP response.
//
// The fiber/fasthttp response is released back to its pool before this struct
// is returned, so Body is an owned copy that remains valid for the caller's
// lifetime and is safe to use across goroutines.
type Response struct {
	// StatusCode is the HTTP status code (e.g. 200).
	StatusCode int

	// Status is the HTTP status line (e.g. "200 OK").
	Status string

	// Header holds the response headers.
	Header http.Header

	// Body is the full response body.
	Body []byte
}

// IsSuccess reports whether the status code is in the 2xx range.
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// String returns the body as a string.
func (r *Response) String() string {
	return string(r.Body)
}

// JSON unmarshals the response body into v.
func (r *Response) JSON(v any) error {
	if len(r.Body) == 0 {
		return fmt.Errorf("httpclient: empty response body")
	}
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("httpclient: decode JSON response: %w", err)
	}
	return nil
}

// newResponse copies everything off the pooled fiber response so the caller
// never touches recycled fasthttp memory.
func newResponse(fr *fiberclient.Response) *Response {
	hdr := http.Header{}
	for k, vals := range fr.Headers() {
		for _, v := range vals {
			hdr.Add(k, v)
		}
	}

	src := fr.Body()
	body := make([]byte, len(src))
	copy(body, src)

	return &Response{
		StatusCode: fr.StatusCode(),
		Status:     fr.Status(),
		Header:     hdr,
		Body:       body,
	}
}
