//go:build unix

package gatewayhttp

import (
	"context"
	"errors"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

// disconnectPollInterval is how often an in-flight request checks whether
// its client is still connected.
const disconnectPollInterval = 250 * time.Millisecond

// clientContext returns a context cancelled when the client of c closes its
// connection, so upstream work for an abandoned request stops (fasthttp's
// request context never ends while a handler runs). Call stop before the
// handler returns. Connections that expose no socket (TLS, in-memory test
// transports) are not watched.
func clientContext(c *fiber.Ctx) (ctx context.Context, stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sc, ok := c.Context().Conn().(syscall.Conn)
	if !ok {
		return ctx, cancel
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return ctx, cancel
	}

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(disconnectPollInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if peerClosed(raw) {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, func() {
		close(done)
		cancel()
	}
}

// peerClosed reports whether the peer has closed the connection, without
// consuming any data: a non-blocking MSG_PEEK reads 0 bytes only at EOF.
func peerClosed(raw syscall.RawConn) bool {
	closed := false
	buf := make([]byte, 1)
	_ = raw.Read(func(fd uintptr) bool {
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case err == nil:
			closed = n == 0
		case errors.Is(err, syscall.EAGAIN), errors.Is(err, syscall.EINTR):
		default:
			closed = true // ECONNRESET and friends
		}
		return true // never wait for readiness
	})
	return closed
}
