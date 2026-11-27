package customerror

import (
	"errors"
	"fmt"
)

// Error represents a typed application error with context
type Error struct {
	code    ErrorCode
	message string
	cause   error
	context map[string]interface{}
	status  int
}

// New creates a new Error with the given code and message
func New(code ErrorCode, message string) *Error {
	return &Error{
		code:    code,
		message: message,
		context: make(map[string]interface{}),
	}
}

// FromHTTPStatus builds a typed Error whose HTTP response status matches the
// given upstream status code. Use it to propagate a downstream service's status
// instead of collapsing every failure into a generic 500.
func FromHTTPStatus(status int, message string) *Error {
	return New(codeForStatus(status), message).WithStatus(status)
}

// codeForStatus maps an HTTP status code to the closest ErrorCode.
func codeForStatus(status int) ErrorCode {
	switch status {
	case 400:
		return ErrCodeInvalidInput
	case 401:
		return ErrCodeUnauthorized
	case 403:
		return ErrCodeForbidden
	case 404:
		return ErrCodeNotFound
	case 408:
		return ErrCodeTimeout
	case 409:
		return ErrCodeConflict
	case 422:
		return ErrCodeBusiness
	case 429:
		return ErrCodeRateLimited
	default:
		if status >= 400 && status < 500 {
			return ErrCodeInvalidInput
		}
		return ErrCodeInternal
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Code returns the error code
func (e *Error) Code() ErrorCode {
	return e.code
}

// Message returns the error message
func (e *Error) Message() string {
	return e.message
}

// Cause returns the underlying cause of the error
func (e *Error) Cause() error {
	return e.cause
}

// Context returns the error context
func (e *Error) Context() map[string]interface{} {
	return e.context
}

// Unwrap returns the cause for customerror.Is and customerror.As compatibility
func (e *Error) Unwrap() error {
	return e.cause
}

// Wrap wraps another error as the cause
func (e *Error) Wrap(err error) *Error {
	newErr := *e
	newErr.cause = err
	return &newErr
}

// WithContext adds context to the error (key-value pairs)
func (e *Error) WithContext(key string, value interface{}) *Error {
	newErr := *e
	newErr.context = make(map[string]interface{})
	for k, v := range e.context {
		newErr.context[k] = v
	}
	newErr.context[key] = value
	return &newErr
}

// WithKey is a convenience method for adding a "key" context
func (e *Error) WithKey(key string) *Error {
	return e.WithContext("key", key)
}

// WithID is a convenience method for adding an "id" context
func (e *Error) WithID(id string) *Error {
	return e.WithContext("id", id)
}

// WithStatus sets an explicit HTTP status override for the error, taking
// precedence over the status derived from the error code.
func (e *Error) WithStatus(status int) *Error {
	newErr := *e
	newErr.status = status
	return &newErr
}

// HTTPStatus returns the explicit status override when set, otherwise the
// status derived from the error code.
func (e *Error) HTTPStatus() int {
	if e.status != 0 {
		return e.status
	}
	return e.code.HTTPStatusCode()
}

// Is checks if the error matches the target error
// This allows customerror.Is(err, SomeError) to work
func (e *Error) Is(target error) bool {
	var t *Error
	ok := errors.As(target, &t)
	if !ok {
		return false
	}
	return e.code == t.code
}

// As allows customerror.As to extract typed errors
func (e *Error) As(target interface{}) bool {
	if t, ok := target.(**Error); ok {
		*t = e
		return true
	}
	return false
}

// Is checks if an error is of a specific type
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target
func As(err error, target interface{}) bool {
	return errors.As(err, &target)
}

// Cause returns the underlying cause of an error
func Cause(err error) error {
	type causer interface {
		Cause() error
	}

	for err != nil {
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}
	return err
}

// GetCode extracts the error code from an error, returns ErrCodeUnknown if not a typed error
func GetCode(err error) ErrorCode {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.Code()
	}
	return ErrCodeUnknown
}

// HTTPStatus resolves the HTTP status for an error: the explicit override or
// code-derived status of a typed *Error, otherwise the status mapped from the
// error code (500 for untyped errors).
func HTTPStatus(err error) int {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.HTTPStatus()
	}
	return GetCode(err).HTTPStatusCode()
}

// GetContext extracts context from an error, returns nil if not a typed error
func GetContext(err error) map[string]interface{} {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.Context()
	}
	return nil
}

// HTTPStatusCode resolves the HTTP status for a typed *Error, honouring any
// explicit WithStatus override. Untyped errors resolve to fallback, letting
// proxy handlers surface upstream failures as 502 rather than 500.
func HTTPStatusCode(err error, fallback int) int {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.HTTPStatus()
	}
	return fallback
}
