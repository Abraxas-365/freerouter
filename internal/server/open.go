package server

import (
	"net"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// LocalSubject is the principal subject of every caller in none mode.
const LocalSubject = "local"

// Open is FREEROUTER_AUTH=none: every local caller is the owner, holding
// every permission. The process only listens on loopback (config.Validate),
// and Open trusts a request only when it is local in fact, not just on a
// loopback socket:
//   - Host must be a loopback name. A page on another site that re-points
//     its own hostname at 127.0.0.1 (DNS rebinding) is same-origin to the
//     browser, but its requests carry the foreign Host.
//   - No forwarding headers. A reverse proxy or tunnel on this machine
//     (Caddy, nginx, cloudflared, ngrok, tailscale serve) would turn remote
//     callers into loopback connections; they all add X-Forwarded-For or
//     similar. Vite's dev proxy adds none, so `npm run dev` keeps working.
//   - No browser request from another origin, loopback ones included, unless
//     the operator listed it (ConsoleOrigins, e.g. the console on Vite's
//     port calling :3000 directly). CORS allows any origin, so without this
//     any web page — or any other local app — could drive the open API.
//
// A gateway key (fr_...) sent anyway is resolved as that key's principal (a
// wrong key is 401, never ignored); any other bearer is ignored, so SDKs
// that insist on some API key work unchanged.
type Open struct {
	Keys KeyResolver // optional
	// ConsoleOrigins are extra trusted browser origins; only loopback ones
	// count (none mode is local only).
	ConsoleOrigins []string
}

var _ Authenticator = Open{}

func (o Open) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !o.allow(c) {
			return nil // refusal written
		}
		return c.Next()
	}
}

// allow checks the request is local and sets its principal. On false the
// refusal has been written and the request must stop.
func (o Open) allow(c *fiber.Ctx) bool {
	refuse := func(status int, code, msg string) bool {
		_ = authError(c, status, code, msg)
		return false
	}
	if !loopbackHost(string(c.Request().Host())) {
		return refuse(fiber.StatusForbidden, "FORBIDDEN", "FREEROUTER_AUTH=none only answers to localhost")
	}
	if h := forwardedBy(c); h != "" {
		return refuse(fiber.StatusForbidden, "FORBIDDEN",
			"FREEROUTER_AUTH=none refuses proxied requests ("+h+"); use FREEROUTER_AUTH=token behind a proxy")
	}
	if !o.localOrigin(c) {
		return refuse(fiber.StatusForbidden, "FORBIDDEN", "cross-origin request rejected")
	}
	p := Principal{Subject: LocalSubject, Permissions: LocalPermissions()}
	normalizeAPIKeyHeader(c)
	if key := bearer(c); strings.HasPrefix(key, LocalKeyPrefix) && o.Keys != nil {
		resolved, err := o.Keys.ResolveKey(c.UserContext(), key)
		if err != nil {
			return refuse(fiber.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
		}
		p = resolved
	}
	SetPrincipal(c, p)
	return true
}

func (o Open) Session() SessionRoutes {
	// Through allow: a rebinding page must not learn the API is open.
	state := func(c *fiber.Ctx) error {
		if !o.allow(c) {
			return nil
		}
		return c.JSON(SessionState{Authenticated: true, Subject: LocalSubject, Permissions: LocalPermissions()})
	}
	return SessionRoutes{Current: state, Login: state,
		Logout: func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }}
}

// forwardingHeaders mark a request relayed by a proxy or tunnel. A raw TCP
// forwarder (socat, ssh -R) adds none and connects from loopback like a
// local client; exposing a loopback port that way is the operator's choice
// and the docs say to use token mode for anything reachable remotely.
var forwardingHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host",
	"X-Forwarded-Proto", "X-Forwarded-Server", "X-Real-Ip", "X-Client-Ip", "Via",
	"Cf-Connecting-Ip", "True-Client-Ip", "Tailscale-User-Login"}

// forwardedBy names the first forwarding header present, or "".
func forwardedBy(c *fiber.Ctx) string {
	for _, h := range forwardingHeaders {
		if len(c.Request().Header.PeekAll(h)) > 0 {
			return h
		}
	}
	return ""
}

// loopbackHost: localhost or a loopback IP, with or without a port.
func loopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// localOrigin is false for a browser request made by any page other than
// the console itself: same origin, or a loopback origin the operator listed
// in ConsoleOrigins — not merely any loopback origin (any other local app
// or dev server would otherwise hold the owner's access). Vite's dev proxy
// keeps the browser's Host, so the console under `npm run dev` is
// same-origin. Non-browser clients send neither header.
func (o Open) localOrigin(c *fiber.Ctx) bool {
	origin := c.Get("Origin")
	if origin != "" && o.consoleOrigin(origin) {
		return true
	}
	switch c.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") &&
		strings.EqualFold(u.Host, string(c.Request().Host()))
}

func (o Open) consoleOrigin(origin string) bool {
	for _, allowed := range o.ConsoleOrigins {
		if strings.EqualFold(strings.TrimSuffix(allowed, "/"), origin) {
			u, err := url.Parse(origin)
			return err == nil && u.Scheme == "http" && loopbackHost(u.Host)
		}
	}
	return false
}
