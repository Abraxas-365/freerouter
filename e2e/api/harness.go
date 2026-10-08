//go:build e2e

// Package api holds black-box functional tests that run against a live e2e
// stack (e2e/up.sh). They are excluded from `go test ./...` by the e2e build
// tag; run them with:
//
//	go test -tags e2e -count=1 ./e2e/api/... -run TestWS01
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fixtures mirrors e2e/.run/fixtures.json.
type Fixtures struct {
	URLs struct {
		Server, API, Gateway, IAMKit, Web, FakeLLM string
		WebhookSink                                string `json:"webhook_sink"`
		DB, Redis                                  string
	} `json:"urls"`
	IAMKit struct {
		EnvironmentID  string `json:"environment_id"`
		OrganizationID string `json:"organization_id"`
		ApplicationID  string `json:"application_id"`
		ResourceID     string `json:"resource_id"`
		Audience       string `json:"audience"`
		ManagementKey  string `json:"management_key"`
	} `json:"iamkit"`
	Personas     map[string]Persona `json:"personas"`
	Roles        map[string]string  `json:"roles"`
	Boundary     map[string]string  `json:"boundary"`
	Providers    map[string]string  `json:"providers"`
	ProviderKeys map[string]string  `json:"provider_keys"`
	Models       map[string]string  `json:"models"`
	Permissions  []string           `json:"permissions"`
}

