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
	addr string
}

// Option configures New.
type Option func(*fiber.Config)

// WithNetwork sets the listener network ("tcp4" default, "tcp6", "tcp").
func WithNetwork(network string) Option {
	return func(c *fiber.Config) { c.Network = network }
}

// WithTrustedProxies makes c.IP() the client IP taken from header when the
// connection comes from one of proxies (IPs or CIDRs). From anyone else,
// and when proxies is empty, c.IP() stays the socket peer: a forwarding
// header can't be spoofed to dodge per-client throttling.
func WithTrustedProxies(proxies []string, header string) Option {
	return func(c *fiber.Config) {
		if len(proxies) == 0 {
			return
		}
		c.EnableTrustedProxyCheck = true
		c.TrustedProxies = proxies
		c.ProxyHeader = header
		c.EnableIPValidation = true
	}
}

// New creates a configured Fiber server with the global error handler,
// listening on addr (host:port; empty host = all interfaces).
func New(addr string, opts ...Option) *Server {
	cfg := fiber.Config{ErrorHandler: ErrorMiddleware}
	for _, o := range opts {
		o(&cfg)
	}
	return &Server{app: fiber.New(cfg), addr: addr}
}

// App returns the underlying Fiber app for route registration.
func (s *Server) App() *fiber.App { return s.app }

// Start begins listening. Blocks until context is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Listen(s.addr)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down server...")
		return s.app.Shutdown()
	}
}
