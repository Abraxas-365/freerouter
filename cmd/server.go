package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/logx"
	// manifesto:server-imports
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		logx.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize Logger
	switch cfg.Server.LogLevel {
	case "debug":
		logx.SetLevel(logx.LevelDebug)
	case "warn":
		logx.SetLevel(logx.LevelWarn)
	case "error":
		logx.SetLevel(logx.LevelError)
	default:
		logx.SetLevel(logx.LevelInfo)
	}

	logx.Info("Starting freerouter API Server...")
	logx.Infof("Environment: %s", cfg.Server.Environment)

	// 3. Initialize Dependency Container
	container := NewContainer(cfg)
	defer container.Cleanup()

	// 4. Start background services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	container.StartBackgroundServices(ctx)

	// 5. Create Fiber App
	app := fiber.New(fiber.Config{
		AppName:               "freerouter API",
		DisableStartupMessage: true,
		ErrorHandler:          globalErrorHandler(cfg),
		BodyLimit:             10 * 1024 * 1024, // 10MB
		IdleTimeout:           120,
		EnablePrintRoutes:     false,
	})

	// 6. Global Middleware
	setupMiddleware(app, cfg)

	// 7. Health Check & Info
	app.Get("/health", healthCheckHandler(container))
	app.Get("/", infoHandler(cfg))

	// 8. Register Routes
	registerRoutes(app, container)

	// 9. 404 Handler
	app.Use(notFoundHandler)

	// 10. Print Route Summary
	printRouteSummary()

	// 11. Start Server with Graceful Shutdown
	startServer(app, cfg, cancel)
}

// ============================================================================
// Middleware
// ============================================================================

func setupMiddleware(app *fiber.App, cfg *config.Config) {
	// Panic recovery
	app.Use(recover.New(recover.Config{
		EnableStackTrace: cfg.IsDevelopment(),
	}))

	// Request ID
	app.Use(requestid.New(requestid.Config{
		Header: "X-Request-ID",
		Generator: func() string {
			return "req-" + randomString(16)
		},
	}))

	// CORS
	corsOrigins := "*"
	if len(cfg.Server.CORSOrigins) > 0 {
		corsOrigins = ""
		for i, origin := range cfg.Server.CORSOrigins {
			if i > 0 {
				corsOrigins += ","
			}
			corsOrigins += origin
		}
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-API-Key, X-Request-ID",
		AllowMethods:     "GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS",
		AllowCredentials: true,
		ExposeHeaders:    "X-Request-ID",
	}))

	// Request logger
	logFormat := "${time} | ${status} | ${latency} | ${method} ${path}"
	if cfg.IsDevelopment() {
		logFormat += " | ${ip} | ${reqHeader:X-Request-ID}\n"
	} else {
		logFormat += "\n"
	}

	app.Use(logger.New(logger.Config{
		Format:     logFormat,
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))
}

// ============================================================================
// Routes
// ============================================================================

func registerRoutes(app *fiber.App, container *Container) {
	logx.Info("Registering routes...")

		// IAM Routes
	container.IAM.OAuthHandlers.RegisterRoutes(app)
	logx.Info("  > OAuth routes registered")

	container.IAM.PasswordlessHandlers.RegisterRoutes(app)
	logx.Info("  > Passwordless auth routes registered")

	// Stripe webhook (public — Stripe signs requests, verified in handler)
	container.Billing.Handlers.RegisterPublicRoutes(app)
	logx.Info("  > Stripe webhook route registered")

	// manifesto:public-routes

		protected := app.Group("/api/v1",
		container.IAM.UnifiedAuthMiddleware.Authenticate(),
	)

		container.IAM.APIKeyHandlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > API key routes registered")

	container.IAM.InvitationHandlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Invitation routes registered")

	container.IAM.RoleHandlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Role routes registered")

	container.IAM.UserHandlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > User routes registered")

	// AI Routes
	container.AI.Provider.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Provider routes registered")

	container.AI.ProviderKey.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Provider key routes registered")

	container.AI.Usage.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Usage routes registered")

	container.Billing.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Billing routes registered")

	container.AI.Guardrails.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Guardrails routes registered")

	container.Webhook.Handlers.RegisterRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Webhook routes registered")

	// Gateway (on root, not under /api/v1 — OpenAI-compatible /v1/chat/completions)
	container.AI.Gateway.Handlers.RegisterRoutes(app, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Gateway routes registered")

	// Rate limit config CRUD (under /api/v1)
	container.AI.Gateway.Handlers.RegisterAdminRoutes(protected, container.IAM.UnifiedAuthMiddleware)
	logx.Info("  > Rate limit config routes registered")

	// Prometheus metrics endpoint
	app.Get("/metrics", adaptor.HTTPHandler(
		promhttp.HandlerFor(container.AI.Gateway.Metrics.Registry, promhttp.HandlerOpts{}),
	))
	logx.Info("  > Metrics endpoint registered")

	// manifesto:route-registration

	logx.Info("All routes registered")
}


// ============================================================================
// Handlers
// ============================================================================

func healthCheckHandler(container *Container) fiber.Handler {
	return func(c *fiber.Ctx) error {
		health := fiber.Map{
			"status":      "healthy",
			"service":     "freerouter",
			"environment": container.Config.Server.Environment,
			"timestamp":   fmt.Sprintf("%d", c.Context().Time().Unix()),
		}

		// Check database
		if err := container.DB.Ping(); err != nil {
			health["db"] = "unhealthy"
			health["db_error"] = err.Error()
			health["status"] = "degraded"
		} else {
			health["db"] = "healthy"
		}

		// Check Redis
		if _, err := container.Redis.Ping(c.Context()).Result(); err != nil {
			health["redis"] = "unhealthy"
			health["redis_error"] = err.Error()
			health["status"] = "degraded"
		} else {
			health["redis"] = "healthy"
		}

		status := fiber.StatusOK
		if health["status"] == "degraded" {
			status = fiber.StatusServiceUnavailable
		}

		return c.Status(status).JSON(health)
	}
}

func infoHandler(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service":     "freerouter",
			"version":     "1.0.0",
			"environment": cfg.Server.Environment,
			"endpoints": fiber.Map{
				"health": "/health",
				"api":    "/api/v1",
			},
		})
	}
}