type Persona struct {
	Kind     string `json:"kind"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Secret   string `json:"secret"`
	UserID   string `json:"user_id"`
	ID       string `json:"id"`
	Note     string `json:"note"`
}

var (
	fxOnce sync.Once
	fx     Fixtures
	fxErr  error
)

// FX loads fixtures once; skips the test when the stack is not running.
func FX(t *testing.T) Fixtures {
	t.Helper()
	fxOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		path := filepath.Join(filepath.Dir(file), "..", ".run", "fixtures.json")
		if p := os.Getenv("E2E_FIXTURES"); p != "" {
			path = p
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fxErr = err
			return
		}
		fxErr = json.Unmarshal(b, &fx)
	})
	if fxErr != nil {
		t.Skipf("e2e stack not available (%v); run e2e/up.sh", fxErr)
	}
	return fx
}

// Resp is a decoded HTTP response.
type Resp struct {
	Status  int
	Header  http.Header
	Body    []byte
	Elapsed time.Duration
}

// JSON decodes the body into v (fails the test on malformed JSON).
func (r Resp) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %d response: %v\n%s", r.Status, err, r.Body)
	}
}

// Map decodes the body as a generic object.
func (r Resp) Map(t *testing.T) map[string]any {
	t.Helper()
	m := map[string]any{}
	r.JSON(t, &m)
	return m
}

// Client is a minimal HTTP client bound to a base URL and credential.
type Client struct {
	Base    string
	Headers map[string]string
	HTTP    *http.Client
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// Bearer builds a client that sends Authorization: Bearer <token>.
func Bearer(base, token string) *Client {
	return &Client{Base: base, Headers: map[string]string{"Authorization": "Bearer " + token}, HTTP: httpClient}
}

// Anon builds a client with no credentials.
func Anon(base string) *Client {
	return &Client{Base: base, Headers: map[string]string{}, HTTP: httpClient}
}

// AdminAPI returns a /api/v1 client using the all-permissions admin key.
func AdminAPI(t *testing.T) *Client {
	f := FX(t)
	return Bearer(f.URLs.API, f.Personas["admin_key"].Secret)
}

// Do performs a request; body may be nil, []byte, string, io.Reader or any JSON-marshalable value.
func (c *Client) Do(t *testing.T, method, path string, body any) Resp {
	t.Helper()
	r, err := c.DoCtx(context.Background(), method, path, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return r
}

// DoCtx is Do without the test dependency (for goroutines / timing tests).
func (c *Client) DoCtx(ctx context.Context, method, path string, body any) (Resp, error) {
	var rd io.Reader
	ct := ""
	switch b := body.(type) {
	case nil:
	case []byte:
		rd, ct = bytes.NewReader(b), "application/json"
	case string:
		rd, ct = strings.NewReader(b), "application/json"
	case io.Reader:
		rd = b
	default:
		j, err := json.Marshal(b)
		if err != nil {
			return Resp{}, err
		}
		rd, ct = bytes.NewReader(j), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return Resp{}, err
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	res, err := c.HTTP.Do(req)
	if err != nil {
		return Resp{}, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return Resp{Status: res.StatusCode, Header: res.Header, Body: b, Elapsed: time.Since(start)}, err
	}
	return Resp{Status: res.StatusCode, Header: res.Header, Body: b, Elapsed: time.Since(start)}, nil
}

// Get/Post/Put/Patch/Delete shorthands.
func (c *Client) Get(t *testing.T, path string) Resp { return c.Do(t, http.MethodGet, path, nil) }
func (c *Client) Post(t *testing.T, path string, body any) Resp {
	return c.Do(t, http.MethodPost, path, body)
}
func (c *Client) Put(t *testing.T, path string, body any) Resp {
	return c.Do(t, http.MethodPut, path, body)
}
func (c *Client) Patch(t *testing.T, path string, body any) Resp {
	return c.Do(t, http.MethodPatch, path, body)
}
func (c *Client) Delete(t *testing.T, path string) Resp { return c.Do(t, http.MethodDelete, path, nil) }

// Stream opens a request and returns the raw response for SSE reading. Caller closes Body.
func (c *Client) Stream(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	j, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, c.Base+path, bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	res, err := (&http.Client{Timeout: 0}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return res
}

// Login obtains a user JWT from IAMKit's /identity/v1/login for a user persona.
func Login(t *testing.T, p Persona) (access, refresh string) {
	t.Helper()
	f := FX(t)
	r := Anon(f.URLs.IAMKit).Post(t, "/identity/v1/login", map[string]string{
		"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
		"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
		"email": p.Email, "password": p.Password,
	})
	if r.Status != 200 {
		t.Fatalf("login %s: %d %s", p.Email, r.Status, r.Body)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	r.JSON(t, &out)
	return out.AccessToken, out.RefreshToken
}

// AsUser returns a /api/v1 client authenticated as the given user persona.
func AsUser(t *testing.T, name string) *Client {
	t.Helper()
	f := FX(t)
	tok, _ := Login(t, f.Personas[name])
	return Bearer(f.URLs.API, tok)
}

// Management returns a client for IAMKit's /management/v1 (owner key) for boundary setups.
func Management(t *testing.T) *Client {
	f := FX(t)
	return &Client{Base: f.URLs.IAMKit + "/management/v1", Headers: map[string]string{"X-API-Key": f.IAMKit.ManagementKey}, HTTP: httpClient}
}

// Uniq returns a collision-free name for created objects.
func Uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// Eventually polls fn until it returns true or the timeout expires.
func Eventually(t *testing.T, timeout time.Duration, fn func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out after %s: %s", timeout, msg)
}

// FakeLLMRequests returns everything the fake upstream recorded since the last reset.
func FakeLLMRequests(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &out)
	return out
}

// ResetFakeLLM clears recorded upstream requests and flaky counters.
func ResetFakeLLM(t *testing.T) { Anon(FX(t).URLs.FakeLLM).Post(t, "/_reset", nil) }

// SinkDeliveries returns webhook deliveries recorded by the sink.
func SinkDeliveries(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	Anon(FX(t).URLs.WebhookSink).Get(t, "/_deliveries").JSON(t, &out)
	return out
}

// ResetSink clears recorded webhook deliveries and failure counters.
func ResetSink(t *testing.T) { Anon(FX(t).URLs.WebhookSink).Delete(t, "/_deliveries") }

// Chat builds a minimal OpenAI chat request body.
func Chat(model, content string, stream bool) map[string]any {
	return map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": content}}, "stream": stream}
}
