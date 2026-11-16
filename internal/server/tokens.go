package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AdminSubject is the principal subject of the admin token (token mode).
const AdminSubject = "admin"

// SessionCookie carries a console session in token mode.
const SessionCookie = "freerouter_session"

// SessionTTL is how long a console session lasts in token mode.
const SessionTTL = 12 * time.Hour

// SessionState is GET /api/v1/session: whether this browser is signed in,
// as whom, and what it may do (the console hides what it can't).
type SessionState struct {
	Authenticated bool       `json:"authenticated"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	Subject       string     `json:"subject,omitempty"`
	Permissions   []string   `json:"permissions"`
}

// Tokens is FREEROUTER_AUTH=token: one static admin token (bearer, or traded
// for an HttpOnly session cookie by the console) holding every permission,
// and gateway keys (fr_...) issued from the console, each with its grants.
// No IAMKit. Build it with NewTokens: guesses at the admin token are
// throttled per client address.
type Tokens struct {
	AdminToken string
	Keys       KeyResolver

	guesses *failureLimiter // nil: unthrottled (tests)
}

// Admin-token guesses allowed per client address and window. With the
// 32-character minimum this is defense in depth, and the short window caps
// how long a misbehaving client on the same address can lock the admin out.
const (
	adminGuessLimit  = 10
	adminGuessWindow = time.Minute
)

// NewTokens is the token-mode authenticator with admin-token throttling.
func NewTokens(adminToken string, keys KeyResolver) Tokens {
	return Tokens{AdminToken: adminToken, Keys: keys, guesses: newFailureLimiter(adminGuessLimit, adminGuessWindow)}
}

var _ Authenticator = Tokens{}

// throttled answers 429 when this client has guessed wrong too often.
func (t Tokens) throttled(c *fiber.Ctx) bool {
	if t.guesses == nil {
		return false
	}
	left, blocked := t.guesses.blocked(c.IP())
	if blocked {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(left.Seconds())+1))
		_ = authError(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "too many failed attempts; retry later")
	}
	return blocked
}

func (t Tokens) failed(c *fiber.Ctx) {
	if t.guesses != nil {
		t.guesses.fail(c.IP())
	}
}

func (t Tokens) Authenticate() fiber.Handler {
	admin := Principal{Subject: AdminSubject, Permissions: LocalPermissions()}
	return func(c *fiber.Ctx) error {
		normalizeAPIKeyHeader(c)
		if c.Get("Authorization") != "" {
			got := bearer(c)
			if strings.HasPrefix(got, LocalKeyPrefix) && t.Keys != nil {
				// 256-bit keys: unguessable, so not throttled.
				if p, err := t.Keys.ResolveKey(c.UserContext(), got); err == nil {
					SetPrincipal(c, p)
					return c.Next()
				}
				return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			}
			if got == "" {
				return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
			}
			if t.throttled(c) {
				return nil
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(t.AdminToken)) == 1 {
				SetPrincipal(c, admin)
				return c.Next()
			}
			t.failed(c)
			return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
		}
		if _, ok := t.session(c); ok {
			if unsafeMethod(c.Method()) && !sameOrigin(c) {
				return authError(c, fiber.StatusForbidden, "FORBIDDEN", "cross-origin request rejected")
			}
			SetPrincipal(c, admin)
			return c.Next()
		}
		return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
	}
}

func (t Tokens) Session() SessionRoutes {
	return SessionRoutes{Current: t.current, Login: t.login, Logout: t.logout}
}

// current never answers 401, so the console can ask before showing the form.
func (t Tokens) current(c *fiber.Ctx) error {
	if exp, ok := t.session(c); ok {
		return c.JSON(SessionState{Authenticated: true, ExpiresAt: &exp, Subject: AdminSubject, Permissions: LocalPermissions()})
	}
	return c.JSON(SessionState{Permissions: []string{}})
}

// login is POST /api/v1/session {"token": admin token}: sets the cookie.
func (t Tokens) login(c *fiber.Ctx) error {
	if !sameOrigin(c) {
		return authError(c, fiber.StatusForbidden, "FORBIDDEN", "cross-origin request rejected")
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return authError(c, fiber.StatusBadRequest, "VALIDATION", "invalid request body")
	}
	if t.throttled(c) {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Token)), []byte(t.AdminToken)) != 1 {
		t.failed(c)
		return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	exp := time.Now().Add(SessionTTL).Truncate(time.Second)
	c.Cookie(t.cookie(c, strconv.FormatInt(exp.Unix(), 10)+"."+t.sign(exp.Unix()), int(SessionTTL.Seconds())))
	exp = exp.UTC()
	return c.JSON(SessionState{Authenticated: true, ExpiresAt: &exp, Subject: AdminSubject, Permissions: LocalPermissions()})
}

// logout is DELETE /api/v1/session: clears this browser's cookie (rotate
// the admin token to end every session).
func (t Tokens) logout(c *fiber.Ctx) error {
	c.Cookie(t.cookie(c, "", -1))
	return c.SendStatus(fiber.StatusNoContent)
}

// The cookie is stateless, "<expiry>.<hmac>" keyed by the admin token, so
// rotating the token ends every session.
func (t Tokens) sign(expiry int64) string {
	m := hmac.New(sha256.New, []byte(t.AdminToken))
	m.Write([]byte("freerouter-session:" + strconv.FormatInt(expiry, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

func (t Tokens) session(c *fiber.Ctx) (time.Time, bool) {
	value := c.Cookies(SessionCookie)
	if value == "" || t.AdminToken == "" {
		return time.Time{}, false
	}
	rawExp, mac, ok := strings.Cut(value, ".")
	exp, err := strconv.ParseInt(rawExp, 10, 64)
	if !ok || err != nil || time.Now().Unix() >= exp {
		return time.Time{}, false
	}
	if !hmac.Equal([]byte(mac), []byte(t.sign(exp))) {
		return time.Time{}, false
	}
	return time.Unix(exp, 0).UTC(), true
}

func (t Tokens) cookie(c *fiber.Ctx, value string, maxAge int) *fiber.Cookie {
	cookie := &fiber.Cookie{Name: SessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HTTPOnly: true, Secure: overTLS(c), SameSite: fiber.CookieSameSiteStrictMode}
	if maxAge < 0 {
		// fasthttp drops a negative Max-Age; an Expires in the past deletes.
		cookie.MaxAge, cookie.Expires = 0, time.Unix(1, 0)
	}
	return cookie
}

func unsafeMethod(m string) bool {
	return m != fiber.MethodGet && m != fiber.MethodHead && m != fiber.MethodOptions
}

// overTLS: the browser reached us over HTTPS, directly, through a proxy
// that says so (X-Forwarded-Proto), or through one that doesn't — then the
// page's own Origin (sent on the login POST) still tells. The session
// cookie is Secure then, so it never travels over plain HTTP.
func overTLS(c *fiber.Ctx) bool {
	return c.Protocol() == "https" || strings.HasPrefix(strings.ToLower(c.Get("Origin")), "https://")
}

// sameOrigin rejects what a browser sends from another origin (CSRF on the
// cookie). Requests without browser headers (curl, SDKs) pass.
func sameOrigin(c *fiber.Ctx) bool {
	switch c.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "":
	default:
		return false
	}
	origin := c.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, string(c.Request().Host()))
}
