package customerror

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeNotFound, "resource not found")

	if err.Code() != ErrCodeNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeNotFound, err.Code())
	}

	if err.Message() != "resource not found" {
		t.Errorf("Expected message 'resource not found', got '%s'", err.Message())
	}

	if err.Error() != "resource not found" {
		t.Errorf("Expected error string 'resource not found', got '%s'", err.Error())
	}
}

func TestWrap(t *testing.T) {
	baseErr := errors.New("base error")
	appErr := New(ErrCodeInternal, "internal error").Wrap(baseErr)

	if appErr.Cause() != baseErr {
		t.Error("Wrapped error should preserve cause")
	}

	if appErr.Error() != "internal error: base error" {
		t.Errorf("Expected 'internal error: base error', got '%s'", appErr.Error())
	}

	// Test unwrapping
	if errors.Unwrap(appErr) != baseErr {
		t.Error("Unwrap should return base error")
	}
}

func TestWithContext(t *testing.T) {
	err := New(ErrCodeNotFound, "user not found").
		WithContext("id", "123").
		WithContext("email", "test@example.com")

	ctx := err.Context()
	if ctx["id"] != "123" {
		t.Errorf("Expected id '123', got '%v'", ctx["id"])
	}
	if ctx["email"] != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%v'", ctx["email"])
	}
}

func TestWithKey(t *testing.T) {
	err := New(ErrCodeNotFound, "cache miss").WithKey("user:123")

	ctx := err.Context()
	if ctx["key"] != "user:123" {
		t.Errorf("Expected key 'user:123', got '%v'", ctx["key"])
	}
}

func TestWithID(t *testing.T) {
	err := New(ErrCodeNotFound, "record not found").WithID("abc-def")

	ctx := err.Context()
	if ctx["id"] != "abc-def" {
		t.Errorf("Expected id 'abc-def', got '%v'", ctx["id"])
	}
}

func TestErrorIs(t *testing.T) {
	baseErr := New(ErrCodeNotFound, "not found")
	wrappedErr := baseErr.Wrap(errors.New("underlying"))

	// errors.Is should work with our errors
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is should match base error")
	}

	differentErr := New(ErrCodeInvalidInput, "invalid")
	if errors.Is(wrappedErr, differentErr) {
		t.Error("errors.Is should not match different error code")
	}
}

func TestErrorAs(t *testing.T) {
	appErr := New(ErrCodeValidation, "validation failed").WithContext("field", "email")
	wrappedErr := appErr.Wrap(errors.New("invalid format"))

	var targetErr *Error
	if !errors.As(wrappedErr, &targetErr) {
		t.Fatal("errors.As should extract typed error")
	}

	if targetErr.Code() != ErrCodeValidation {
		t.Errorf("Expected code %s, got %s", ErrCodeValidation, targetErr.Code())
	}

	ctx := targetErr.Context()
	if ctx["field"] != "email" {
		t.Error("Context should be preserved")
	}
}

func TestGetCode(t *testing.T) {
	appErr := New(ErrCodeTimeout, "operation timed out")

	code := GetCode(appErr)
	if code != ErrCodeTimeout {
		t.Errorf("Expected code %s, got %s", ErrCodeTimeout, code)
	}

	// Test with non-typed error
	stdErr := errors.New("standard error")
	code = GetCode(stdErr)
	if code != ErrCodeUnknown {
		t.Errorf("Expected code %s for standard error, got %s", ErrCodeUnknown, code)
	}
}

func TestGetContext(t *testing.T) {
	appErr := New(ErrCodeNotFound, "not found").
		WithContext("resource", "user").
		WithContext("id", 123)

	ctx := GetContext(appErr)
	if ctx == nil {
		t.Fatal("Context should not be nil")
	}

	if ctx["resource"] != "user" {
		t.Error("Context should contain resource")
	}

	// Test with non-typed error
	stdErr := errors.New("standard error")
	ctx = GetContext(stdErr)
	if ctx != nil {
		t.Error("Context should be nil for standard error")
	}
}

func TestCause(t *testing.T) {
	rootErr := errors.New("root cause")
	err1 := New(ErrCodeInternal, "level 1").Wrap(rootErr)
	err2 := New(ErrCodeInternal, "level 2").Wrap(err1)

	cause := Cause(err2)
	if cause != rootErr {
		t.Error("Cause should return root error")
	}
}

func TestErrorCodeHTTPStatus(t *testing.T) {
	tests := []struct {
		code   ErrorCode
		status int
	}{
		{ErrCodeNotFound, 404},
		{ErrCodeInvalidInput, 400},
		{ErrCodeUnauthorized, 401},
		{ErrCodeForbidden, 403},
		{ErrCodeConflict, 409},
		{ErrCodeTimeout, 408},
		{ErrCodeRateLimited, 429},
		{ErrCodeInternal, 500},
	}

	for _, tt := range tests {
		status := tt.code.HTTPStatusCode()
		if status != tt.status {
			t.Errorf("Code %s: expected HTTP %d, got %d", tt.code, tt.status, status)
		}
	}
}

func TestErrorCodeHTTPStatusPreservesNumericStatusCodes(t *testing.T) {
	code := ErrorCode("422")
	if code != "422" {
		t.Fatalf("code = %s, want 422", code)
	}
	if got := code.HTTPStatusCode(); got != 422 {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestErrorCodeIsRetryable(t *testing.T) {
	retryable := []ErrorCode{
		ErrCodeTimeout,
		ErrCodeConnection,
		ErrCodeNetworkError,
		ErrCodeDisconnected,
		ErrCodeQueueFull,
	}

	for _, code := range retryable {
		if !code.IsRetryable() {
			t.Errorf("Code %s should be retryable", code)
		}
	}

	nonRetryable := []ErrorCode{
		ErrCodeNotFound,
		ErrCodeInvalidInput,
		ErrCodeValidation,
	}

	for _, code := range nonRetryable {
		if code.IsRetryable() {
			t.Errorf("Code %s should not be retryable", code)
		}
	}
}

func TestErrorCodeIsClientError(t *testing.T) {
	clientErrors := []ErrorCode{
		ErrCodeNotFound,
		ErrCodeInvalidInput,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
	}

	for _, code := range clientErrors {
		if !code.IsClientError() {
			t.Errorf("Code %s should be a client error", code)
		}
		if code.IsServerError() {
			t.Errorf("Code %s should not be a server error", code)
		}
	}
}

func TestErrorCodeIsServerError(t *testing.T) {
	serverErrors := []ErrorCode{
		ErrCodeInternal,
		ErrCodeUnknown,
	}

	for _, code := range serverErrors {
		if !code.IsServerError() {
			t.Errorf("Code %s should be a server error", code)
		}
		if code.IsClientError() {
			t.Errorf("Code %s should not be a client error", code)
		}
	}
}

func TestMultipleContextAdditions(t *testing.T) {
	err := New(ErrCodeNotFound, "not found")
	err1 := err.WithContext("key1", "value1")
	err2 := err1.WithContext("key2", "value2")

	// Original should not be modified
	if len(err.Context()) != 0 {
		t.Error("Original error should not be modified")
	}

	// err1 should only have key1
	if len(err1.Context()) != 1 {
		t.Error("err1 should have 1 context item")
	}

	// err2 should have both
	if len(err2.Context()) != 2 {
		t.Error("err2 should have 2 context items")
	}

	if err2.Context()["key1"] != "value1" || err2.Context()["key2"] != "value2" {
		t.Error("err2 should preserve all context")
	}
}
