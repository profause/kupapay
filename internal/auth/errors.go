package auth

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// AuthError is a user-facing auth failure carrying its own HTTP status. The
// handler maps any error implementing HTTPStatus() to that code, so adding a
// new sentinel doesn't require an edit in handler.go.
//
// Sentinel matching: each exported Err* is a unique *AuthError value, so
// errors.Is(err, ErrFoo) reduces to pointer equality (works through wrapping
// because AuthError unwraps to its Reason — typically a one-line shared
// sentinel only useful as a marker).
type AuthError struct {
	// Reason is an optional inner error used by errors.Is/As matching for
	// callers that want a categorical check (e.g. "any credential error").
	Reason error
	// Message is what users see. Distinct from Reason because some errors
	// intentionally lie to clients (login-forbidden returns the same
	// message as invalid-credentials so account state isn't probed).
	Message string
	// Status is the HTTP status code to return.
	Status int
}

func (e *AuthError) Error() string { return e.Message }

// HTTPStatus is what handler.authError dispatches on.
func (e *AuthError) HTTPStatus() int { return e.Status }

func (e *AuthError) Unwrap() error { return e.Reason }

// statusFor extracts an HTTP status from an error if any wrapped layer
// implements HTTPStatus(). Returns 0 on no match.
func statusFor(err error) int {
	var s interface{ HTTPStatus() int }
	if errors.As(err, &s) {
		return s.HTTPStatus()
	}
	return 0
}

// helpers for declaring the canonical sentinels concisely
func authErrBadRequest(msg string) *AuthError {
	return &AuthError{Message: msg, Status: fiber.StatusBadRequest}
}
func authErrUnauthorized(msg string) *AuthError {
	return &AuthError{Message: msg, Status: fiber.StatusUnauthorized}
}
func authErrForbidden(msg string) *AuthError {
	return &AuthError{Message: msg, Status: fiber.StatusForbidden}
}
func authErrNotFound(msg string) *AuthError {
	return &AuthError{Message: msg, Status: fiber.StatusNotFound}
}
