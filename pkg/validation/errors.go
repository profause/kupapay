package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidationError represents a single field validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value,omitempty"`
	Tag     string      `json:"tag"`
	Param   string      `json:"param,omitempty"`
	Message string      `json:"message"`
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents a collection of validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error implements the error interface
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var sb strings.Builder
	sb.WriteString("validation failed: ")
	for i, err := range e.Errors {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Fields returns a slice of field names that failed validation
func (e *ValidationErrors) Fields() []string {
	fields := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		fields = append(fields, err.Field)
	}
	return fields
}

// ByField returns errors for a specific field
func (e *ValidationErrors) ByField(field string) []ValidationError {
	var errs []ValidationError
	for _, err := range e.Errors {
		if err.Field == field {
			errs = append(errs, err)
		}
	}
	return errs
}

// NewValidationErrors converts go-playground/validator errors to our format
func NewValidationErrors(errs validator.ValidationErrors) *ValidationErrors {
	validationErrors := make([]ValidationError, 0, len(errs))

	for _, err := range errs {
		validationErrors = append(validationErrors, ValidationError{
			Field:   extractFieldName(err),
			Value:   validationErrorValue(err),
			Tag:     err.Tag(),
			Param:   err.Param(),
			Message: humanReadableMessage(err),
		})
	}

	return &ValidationErrors{Errors: validationErrors}
}

// extractFieldName extracts the JSON field name from the validator error
func extractFieldName(err validator.FieldError) string {
	// Use the namespace for nested fields, but convert to JSON-style path
	namespace := err.Namespace()
	// Remove the struct prefix (e.g., "CreateUserDTO.email" -> "email")
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return err.Field()
}

// humanReadableMessage generates a human-readable error message for common validation tags
func humanReadableMessage(err validator.FieldError) string {
	field := extractFieldName(err)
	param := err.Param()

	switch err.Tag() {
	// Required/existence
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "required_if":
		return fmt.Sprintf("%s is required", field)
	case "required_unless":
		return fmt.Sprintf("%s is required", field)
	case "required_with":
		return fmt.Sprintf("%s is required when %s is present", field, param)
	case "required_without":
		return fmt.Sprintf("%s is required when %s is not present", field, param)

	// String validations
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "uri":
		return "must be a valid URI"
	case "uuid":
		return "must be a valid UUID"
	case "uuid3":
		return "must be a valid UUID v3"
	case "uuid4":
		return "must be a valid UUID v4"
	case "uuid5":
		return "must be a valid UUID v5"
	case "alpha":
		return "must contain only letters"
	case "alphanum":
		return "must contain only letters and numbers"
	case "alphanumunicode":
		return "must contain only unicode letters and numbers"
	case "numeric":
		return "must be a numeric value"
	case "hexadecimal":
		return "must be a hexadecimal value"
	case "hexcolor":
		return "must be a valid hex color"
	case "rgb":
		return "must be a valid RGB color"
	case "rgba":
		return "must be a valid RGBA color"
	case "lowercase":
		return "must be lowercase"
	case "uppercase":
		return "must be uppercase"
	case "ascii":
		return "must contain only ASCII characters"
	case "printascii":
		return "must contain only printable ASCII characters"
	case "base64":
		return "must be a valid base64 string"
	case "json":
		return "must be a valid JSON string"
	case "jwt":
		return "must be a valid JWT token"
	case "hostname":
		return "must be a valid hostname"
	case "fqdn":
		return "must be a fully qualified domain name"
	case "ip":
		return "must be a valid IP address"
	case "ipv4":
		return "must be a valid IPv4 address"
	case "ipv6":
		return "must be a valid IPv6 address"
	case "cidr":
		return "must be a valid CIDR notation"
	case "mac":
		return "must be a valid MAC address"

	// Length/size constraints
	case "len":
		return fmt.Sprintf("must be exactly %s characters", param)
	case "min":
		return fmt.Sprintf("must be at least %s characters", param)
	case "max":
		return fmt.Sprintf("must be at most %s characters", param)

	// Numeric comparisons
	case "eq":
		return fmt.Sprintf("must be equal to %s", param)
	case "ne":
		return fmt.Sprintf("must not be equal to %s", param)
	case "gt":
		return fmt.Sprintf("must be greater than %s", param)
	case "gte":
		return fmt.Sprintf("must be at least %s", param)
	case "lt":
		return fmt.Sprintf("must be less than %s", param)
	case "lte":
		return fmt.Sprintf("must be at most %s", param)

	// Field comparisons
	case "eqfield":
		return fmt.Sprintf("must be equal to %s", param)
	case "nefield":
		return fmt.Sprintf("must not be equal to %s", param)
	case "gtfield":
		return fmt.Sprintf("must be greater than %s", param)
	case "gtefield":
		return fmt.Sprintf("must be greater than or equal to %s", param)
	case "ltfield":
		return fmt.Sprintf("must be less than %s", param)
	case "ltefield":
		return fmt.Sprintf("must be less than or equal to %s", param)

	// Enum/choice
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(param, " ", ", "))

	// Contains/excludes
	case "contains":
		return fmt.Sprintf("must contain '%s'", param)
	case "containsany":
		return fmt.Sprintf("must contain at least one of '%s'", param)
	case "excludes":
		return fmt.Sprintf("must not contain '%s'", param)
	case "excludesall":
		return fmt.Sprintf("must not contain any of '%s'", param)
	case "startswith":
		return fmt.Sprintf("must start with '%s'", param)
	case "endswith":
		return fmt.Sprintf("must end with '%s'", param)

	// Date/time
	case "datetime":
		return fmt.Sprintf("must be a valid datetime in format %s", param)

	// File/MIME
	case "file":
		return "must be a valid file"
	case "image":
		return "must be a valid image file"

	// Other
	case "unique":
		return "must contain unique values"
	case "isdefault":
		return "must be the default value"
	case "boolean":
		return "must be a boolean value"

	// Project-specific validators
	case "password":
		return "must be at least 8 characters and include uppercase, lowercase, number, and special character"
	case "slug":
		return "must be lowercase letters, numbers, and single hyphens only"
	case "phone":
		return "must be a valid international phone number"
	case "currency":
		return "must be a valid currency code"
	case "nowhitespace":
		return "must not contain whitespace"
	case "nohtml":
		return "must not contain HTML"

	// Credit cards
	case "credit_card":
		return "must be a valid credit card number"
	case "isbn":
		return "must be a valid ISBN"
	case "isbn10":
		return "must be a valid ISBN-10"
	case "isbn13":
		return "must be a valid ISBN-13"

	// Custom/unknown - return structured message
	default:
		if param != "" {
			return fmt.Sprintf("failed %s validation with parameter '%s'", err.Tag(), param)
		}
		return fmt.Sprintf("failed %s validation", err.Tag())
	}
}

func validationErrorValue(err validator.FieldError) any {
	field := strings.ToLower(extractFieldName(err))
	tag := strings.ToLower(err.Tag())
	for _, sensitive := range []string{"password", "pin", "token", "secret", "key", "otp", "code"} {
		if strings.Contains(field, sensitive) || strings.Contains(tag, sensitive) {
			return nil
		}
	}
	return err.Value()
}

// IsValidationError checks if an error is a ValidationErrors type
func IsValidationError(err error) bool {
	var validationErrors *ValidationErrors
	ok := errors.As(err, &validationErrors)
	return ok
}

// AsValidationErrors attempts to convert an error to ValidationErrors
func AsValidationErrors(err error) (*ValidationErrors, bool) {
	var ve *ValidationErrors
	ok := errors.As(err, &ve)
	return ve, ok
}
