package validation

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/profause/kupapay/pkg/apiresponse"
)

// BindAndValidate parses the request body into the target struct and validates it.
// Returns nil on success, or a ValidationErrors on validation failure.
// For parse errors (invalid JSON), returns a simple error.
func (v *Validator) BindAndValidate(c fiber.Ctx, out interface{}) error {
	// Parse body
	if err := c.Bind().Body(out); err != nil {
		return &ParseError{Err: err}
	}

	// Validate
	return v.Validate(out)
}

// BindQueryAndValidate parses query parameters into the target struct and validates it.
func (v *Validator) BindQueryAndValidate(c fiber.Ctx, out interface{}) error {
	if err := c.Bind().Query(out); err != nil {
		return &ParseError{Err: err}
	}
	return v.Validate(out)
}

// BindParamsAndValidate parses route parameters into the target struct and validates it.
func (v *Validator) BindParamsAndValidate(c fiber.Ctx, out interface{}) error {
	if err := c.Bind().URI(out); err != nil {
		return &ParseError{Err: err}
	}
	return v.Validate(out)
}

// ParseError represents a request parsing error
type ParseError struct {
	Err error
}

func (e *ParseError) Error() string {
	return "invalid request body"
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// ErrorResponse sends a standardized validation error response.
// Returns the appropriate fiber error to be returned from the handler.
func (v *Validator) ErrorResponse(c fiber.Ctx, err error) error {
	// collectionWebhookHandler parse errors
	if parseErr, ok := errors.AsType[*ParseError](err); ok {
		return apiresponse.FailureWithDetails(
			c,
			fiber.StatusBadRequest,
			"Invalid request body",
			"INVALID_INPUT",
			parseErr.Err.Error(),
		)
	}

	// collectionWebhookHandler validation errors
	if validationErrs, ok := AsValidationErrors(err); ok {
		return apiresponse.FailureWithDetails(
			c,
			fiber.StatusBadRequest,
			validationErrs.Error(),
			"VALIDATION_FAILED",
			validationErrs.Errors,
		)
	}

	// Fallback for unknown errors
	return apiresponse.FailureWithDetails(
		c,
		fiber.StatusBadRequest,
		err.Error(),
		"BAD_REQUEST",
		nil,
	)
}

// ValidateMiddleware creates a Fiber middleware that validates the request body
// against the provided struct type. Use this for routes where you want automatic
// validation without manual BindAndValidate calls.
//
// Usage:
//
//	app.Post("/users", validation.ValidateMiddleware[CreateUserDTO](v), handler)
func ValidateMiddleware[T any](validator *Validator) fiber.Handler {
	return func(c fiber.Ctx) error {
		var target T
		if err := validator.BindAndValidate(c, &target); err != nil {
			return validator.ErrorResponse(c, err)
		}
		// Store validated struct in locals for handler access
		fiber.Locals[*T](c, "validated", &target)
		return c.Next()
	}
}

// GetValidated retrieves the validated struct from Fiber locals.
// Use this in handlers after ValidateMiddleware.
//
// Usage:
//
//	dto := validation.GetValidated[CreateUserDTO](c)
func GetValidated[T any](c fiber.Ctx) *T {
	return fiber.Locals[*T](c, "validated")
}

// BindAndValidateForm parses multipart/form-data fields into the target struct and validates it.
// Returns nil on success, or a ValidationErrors on validation failure.
// For parse errors, returns a simple error.
func (v *Validator) BindAndValidateForm(c fiber.Ctx, out interface{}) error {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return &ParseError{Err: err}
	}

	// Convert form values into map
	values := make(map[string]interface{})

	for key, val := range form.Value {
		if len(val) > 0 {
			values[key] = val[0]
		}
	}

	// Convert map to JSON
	data, err := json.Marshal(values)
	if err != nil {
		return &ParseError{Err: err}
	}

	// Bind into struct
	if err := json.Unmarshal(data, out); err != nil {
		return &ParseError{Err: err}
	}

	// Validate struct tags
	return v.Validate(out)
}
