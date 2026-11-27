package redact

import "testing"

func TestField_RedactsSensitiveScalarsAndNestedValues(t *testing.T) {
	got := Field("payload", map[string]any{
		"safe": "ok",
		"nested": map[string]any{
			"accessToken": "abc123",
		},
		"headers": map[string][]string{
			"Authorization": {"Bearer token"},
		},
		"items": []any{
			map[string]any{"client_secret": "top-secret"},
			"keep-me",
		},
	})

	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	if payload["safe"] != "ok" {
		t.Fatalf("expected safe field to be preserved, got %#v", payload["safe"])
	}

	nested, ok := payload["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", payload["nested"])
	}
	if nested["accessToken"] != Placeholder {
		t.Fatalf("expected nested access token to be redacted, got %#v", nested["accessToken"])
	}

	headers, ok := payload["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers map, got %T", payload["headers"])
	}
	if headers["Authorization"] != Placeholder {
		t.Fatalf("expected authorization header to be redacted, got %#v", headers["Authorization"])
	}

	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected items slice, got %T", payload["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first item to be a map, got %T", items[0])
	}
	if first["client_secret"] != Placeholder {
		t.Fatalf("expected client_secret to be redacted, got %#v", first["client_secret"])
	}
	if items[1] != "keep-me" {
		t.Fatalf("expected non-sensitive slice item to be preserved, got %#v", items[1])
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := map[string]bool{
		"Authorization":      true,
		"api_key":            true,
		"initiatorPassword":  true,
		"securityCredential": true,
		"tokenExpiry":        false,
		"publicKey":          false,
		"reference":          false,
	}

	for key, want := range tests {
		if got := IsSensitiveKey(key); got != want {
			t.Fatalf("IsSensitiveKey(%q) = %v, want %v", key, got, want)
		}
	}
}
