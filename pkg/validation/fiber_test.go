package validation

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type passwordValidationDTO struct {
	Password string `json:"password" validate:"required,password"`
}

func TestErrorResponseIncludesFieldValidationMessage(t *testing.T) {
	v := NewValidator()
	app := fiber.New()
	app.Post("/test", func(c fiber.Ctx) error {
		var dto passwordValidationDTO
		if err := v.BindAndValidate(c, &dto); err != nil {
			return v.ErrorResponse(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodPost, "/test", strings.NewReader(`{"password":"weak"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Second})
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	var payload struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Details []struct {
			Field   string `json:"field"`
			Value   any    `json:"value"`
			Tag     string `json:"tag"`
			Message string `json:"message"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Error == "Validation failed" {
		t.Fatalf("error should include the failed field, got %q", payload.Error)
	}
	if !strings.Contains(payload.Error, "password") || !strings.Contains(payload.Error, "uppercase") {
		t.Fatalf("error should explain password rules, got %q", payload.Error)
	}
	if payload.Code != "VALIDATION_FAILED" {
		t.Fatalf("code=%q", payload.Code)
	}
	if len(payload.Details) != 1 {
		t.Fatalf("details=%+v", payload.Details)
	}
	detail := payload.Details[0]
	if detail.Field != "password" || detail.Tag != "password" || !strings.Contains(detail.Message, "uppercase") {
		t.Fatalf("unexpected validation detail: %+v", detail)
	}
	if detail.Value != nil {
		t.Fatalf("password value should not be included in validation response: %+v", detail.Value)
	}
}
