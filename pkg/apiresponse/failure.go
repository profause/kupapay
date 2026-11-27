package apiresponse

import "github.com/gofiber/fiber/v3"

// FailureEnvelope is the standard JSON envelope for failed API responses.
type FailureEnvelope struct {
	Status  bool   `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ValidationEnvelope struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// Failure writes a standardized failed JSON response with an error message.
func Failure(c fiber.Ctx, status int, errMsg string) error {
	return c.Status(status).JSON(FailureEnvelope{
		Status:  false,
		Message: errMsg,
		Data:    nil,
	})
}

// FailureCode writes a standardized failure carrying a stable machine-readable
// code alongside the human message, in the standard envelope. The code lets
// merchants branch on the failure programmatically (see the singular money API
// error taxonomy); the message stays the human copy.
func FailureCode(c fiber.Ctx, status int, code, errMsg string) error {
	return c.Status(status).JSON(FailureEnvelope{
		Status:  false,
		Code:    code,
		Message: errMsg,
		Data:    nil,
	})
}

// FailureWithDetails writes a standardized failed JSON response with optional metadata.
func FailureWithDetails(c fiber.Ctx, status int, errMsg string, code string, details any) error {
	return c.Status(status).JSON(ValidationEnvelope{
		Error:   errMsg,
		Code:    code,
		Details: details,
	})
}
