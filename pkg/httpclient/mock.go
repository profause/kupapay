package httpclient

import (
	"context"
	"errors"
	"net/url"
)

// ErrPassThrough is returned by a MockResponder to decline a request it does
// not recognize. Do then falls back to a real network round trip for that
// request only, exactly as if no responder were attached — so a responder
// can cover just the routes it knows about, and everything else keeps
// working as coverage grows.
var ErrPassThrough = errors.New("httpclient: mock responder declined, falling through to the network")

// MockResponder synthesizes a Response for a request instead of the client
// performing a real network round trip. It receives the same request view a
// hook would see, plus the encoded body the wire would have carried — the
// bytes given to Body/JSON, or the url-encoded form when the request was
// built with FormData — and must return exactly what Do would otherwise have
// produced from the wire: a non-2xx outcome is a *Response with a nil error,
// not an error return. Return ErrPassThrough (wrapped or bare) to let this
// one request go to the network instead.
//
// Request hooks, response hooks and logging all still run around it, so a
// client with a responder attached is indistinguishable from a real one to
// its caller — for a passed-through request, indistinguishable from one with
// no responder at all.
type MockResponder func(ctx context.Context, info *RequestInfo, body []byte) (*Response, error)

// WithMockResponder replaces the client's network round trip with fn. It
// exists for environments — sandbox, most often — where a provider must
// never be reached over the network at all. Each provider package owns its
// own responder (see pkg/nexus's, for one): only that package knows the
// shape of its own requests and responses. httpclient owns only the
// mechanism for swapping one in.
func WithMockResponder(fn MockResponder) Option {
	return func(c *Client) { c.mock = fn }
}

// encodedBody returns the bytes this request would have put on the wire, so a
// responder sees the same payload the real round trip would send. Body takes
// precedence over FormData here exactly as it does when building the fiber
// request, keeping the two paths from disagreeing about which one wins.
func (rc *requestConfig) encodedBody() []byte {
	if rc.body != nil || len(rc.formData) == 0 {
		return rc.body
	}
	form := url.Values{}
	for k, v := range rc.formData {
		form.Set(k, v)
	}
	return []byte(form.Encode())
}

// ChainMockResponders combines several responders into one, for a client
// (like nexus's) shared by more than one feature area. Each is tried in
// order; the first that does not decline with ErrPassThrough wins. If every
// one declines, the chain itself declines, so Do falls through to the real
// network exactly as it would for a single non-matching responder.
//
// This is how independent responders accumulate on one client — a card
// responder, a payout responder, and so on — without any of them needing to
// know the others exist.
func ChainMockResponders(responders ...MockResponder) MockResponder {
	return func(ctx context.Context, info *RequestInfo, body []byte) (*Response, error) {
		for _, respond := range responders {
			if respond == nil {
				continue
			}
			resp, err := respond(ctx, info, body)
			if errors.Is(err, ErrPassThrough) {
				continue
			}
			return resp, err
		}
		return nil, ErrPassThrough
	}
}
