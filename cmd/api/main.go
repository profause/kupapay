package main

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/joho/godotenv"
	"github.com/profause/kupapay/internal/fx"
	"github.com/profause/kupapay/pkg/env"
	uberfx "go.uber.org/fx"
)

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
	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	// API Group
	_ = app.Group("/api/v1")
	return app
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
