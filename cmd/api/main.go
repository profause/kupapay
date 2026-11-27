//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d .,../../internal/auth,../../pkg/apiresponse -o ../../docs

package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"github.com/profause/kupapay/internal/fx"
	"github.com/profause/kupapay/pkg/apiresponse"
	"github.com/profause/kupapay/pkg/env"
	uberfx "go.uber.org/fx"
)

// @title KupaPay API
// @version 1.0
// @description API documentation for KupaPay. Protected /api/v1 routes accept an API key or a Bearer JWT; sign-up and token issuance are public.
// @BasePath /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Use the value "Bearer <token>".
func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println(".env not found")
	}
	app := uberfx.New(
		fx.AppModule,
		uberfx.Provide(registerRoutes),
		uberfx.Invoke(startServer),
		//uberfx.NopLogger,
	)
	app.Run()
}

// func registerRoutes(authService *auth.Service) *fiber.App {
// 	app := fiber.New(fiber.Config{
// 		AppName:      env.GetString("APP_NAME", "kupapay"),
// 		ErrorHandler: apiErrorHandler,
// 	})
// 	app.Use(cors.New(cors.Config{
// 		AllowHeaders: []string{
// 			fiber.HeaderOrigin,
// 			fiber.HeaderContentType,
// 			fiber.HeaderAccept,
// 			fiber.HeaderAuthorization,
// 			"X-API-Key",
// 		},
// 	}))
// 	app.Use("/api/v1", authService.Middleware())

// 	app.Post("/api/v1/auth/signup", authService.SignUp)
// 	app.Post("/api/v1/auth/token", authService.CreateToken)

// 	// Health Check
// 	app.Get("/health", healthCheck)

// 	app.Use(swagger.New(swagger.Config{
// 		FilePath:    "swagger/doc.json",
// 		Path:        "swagger",
// 		Title:       "KupaPay API Documentation",
// 		FileContent: []byte(docs.SwaggerInfo.ReadDoc()),
// 	}))

// 	return app
// }

// healthCheck godoc
// @Summary      Check API health
// @Description  Returns the current health status of the API.
// @Tags         health
// @Produce      json
// @Success      200 {object} apiresponse.SuccessDoc{data=map[string]string} "API is healthy"
// @Failure      500 {object} apiresponse.FailureDoc
// @Router       /health [get]

func apiErrorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "internal server error"
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
		if status < fiber.StatusInternalServerError {
			message = fiberErr.Message
		}
	}
	if status >= fiber.StatusInternalServerError {
		log.Printf("HTTP request failed: %v", err)
	}
	return apiresponse.Failure(c, status, message)
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
