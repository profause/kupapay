package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/profause/kupapay/pkg/logger"
)

func TestClient_GetWithBaseURLHeadersAndQuery(t *testing.T) {
	var gotPath, gotDefault, gotPerReq, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotDefault = r.Header.Get("X-App")
		gotPerReq = r.Header.Get("X-Trace")
		gotQuery = r.URL.Query().Get("verbose")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c, err := New(
		WithBaseURL(srv.URL),
		WithHeaders(map[string]string{"X-App": "atlas"}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.Get(context.Background(), "/v1/ping",
		Header("X-Trace", "abc"),
		QueryParam("verbose", "true"),
	)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !resp.IsSuccess() || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.String() != "ok" {
		t.Fatalf("unexpected body %q", resp.String())
	}
	if gotPath != "/v1/ping" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotDefault != "atlas" {
		t.Fatalf("default header = %q", gotDefault)
	}
	if gotPerReq != "abc" {
		t.Fatalf("per-request header = %q", gotPerReq)
	}
	if gotQuery != "true" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestClient_PostJSONRoundTrip(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
	}
	type respBody struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("content-type = %q", ct)
		}
		var in reqBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(respBody{ID: 7, Name: in.Name})
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	reqJSON, err := json.Marshal(reqBody{Name: "ada"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := c.Post(context.Background(), "/v1/users",
		Body(reqJSON),
		Header("Content-Type", "application/json"),
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out respBody
	if err := resp.JSON(&out); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if out.ID != 7 || out.Name != "ada" {
		t.Fatalf("unexpected response %+v", out)
	}
}

func TestClient_PathParams(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := New(WithBaseURL(srv.URL))
	if _, err := c.Get(context.Background(), "/users/:id", PathParams(map[string]string{"id": "42"})); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/users/42" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestClient_NonSuccessIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()

	c, _ := New()
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("expected nil error for non-2xx, got %v", err)
	}
	if resp.IsSuccess() {
		t.Fatalf("expected IsSuccess false for %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestClient_TransportErrorIsErrTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	url := srv.URL
	srv.Close() // nothing is listening now

	c, _ := New(WithTimeout(500 * time.Millisecond))
	resp, err := c.Get(context.Background(), url)
	if err == nil {
		t.Fatalf("expected transport error, got resp %+v", resp)
	}
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("expected errors.Is ErrTransport, got %v", err)
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Method != http.MethodGet {
		t.Fatalf("expected *TransportError with method GET, got %v", err)
	}
}

func TestClient_PerRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := New(WithTimeout(5 * time.Second))
	_, err := c.Get(context.Background(), srv.URL, Timeout(20*time.Millisecond))
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("expected ErrTransport, got %v", err)
	}
}

func TestClient_LoggingRedactsSensitiveHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"super-secret","ok":true}`))
	}))
	defer srv.Close()

	logFile := filepath.Join(t.TempDir(), "http.log")
	svc, err := logger.NewService("debug", "json", logFile)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	c, _ := New(
		WithLogger(svc.Get(), LogOptions{
			LogLatency:        true,
			LogRequestHeaders: true,
			LogResponseBody:   true,
			LogRequestBody:    true,
		}),
	)

	reqBody, err := json.Marshal(map[string]string{"password": "hunter2", "name": "ada"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = c.Post(context.Background(), srv.URL,
		Header("Authorization", "Bearer leak-me"),
		Header("Content-Type", "application/json"),
		Body(reqBody),
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logged := string(data)

	if strings.Contains(logged, "leak-me") {
		t.Fatalf("Authorization header leaked into logs:\n%s", logged)
	}
	if strings.Contains(logged, "hunter2") {
		t.Fatalf("password leaked into logs:\n%s", logged)
	}
	if strings.Contains(logged, "super-secret") {
		t.Fatalf("response token leaked into logs:\n%s", logged)
	}
	if !strings.Contains(logged, "[REDACTED]") {
		t.Fatalf("expected redaction placeholder in logs:\n%s", logged)
	}
	if !strings.Contains(logged, "latency_ms") {
		t.Fatalf("expected latency_ms in logs:\n%s", logged)
	}
	if !strings.Contains(logged, "ada") {
		t.Fatalf("expected non-sensitive request field to survive:\n%s", logged)
	}
}

func TestClient_NoLoggerIsSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.loggingOn {
		t.Fatalf("expected logging off by default")
	}
	if _, err := c.Get(context.Background(), srv.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}
}

func TestClient_RequestHookMutatesHeadersAndCanAbort(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Sig")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	calls := 0
	c, _ := New(WithRequestHook(func(_ context.Context, info *RequestInfo) error {
		calls++
		info.Headers["X-Sig"] = "signed-" + info.Method
		return nil
	}))

	if _, err := c.Post(context.Background(), srv.URL, Body([]byte(`{"a":"b"}`))); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotSig != "signed-POST" {
		t.Fatalf("hook did not inject header, got %q", gotSig)
	}
	if calls != 1 {
		t.Fatalf("expected hook called once, got %d", calls)
	}

	// A request hook returning an error aborts before any network call and is
	// not classified as a transport error.
	sentinel := errors.New("blocked by policy")
	c2, _ := New(WithRequestHook(func(_ context.Context, _ *RequestInfo) error {
		return sentinel
	}))
	_, err := c2.Get(context.Background(), srv.URL)
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if errors.Is(err, ErrTransport) {
		t.Fatalf("hook abort should not be a transport error")
	}
}

func TestClient_ResponseHookObservesStatusAndTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	var seenStatus int
	var seenErr error
	c, _ := New(WithResponseHook(func(_ context.Context, _ *RequestInfo, resp *Response, err error) error {
		seenErr = err
		if resp != nil {
			seenStatus = resp.StatusCode
		}
		return nil
	}))

	if _, err := c.Get(context.Background(), srv.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if seenStatus != http.StatusBadGateway || seenErr != nil {
		t.Fatalf("response hook saw status=%d err=%v", seenStatus, seenErr)
	}

	// Transport failure: resp nil, err non-nil, and a hook error propagates.
	url := srv.URL
	srv.Close()
	hookErr := errors.New("hook rejected")
	c2, _ := New(
		WithTimeout(300*time.Millisecond),
		WithResponseHook(func(_ context.Context, _ *RequestInfo, resp *Response, err error) error {
			if resp != nil || err == nil {
				t.Errorf("expected nil resp and non-nil err on transport failure")
			}
			return hookErr
		}),
	)
	_, err := c2.Get(context.Background(), url)
	if !errors.Is(err, hookErr) {
		t.Fatalf("expected hook error to propagate, got %v", err)
	}
}
