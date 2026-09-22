package server

import (
	"context"
	"errors"
	"log"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/gofiber/fiber/v2"
)

// ErrorMiddleware is Fiber's global error handler. It converts *errx.Error into
// structured JSON responses. Unknown errors become 500 with a generic message.
func ErrorMiddleware(c *fiber.Ctx, err error) error {
	// Fiber's own errors (404 route not found, etc.)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(errx.HTTPErrorResponse{
			Code:       "FIBER_ERROR",
			Message:    fiberErr.Message,
			Type:       "INTERNAL",
			StatusCode: fiberErr.Code,
		})
	}

	// Application errors
	var appErr *errx.Error
	if errors.As(err, &appErr) {
		return c.Status(appErr.HTTPStatus).JSON(appErr.ToHTTPResponse())
	}

	// Unknown errors — log but don't leak details
	log.Printf("unhandled error: %v", err)
	return c.Status(500).JSON(errx.HTTPErrorResponse{
		Code:       "INTERNAL",
		Message:    "an unexpected error occurred",
		Type:       "INTERNAL",
		StatusCode: 500,
	})
}

// RouteRegistrar is a function that registers routes on a fiber.Router.
type RouteRegistrar func(router fiber.Router)

// Server wraps a Fiber app with structured startup/shutdown.
type Server struct {
	app  *fiber.App
	port string
}

// New creates a configured Fiber server with the global error handler.
func New(port string) *Server {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorMiddleware,
	})

	return &Server{app: app, port: port}
}

// App returns the underlying Fiber app for route registration.
func (s *Server) App() *fiber.App { return s.app }

// Start begins listening. Blocks until context is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Listen(":" + s.port)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down server...")
		return s.app.Shutdown()
	}
}
