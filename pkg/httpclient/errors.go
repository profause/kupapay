package httpclient

import (
	"errors"
	"fmt"
)

// ErrTransport indicates the request never produced an HTTP response
// (DNS failure, connection refused, timeout, context cancelled, etc.).
//
// Use errors.Is(err, httpclient.ErrTransport) to distinguish a transport
// failure from a non-2xx response.
var ErrTransport = errors.New("httpclient: transport error")

// TransportError wraps the underlying transport failure while satisfying
// errors.Is(err, ErrTransport).
type TransportError struct {
	Method string
	URL    string
	Err    error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("httpclient: %s %s: %v", e.Method, e.URL, e.Err)
}

func (e *TransportError) Unwrap() error { return e.Err }

func (e *TransportError) Is(target error) bool { return target == ErrTransport }

func newTransportError(method, url string, err error) *TransportError {
	return &TransportError{Method: method, URL: url, Err: err}
}
