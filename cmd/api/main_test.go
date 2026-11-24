package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSwaggerDocsRoutes(t *testing.T) {
	app := registerRoutes()

	uiResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/swagger", http.NoBody))
	if err != nil {
		t.Fatal(err)
	}
	defer uiResponse.Body.Close()
	if uiResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /swagger status = %d, want %d", uiResponse.StatusCode, http.StatusOK)
	}

	specResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/swagger/doc.json", http.NoBody))
	if err != nil {
		t.Fatal(err)
	}
	defer specResponse.Body.Close()
	if specResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /swagger/doc.json status = %d, want %d", specResponse.StatusCode, http.StatusOK)
	}

	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.NewDecoder(specResponse.Body).Decode(&spec); err != nil {
		t.Fatal(err)
	}
	if _, ok := spec.Paths["/health"]; !ok {
		t.Fatal("Swagger spec does not document /health")
	}
}

func TestHealthRouteUnaffected(t *testing.T) {
	app := registerRoutes()

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", http.NoBody))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Fatalf("GET /health body = %s, want %s", body, `{"status":"ok"}`)
	}
}