func notFoundHandler(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error":      "Route not found",
		"code":       "NOT_FOUND",
		"path":       c.Path(),
		"method":     c.Method(),
		"request_id": c.Get("X-Request-ID"),
	})
}

// ============================================================================
// Error Handler
// ============================================================================

func globalErrorHandler(cfg *config.Config) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		logx.WithFields(logx.Fields{
			"path":       c.Path(),
			"method":     c.Method(),
			"ip":         c.IP(),
			"request_id": c.Get("X-Request-ID"),
			"user_agent": c.Get("User-Agent"),
		}).Errorf("Request error: %v", err)

		// Fiber error
		if e, ok := err.(*fiber.Error); ok {
			return c.Status(e.Code).JSON(fiber.Map{
				"error":      e.Message,
				"code":       "FIBER_ERROR",
				"status":     e.Code,
				"request_id": c.Get("X-Request-ID"),
			})
		}

		// errx.Error
		if e, ok := err.(*errx.Error); ok {
			response := fiber.Map{
				"error":      e.Message,
				"code":       e.Code,
				"type":       string(e.Type),
				"status":     e.HTTPStatus,
				"request_id": c.Get("X-Request-ID"),
			}
			if len(e.Details) > 0 {
				response["details"] = e.Details
			}
			if cfg.IsDevelopment() && e.Err != nil {
				response["underlying_error"] = e.Err.Error()
			}
			return c.Status(e.HTTPStatus).JSON(response)
		}

		// Unknown error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      "Internal Server Error",
			"code":       "INTERNAL_ERROR",
			"type":       "INTERNAL",
			"message":    "An unexpected error occurred",
			"request_id": c.Get("X-Request-ID"),
		})
	}
}

// ============================================================================
// Server Lifecycle
// ============================================================================

func startServer(app *fiber.App, cfg *config.Config, cancel context.CancelFunc) {
	port := fmt.Sprintf("%d", cfg.Server.Port)

	go func() {
		logx.Info(repeatString("=", 70))
		logx.Infof("Server listening on port %s", port)
		logx.Infof("Health: http://localhost:%s/health", port)
		logx.Infof("Environment: %s", cfg.Server.Environment)
		logx.Info(repeatString("=", 70))

		if err := app.Listen(":" + port); err != nil {
			logx.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigChan
	logx.Infof("Received signal: %v", sig)
	logx.Info("Shutting down gracefully...")

	cancel()

	if err := app.ShutdownWithTimeout(30); err != nil {
		logx.Errorf("Server forced to shutdown: %v", err)
	}

	logx.Info("Server exited successfully")
}

// ============================================================================
// Utilities
// ============================================================================

func printRouteSummary() {
	logx.Info("Route Summary:")
	logx.Info("   |- Health: /health")
	logx.Info("   |- Info: /")
	logx.Info("   |- API: /api/v1/*")
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

func repeatString(s string, count int) string {
	result := ""
	for range count {
		result += s
	}
	return result
}
