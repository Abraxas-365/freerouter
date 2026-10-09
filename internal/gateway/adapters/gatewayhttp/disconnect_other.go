//go:build !unix

package gatewayhttp

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// clientContext returns a context for upstream work. Client disconnects are
// only detected on unix; elsewhere the context ends when stop is called.
func clientContext(_ *fiber.Ctx) (context.Context, func()) {
	return context.WithCancel(context.Background())
}
