package customerror

import (
	"github.com/gofiber/fiber/v3"
	"github.com/profause/kupapay/pkg/apiresponse"
)

// ErrorResponse maps a service error to the appropriate HTTP status and returns
// a consistent JSON error envelope. It uses pkg/errors to extract typed error
// codes when available; untyped errors default to 500.
func ErrorResponse(c fiber.Ctx, err error) error {
	return apiresponse.Failure(c, HTTPStatus(err), err.Error())
}
