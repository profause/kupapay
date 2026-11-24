//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d . -o ../../docs

package main

import (
	"context"
	"fmt"

	"github.com/gofiber/contrib/v3/swagger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/joho/godotenv"
	"github.com/profause/kupapay/docs"
	"github.com/profause/kupapay/internal/fx"
	"github.com/profause/kupapay/pkg/env"
	uberfx "go.uber.org/fx"
)

// @title KupaPay API
// @version 1.0
// @description API documentation for KupaPay.
// @BasePath /
func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println(".env not found")
	}
	app := uberfx.New(
		fx.AppModule,
		uberfx.Provide(registerRoutes),
		uberfx.Invoke(startServer),
	)
	app.Run()
}

func registerRoutes() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: env.GetString("APP_NAME", "kupapay"),
	})
	app.Use(cors.New())

	// Health Check
	app.Get("/health", healthCheck)

	app.Use(swagger.New(swagger.Config{
		FilePath:    "swagger/doc.json",
		Path:        "swagger",
		Title:       "KupaPay API Documentation",
		FileContent: []byte(docs.SwaggerInfo.ReadDoc()),
	}))

	// API Group
	_ = app.Group("/api/v1")
	return app
}

// healthCheck godoc
// @Summary      Check API health
// @Description  Returns the current health status of the API.
// @Tags         health
// @Produce      json
// @Success      200 {object} map[string]string "API is healthy"
// @Router       /health [get]
func healthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func startServer(lc uberfx.Lifecycle, app *fiber.App, shutdowner uberfx.Shutdowner) {
	stopping := make(chan struct{})

	lc.Append(uberfx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				addr := fmt.Sprintf(":%s", env.GetString("APP_ADDR", "3000"))
				println("Starting server on", addr)
				if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
					select {
					case <-stopping:
						//shutdown requested
					default:
						_ = shutdowner.Shutdown()
					}
					println("Failed to start server", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(stopping)
			return app.Shutdown()
		},
	})
}
