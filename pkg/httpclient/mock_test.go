package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClient_MockResponderBypassesNetwork asserts the core guarantee a
// sandbox mock depends on: with a responder attached, Do never reaches the
// network — an unroutable base URL would time out or error if it did — and
// the responder's Response comes back untouched, hooks included.
func TestClient_MockResponderBypassesNetwork(t *testing.T) {
	var sawRequestHook, sawResponseHook bool

	c, err := New(
		// A base URL nothing listens on: any real round trip fails fast,
		// making an accidental network call impossible to miss.
		WithBaseURL("http://127.0.0.1:1"),
		WithRequestHook(func(_ context.Context, info *RequestInfo) error {
			sawRequestHook = true
			info.Headers["X-Signed"] = "yes"
			return nil
		}),
		WithResponseHook(func(_ context.Context, _ *RequestInfo, resp *Response, err error) error {
			sawResponseHook = true
			if err != nil {
				t.Errorf("response hook: unexpected error %v", err)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("response hook: status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			return nil
		}),
		WithMockResponder(func(_ context.Context, info *RequestInfo, body []byte) (*Response, error) {
			if info.Headers["X-Signed"] != "yes" {
				t.Errorf("mock responder: request hook did not run before it, headers = %v", info.Headers)
			}
			if string(body) != `{"amount":100}` {
				t.Errorf("mock responder: body = %q", body)
			}
			return &Response{StatusCode: http.StatusCreated, Body: []byte(`{"ok":true}`)}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.Post(t.Context(), "/v1/cards",
		Body([]byte(`{"amount":100}`)),
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if !sawRequestHook {
		t.Error("request hook did not run")
	}
	if !sawResponseHook {
		t.Error("response hook did not run")
	}
}

// TestClient_MockResponderPropagatesError asserts a responder's own error —
// distinct from a non-2xx Response — surfaces as Do's error return, the same
// contract a transport failure has.
func TestClient_MockResponderPropagatesError(t *testing.T) {
	wantErr := context.DeadlineExceeded

	c, err := New(
		WithBaseURL("http://127.0.0.1:1"),
		WithMockResponder(func(context.Context, *RequestInfo, []byte) (*Response, error) {
			return nil, wantErr
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.Get(t.Context(), "/v1/cards/abc"); err != wantErr {
		t.Fatalf("Get: err = %v, want %v", err, wantErr)
	}
}

// TestClient_MockResponderPassThrough asserts a responder can cover only the
// routes it knows about: a request it declines with ErrPassThrough reaches
// the real server exactly as if no responder were attached.
func TestClient_MockResponderPassThrough(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"real":true}`))
	}))
	defer srv.Close()

	c, err := New(
		WithBaseURL(srv.URL),
		WithMockResponder(func(_ context.Context, info *RequestInfo, _ []byte) (*Response, error) {
			if info.URL == "/v1/cards" {
				return &Response{StatusCode: http.StatusCreated, Body: []byte(`{"mocked":true}`)}, nil
			}
			return nil, ErrPassThrough
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.Get(t.Context(), "/v1/rates")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/v1/rates" {
		t.Errorf("real server saw path %q, want /v1/rates", gotPath)
	}
	if resp.String() != `{"real":true}` {
		t.Errorf("body = %q, want the real server's body", resp.String())
	}
}

// TestChainMockResponders asserts each responder gets a turn in order, the
// first non-declining one wins, and a route none of them recognize still
// falls all the way through to the real network.
func TestChainMockResponders(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cardsOnly := func(_ context.Context, info *RequestInfo, _ []byte) (*Response, error) {
		if info.URL != "/cards" {
			return nil, ErrPassThrough
		}
		return &Response{StatusCode: http.StatusCreated}, nil
	}
	payoutsOnly := func(_ context.Context, info *RequestInfo, _ []byte) (*Response, error) {
		if info.URL != "/payouts/init" {
			return nil, ErrPassThrough
		}
		return &Response{StatusCode: http.StatusAccepted}, nil
	}

	c, err := New(WithBaseURL(srv.URL), WithMockResponder(ChainMockResponders(cardsOnly, payoutsOnly)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if resp, err := c.Post(t.Context(), "/cards"); err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /cards: resp = %+v, err = %v", resp, err)
	}
	if resp, err := c.Post(t.Context(), "/payouts/init"); err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /payouts/init: resp = %+v, err = %v", resp, err)
	}
	if _, err := c.Get(t.Context(), "/rates"); err != nil {
		t.Fatalf("GET /rates: %v", err)
	}
	if gotPath != "/rates" {
		t.Errorf("real server saw path %q, want /rates (both responders should have declined)", gotPath)
	}
}

// TestClient_MockResponderSeesFormData pins the contract MockResponder
// documents: the responder receives the bytes the wire would have carried.
// FormData requests leave rc.body nil, so before encodedBody they reached the
// responder as a nil body and a form-encoded route could not be mocked at all.
func TestClient_MockResponderSeesFormData(t *testing.T) {
	var got string

	c, err := New(
		WithBaseURL("http://127.0.0.1:1"),
		WithMockResponder(func(_ context.Context, _ *RequestInfo, body []byte) (*Response, error) {
			got = string(body)
			return &Response{StatusCode: http.StatusOK}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.Post(t.Context(), "/v1/token",
		FormData(map[string]string{"grant_type": "client_credentials", "scope": "payouts"}),
	); err != nil {
		t.Fatalf("Post: %v", err)
	}

	// url.Values.Encode sorts by key, so this is stable across map orderings.
	if want := "grant_type=client_credentials&scope=payouts"; got != want {
		t.Errorf("responder body = %q, want %q", got, want)
	}
}

// TestClient_MockResponderBodyWinsOverFormData mirrors the precedence the real
// round trip applies when building the fiber request, so the mocked and live
// paths cannot disagree about which of the two a caller set.
func TestClient_MockResponderBodyWinsOverFormData(t *testing.T) {
	var got string

	c, err := New(
		WithBaseURL("http://127.0.0.1:1"),
		WithMockResponder(func(_ context.Context, _ *RequestInfo, body []byte) (*Response, error) {
			got = string(body)
			return &Response{StatusCode: http.StatusOK}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.Post(t.Context(), "/v1/token",
		Body([]byte(`{"explicit":true}`)),
		FormData(map[string]string{"grant_type": "client_credentials"}),
	); err != nil {
		t.Fatalf("Post: %v", err)
	}

	if want := `{"explicit":true}`; got != want {
		t.Errorf("responder body = %q, want %q", got, want)
	}
}
