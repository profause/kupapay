package customerror

import "strconv"

// ErrorCode represents a typed error code for categorizing errors
type ErrorCode string

const (
	// General errors
	ErrCodeUnknown      ErrorCode = "UNKNOWN"
	ErrCodeInternal     ErrorCode = "INTERNAL"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"

	// Connection and network errors
	ErrCodeConnection   ErrorCode = "CONNECTION_FAILED"
	ErrCodeDisconnected ErrorCode = "DISCONNECTED"
	ErrCodeNetworkError ErrorCode = "NETWORK_ERROR"

	// Cache errors
	ErrCodeExpired         ErrorCode = "EXPIRED"
	ErrCodeSerialization   ErrorCode = "SERIALIZATION_FAILED"
	ErrCodeDeserialization ErrorCode = "DESERIALIZATION_FAILED"
	ErrCodeTypeMismatch    ErrorCode = "TYPE_MISMATCH"
	ErrCodeLockFailed      ErrorCode = "LOCK_FAILED"
	ErrCodeRateLimited     ErrorCode = "RATE_LIMITED"
	ErrCodeMaxSize         ErrorCode = "MAX_SIZE_EXCEEDED"
	ErrCodeMemory          ErrorCode = "MEMORY_LIMIT_EXCEEDED"

	// Queue errors
	ErrCodeQueueFull       ErrorCode = "QUEUE_FULL"
	ErrCodeMessageTooLarge ErrorCode = "MESSAGE_TOO_LARGE"
	ErrCodeInvalidMessage  ErrorCode = "INVALID_MESSAGE"
	ErrCodeDLQ             ErrorCode = "DEAD_LETTER_QUEUE"

	// Database errors
	ErrCodeDuplicate   ErrorCode = "DUPLICATE"
	ErrCodeTransaction ErrorCode = "TRANSACTION_FAILED"
	ErrCodeQueryFailed ErrorCode = "QUERY_FAILED"
	ErrCodeMigration   ErrorCode = "MIGRATION_FAILED"

	// Configuration errors
	ErrCodeConfig        ErrorCode = "CONFIGURATION_ERROR"
	ErrCodeMissingConfig ErrorCode = "MISSING_CONFIGURATION"
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIGURATION"

	// Dependency injection errors
	ErrCodeDependency    ErrorCode = "DEPENDENCY_ERROR"
	ErrCodeCircular      ErrorCode = "CIRCULAR_DEPENDENCY"
	ErrCodeNotRegistered ErrorCode = "NOT_REGISTERED"

	// Validation errors
	ErrCodeValidation ErrorCode = "VALIDATION_FAILED"
	ErrCodeRequired   ErrorCode = "REQUIRED_FIELD"
	ErrCodeFormat     ErrorCode = "INVALID_FORMAT"

	// Business logic errors
	ErrCodeBusiness      ErrorCode = "BUSINESS_RULE_VIOLATION"
	ErrCodePrecondition  ErrorCode = "PRECONDITION_FAILED"
	ErrCodePostcondition ErrorCode = "POSTCONDITION_FAILED"

	// Authentication/Authorization errors
	ErrAuthFailed            ErrorCode = "AUTHENTICATION_FAILED"
	ErrTokenGenerationFailed ErrorCode = "TOKEN_GENERATION_FAILED"
	ErrTokenNotFound         ErrorCode = "TOKEN_NOT_FOUND"
	ErrInvalidToken          ErrorCode = "INVALID_TOKEN"
	ErrExpiredToken          ErrorCode = "EXPIRED_TOKEN"
	ErrInvalidClaims         ErrorCode = "INVALID_CLAIMS"

	// Pagination errors
	ErrInvalidLimit  ErrorCode = "INVALID_LIMIT"
	ErrInvalidCursor ErrorCode = "INVALID_CURSOR"
)

// String returns the string representation of the error code
func (e ErrorCode) String() string {
	return string(e)
}

// HTTPStatusCode returns the suggested HTTP status code for the error code
func (e ErrorCode) HTTPStatusCode() int {
	if status, err := strconv.Atoi(string(e)); err == nil && status >= 100 && status <= 599 {
		return status
	}

	switch e {
	case ErrCodeNotFound:
		return 404
	case ErrCodeInvalidInput, ErrCodeValidation, ErrCodeRequired, ErrCodeFormat:
		return 400
	case ErrCodeUnauthorized:
		return 401
	case ErrCodeForbidden:
		return 403
	case ErrCodeConflict, ErrCodeDuplicate:
		return 409
	case ErrCodeTimeout:
		return 408
	case ErrCodeRateLimited:
		return 429
	case ErrCodeBusiness, ErrCodePrecondition, ErrCodePostcondition:
		return 422
	case ErrCodeMessageTooLarge, ErrCodeMaxSize:
		return 413
	default:
		return 500
	}
}

// IsRetryable returns true if the error is potentially retryable
func (e ErrorCode) IsRetryable() bool {
	switch e {
	case ErrCodeTimeout, ErrCodeConnection, ErrCodeNetworkError, ErrCodeDisconnected, ErrCodeQueueFull:
		return true
	default:
		return false
	}
}

// IsClientError returns true if the error is a client error (4xx)
func (e ErrorCode) IsClientError() bool {
	status := e.HTTPStatusCode()
	return status >= 400 && status < 500
}

// IsServerError returns true if the error is a server error (5xx)
func (e ErrorCode) IsServerError() bool {
	status := e.HTTPStatusCode()
	return status >= 500
}
