// Package iamx is FreeRouter's backend connection to IAMKit's
// permission-scoped API (/api/v1). It authenticates as a service account
// (ik_svc_) bound to the environment's built-in IAM resource, holding only
// the iam:* permissions FreeRouter needs — never a workspace-wide
// management key (ik_mgmt_).
package iamx

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/iamkit/sdk/apiclient"
	"github.com/Abraxas-365/iamkit/sdk/apierror"
	"github.com/Abraxas-365/iamkit/sdk/authclient"
)

// renewMargin renews the machine token this long before it expires, so a
// request never carries a token about to expire.
const renewMargin = time.Minute

// Client calls IAMKit's /api/v1 for one environment with a cached machine
// token exchanged from a service account credential.
type Client struct {
	environment string
	secret      string
	auth        *authclient.Client
	api         *apiclient.Client

	mu      sync.Mutex
	expires time.Time
}

// New returns a client for the environment authenticated by secret
// (an ik_svc_ credential). httpClient may be nil.
func New(baseURL, environment, secret string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		environment: environment,
		secret:      secret,
		auth:        authclient.New(baseURL, authclient.WithHTTPClient(httpClient)),
		api:         apiclient.New(baseURL, "", apiclient.WithHTTPClient(httpClient)),
	}
}

// Do runs call against the environment with a valid machine token. When
// IAMKit rejects the token (expired or rotated credential) it renews it
// once and retries. Errors are returned untranslated; see Translate.
func (c *Client) Do(ctx context.Context, call func(apiclient.Environment) error) error {
	if err := c.token(ctx, false); err != nil {
		return err
	}
	env := c.api.Environment(c.environment)
	err := call(env)
	if Status(err) == http.StatusUnauthorized {
		if err := c.token(ctx, true); err != nil {
			return err
		}
		err = call(env)
	}
	return err
}

func (c *Client) token(ctx context.Context, force bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && time.Now().Before(c.expires) {
		return nil
	}
	pair, err := c.auth.MachineToken(ctx, c.secret)
	if err != nil {
		return errx.Wrap(err, "IAMKit service credential rejected", errx.TypeExternal)
	}
	c.api.SetToken(pair.AccessToken)
	c.expires = time.Now().Add(time.Duration(pair.ExpiresIn)*time.Second - renewMargin)
	return nil
}

// Status returns the HTTP status of an IAMKit API error, or 0.
func Status(err error) int {
	var e *apierror.Error
	if errors.As(err, &e) {
		return e.HTTPStatus
	}
	return 0
}

// Translate keeps IAMKit decisions the caller can act on (validation,
// conflict, not found, business rule) and reports everything else as an
// upstream failure. op names the operation, e.g. "create user".
func Translate(err error, op string) error {
	if err == nil {
		return nil
	}
	var x *errx.Error
	if errors.As(err, &x) {
		return err
	}
	var e *apierror.Error
	if errors.As(err, &e) {
		switch e.HTTPStatus {
		case http.StatusBadRequest:
			return errx.Validation(e.Message).WithDetail("source", "iamkit")
		case http.StatusNotFound:
			return errx.NotFound(op + ": not found in IAMKit")
		case http.StatusConflict:
			return errx.Conflict(e.Message).WithDetail("source", "iamkit")
		case http.StatusUnprocessableEntity:
			return errx.Business(e.Message).WithDetail("source", "iamkit")
		}
	}
	return errx.Wrap(err, op+" in IAMKit", errx.TypeExternal)
}
