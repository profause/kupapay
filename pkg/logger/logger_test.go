package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/profause/kupapay/pkg/redact"
	"github.com/rs/zerolog"
)

func TestInfow_RedactsSensitiveFields(t *testing.T) {
	var buffer bytes.Buffer
	log := &Logger{logger: zerolog.New(&buffer)}

	log.Infow("request",
		"apiKey", "super-secret",
		"body", map[string]any{
			"reference": "ref-123",
			"nested": map[string]any{
				"accessToken": "token-value",
			},
		},
	)

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if entry["apiKey"] != redact.Placeholder {
		t.Fatalf("expected apiKey to be redacted, got %#v", entry["apiKey"])
	}

	body, ok := entry["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected body to be a map, got %T", entry["body"])
	}
	if body["reference"] != "ref-123" {
		t.Fatalf("expected reference to be preserved, got %#v", body["reference"])
	}

	nested, ok := body["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested body field to be a map, got %T", body["nested"])
	}
	if nested["accessToken"] != redact.Placeholder {
		t.Fatalf("expected nested access token to be redacted, got %#v", nested["accessToken"])
	}
}

func TestErrorw_LogsErrorString(t *testing.T) {
	var buffer bytes.Buffer
	log := &Logger{logger: zerolog.New(&buffer)}

	log.Errorw("failed", "error", errors.New("provider returned invalid rate"))

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if entry["error"] != "provider returned invalid rate" {
		t.Fatalf("expected error string, got %#v", entry["error"])
	}
}

func TestErrorw_RedactsSensitiveErrorFields(t *testing.T) {
	var buffer bytes.Buffer
	log := &Logger{logger: zerolog.New(&buffer)}

	log.Errorw("failed", "authorization", errors.New("Bearer secret-token"))

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if entry["authorization"] != redact.Placeholder {
		t.Fatalf("expected authorization error to be redacted, got %#v", entry["authorization"])
	}
}

func TestWithFields_RedactsContextFields(t *testing.T) {
	var buffer bytes.Buffer
	log := &Logger{
		logger: zerolog.New(&buffer).With().Logger(),
	}

	log.WithFields(map[string]interface{}{
		"headers": map[string]any{
			"Authorization": "Bearer abc123",
		},
	}).Info("context")

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	headers, ok := entry["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers to be a map, got %T", entry["headers"])
	}
	if headers["Authorization"] != redact.Placeholder {
		t.Fatalf("expected authorization header to be redacted, got %#v", headers["Authorization"])
	}
}
