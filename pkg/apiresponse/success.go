package apiresponse

import "github.com/gofiber/fiber/v3"

// SuccessEnvelope is the standard JSON envelope for successful API responses.
type SuccessEnvelope[T any] struct {
	Status  bool   `json:"status"`
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

// Success writes a standardized successful JSON response.
func Success[T any](c fiber.Ctx, status int, data T, message string) error {
	return c.Status(status).JSON(SuccessEnvelope[T]{
		Status:  true,
		Data:    data,
		Message: message,
	})
}
