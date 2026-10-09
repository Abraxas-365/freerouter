//go:build e2e

package api

// WS-12 Part A — server startup fail-fast.
//
// Each case execs a freshly built ./cmd/server on a free port with the e2e
// stack's environment (e2e/.run/.env) plus per-case overrides, and asserts
// within ~10s whether it exits non-zero with a clear message or comes up.
// The live server on :23000 is never touched; servers started here share the
// e2e Postgres/Redis/IAMKit and are killed in t.Cleanup.

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const ws12StartupWindow = 10 * time.Second

var (
	ws12BinOnce sync.Once
	ws12Bin     string
	ws12BinErr  error
)

func ws12Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// ws12Binary builds ./cmd/server once per test run.
func ws12Binary(t *testing.T) string {
	t.Helper()
	ws12BinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ws12-bin-")
		if err != nil {
			ws12BinErr = err
			return
		}
		ws12Bin = filepath.Join(dir, "server")
		cmd := exec.Command("go", "build", "-o", ws12Bin, "./cmd/server")
		cmd.Dir = ws12Root()
		if out, err := cmd.CombinedOutput(); err != nil {
			ws12BinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if ws12BinErr != nil {
		t.Fatal(ws12BinErr)
	}
	return ws12Bin
}

// ws12BaseEnv parses e2e/.run/.env (KEY=VALUE lines, no quoting).
func ws12BaseEnv(t *testing.T) map[string]string {
	t.Helper()
	FX(t)
	b, err := os.ReadFile(filepath.Join(ws12Root(), "e2e", ".run", ".env"))
	if err != nil {
		t.Skipf("e2e/.run/.env not available: %v", err)
	}
	env := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

func ws12FreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
}

// unset marks an override that removes the variable entirely.
const unset = "\x00unset"

type ws12Proc struct {
	URL      string
	Env      map[string]string
	Exited   bool
	ExitCode int
	Elapsed  time.Duration // time to exit or to first healthy /health
	cmd      *exec.Cmd
	out      *ws12Buf
	done     chan struct{}
}

type ws12Buf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *ws12Buf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *ws12Buf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func (p *ws12Proc) Log() string { return p.out.String() }

// ws12Start runs the server with base env + overrides and waits up to the
// startup window for it to exit or answer /health. Hung processes are
// reported as neither Exited nor up (URL == "").
func ws12Start(t *testing.T, overrides map[string]string) *ws12Proc {
	t.Helper()
	bin := ws12Binary(t)
	env := ws12BaseEnv(t)
	port := ws12FreePort(t)
	env["SERVER_PORT"] = port
	for k, v := range overrides {
		if v == unset {
			delete(env, k)
		} else {
			env[k] = v
		}
	}
	cmd := exec.Command(bin)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	p := &ws12Proc{Env: env, cmd: cmd, out: &ws12Buf{}, done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = p.out, p.out
	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-p.done
			}
		}
	})

	base := "http://127.0.0.1:" + port
	hc := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := start.Add(ws12StartupWindow)
	for time.Now().Before(deadline) {
		select {
		case <-p.done:
			p.Exited, p.ExitCode, p.Elapsed = true, cmd.ProcessState.ExitCode(), time.Since(start)
			return p
		default:
		}
		if r, err := hc.Get(base + "/health"); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				p.URL, p.Elapsed = base, time.Since(start)
				return p
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	p.Elapsed = time.Since(start)
	return p
}

// ws12NoSecretsLogged fails if any credential from env shows up in the log.
func ws12NoSecretsLogged(t *testing.T, p *ws12Proc) {
	t.Helper()
	log := p.Log()
	for _, k := range []string{"ENCRYPTION_KEY", "IAMKIT_SERVICE_SECRET", "OIDC_HMAC_SECRET", "IAMKIT_ENCRYPTION_KEY", "FREEROUTER_ADMIN_PASSWORD"} {
		if v := p.Env[k]; len(v) >= 8 && strings.Contains(log, v) {
			t.Errorf("FINDING: %s value appears in server output", k)
		}
	}
	if v := p.Env["DB_PASSWORD"]; len(v) >= 12 && strings.Contains(log, v) {
		t.Errorf("FINDING: DB_PASSWORD value appears in server output")
	}
}

// ws12ExpectFail asserts a non-zero exit inside the window with msg in the output.
func ws12ExpectFail(t *testing.T, p *ws12Proc, msg string) {
	t.Helper()
	switch {
	case p.URL != "":
		t.Fatalf("server booted (in %s) but should have failed with %q\n%s", p.Elapsed, msg, p.Log())
	case !p.Exited:
		t.Fatalf("server neither exited nor came up within %s (hang)\n%s", ws12StartupWindow, p.Log())
	case p.ExitCode == 0:
		t.Fatalf("server exited 0, want non-zero\n%s", p.Log())
	case !strings.Contains(p.Log(), msg):
		t.Fatalf("exit %d but output lacks %q:\n%s", p.ExitCode, msg, p.Log())
	}
	t.Logf("exit %d after %s: %s", p.ExitCode, p.Elapsed.Round(time.Millisecond), strings.TrimSpace(p.Log()))
	ws12NoSecretsLogged(t, p)
}

func ws12ExpectUp(t *testing.T, p *ws12Proc) {
	t.Helper()
	if p.URL == "" {
		t.Fatalf("server did not come up within %s (exited=%v code=%d)\n%s", ws12StartupWindow, p.Exited, p.ExitCode, p.Log())
	}
	t.Logf("up after %s", p.Elapsed.Round(time.Millisecond))
}

// (1) Each required IAMKit boundary missing → exit 1 naming the variable.
func TestWS12_Startup_MissingIAMKitVar(t *testing.T) {
	cases := map[string]string{
		"IAMKIT_AUDIENCE":        "missing IAMKit configuration: IAMKIT_AUDIENCE (run make bootstrap)",
		"IAMKIT_ENVIRONMENT_ID":  "missing IAMKit configuration: IAMKIT_ENVIRONMENT_ID (run make bootstrap)",
		"IAMKIT_APPLICATION_ID":  "missing IAMKit configuration: IAMKIT_APPLICATION_ID (run make bootstrap)",
		"IAMKIT_RESOURCE_ID":     "missing IAMKit configuration: IAMKIT_RESOURCE_ID (run make bootstrap)",
		"IAMKIT_ORGANIZATION_ID": "IAMKIT_ORGANIZATION_ID is required when IAMKIT_SERVICE_SECRET is set",
	}
	for v, msg := range cases {
		t.Run(v, func(t *testing.T) {
			ws12ExpectFail(t, ws12Start(t, map[string]string{v: unset}), msg)
		})
	}
	t.Run("whitespace-only IAMKIT_JWT_ISSUER", func(t *testing.T) {
		ws12ExpectFail(t, ws12Start(t, map[string]string{"IAMKIT_JWT_ISSUER": "  "}), "missing IAMKit configuration: IAMKIT_JWT_ISSUER")
	})
	t.Run("several missing are listed together", func(t *testing.T) {
		ws12ExpectFail(t, ws12Start(t, map[string]string{"IAMKIT_AUDIENCE": unset, "IAMKIT_RESOURCE_ID": unset}),
			"missing IAMKit configuration: IAMKIT_RESOURCE_ID, IAMKIT_AUDIENCE")
	})
}

// FINDING WS12-1: IAMKIT_BASE_URL and IAMKIT_JWT_ISSUER are "required" in
// config.Validate but config.Load defaults them to http://localhost:8080, so
// omitting them boots silently against the wrong IAMKit (every request 401).
func TestWS12_Startup_MissingIAMKitURLsFailFast(t *testing.T) {
	for _, v := range []string{"IAMKIT_BASE_URL", "IAMKIT_JWT_ISSUER"} {
		t.Run(v, func(t *testing.T) {
			p := ws12Start(t, map[string]string{v: unset})
			if p.URL != "" {
				gw := Bearer(p.URL, FX(t).Personas["admin_key"].Secret).Get(t, "/api/v1/providers?limit=1")
				t.Fatalf("FINDING WS12-1: server booted without %s (defaulted to http://localhost:8080); admin call → %d %s", v, gw.Status, gw.Body)
			}
			ws12ExpectFail(t, p, v)
		})
	}
}

// (2) Management key as backend credential → rejected at startup.
func TestWS12_Startup_ManagementKeyRejected(t *testing.T) {
	p := ws12Start(t, map[string]string{"IAMKIT_SERVICE_SECRET": "ik_mgmt_ws12notarealkey0123456789"})
	ws12ExpectFail(t, p, "IAMKIT_SERVICE_SECRET must be an IAMKit service account credential (ik_svc_...); management keys (ik_mgmt_) are not accepted")
	if strings.Contains(p.Log(), "ik_mgmt_ws12notarealkey") {
		t.Errorf("FINDING: the rejected management key is echoed in the startup error")
	}
}

// (3) No backend service secret → boots; /access and /service-accounts absent.
func TestWS12_Startup_NoServiceSecret(t *testing.T) {
	p := ws12Start(t, map[string]string{"IAMKIT_SERVICE_SECRET": unset})
	ws12ExpectUp(t, p)
	admin := Bearer(p.URL+"/api/v1", FX(t).Personas["admin_key"].Secret)
	for _, path := range []string{"/access/users", "/access/roles", "/service-accounts", "/service-accounts/x"} {
		r := admin.Get(t, path)
		if r.Status != http.StatusNotFound {
			t.Errorf("GET %s without service secret: %d %s, want 404", path, r.Status, r.Body)
		}
	}
	if r := admin.Post(t, "/service-accounts", map[string]any{"name": "ws12-x"}); r.Status != http.StatusNotFound {
		t.Errorf("POST /service-accounts without service secret: %d, want 404", r.Status)
	}
	if r := admin.Get(t, "/providers?limit=1"); r.Status != 200 {
		t.Errorf("GET /providers without service secret: %d %s, want 200", r.Status, r.Body)
	}
	// Auth itself does not need the backend credential.
	if r := Bearer(p.URL+"/v1", FX(t).Personas["gw_key"].Secret).Get(t, "/models"); r.Status != 200 {
		t.Errorf("GET /v1/models without service secret: %d %s", r.Status, r.Body)
	}
	if strings.Contains(strings.ToLower(p.Log()), "service_secret") || strings.Contains(strings.ToLower(p.Log()), "service account") {
		t.Logf("startup log mentions the disabled routes")
	} else {
		t.Logf("note: nothing in the startup log says /access and /service-accounts are disabled")
	}
	ws12NoSecretsLogged(t, p)
}

// (4) Bad ENCRYPTION_KEY → exit 1 with a readable reason, key not echoed.
func TestWS12_Startup_BadEncryptionKey(t *testing.T) {
	cases := []struct{ name, key, msg string }{
		{"empty", unset, "encryption key must be 32 bytes, got 0"},
		{"short hex", "abcd", "encryption key must be 32 bytes, got 2"},
		{"33 bytes", strings.Repeat("ab", 33), "encryption key must be 32 bytes, got 33"},
		{"non-hex 64 chars", strings.Repeat("zq", 32), "invalid encryption key: encoding/hex: invalid byte"},
		{"base64 32 bytes", "q83vEjRWeJCrze8SNFZ4kKvN7xI0VniQq83vEjRWeJA=", "invalid encryption key"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ws12Start(t, map[string]string{"ENCRYPTION_KEY": c.key})
			ws12ExpectFail(t, p, c.msg)
			if c.key != unset && len(c.key) >= 8 && strings.Contains(p.Log(), c.key) {
				t.Errorf("FINDING: bad ENCRYPTION_KEY echoed in startup error")
			}
		})
	}
}

// (5) Postgres unreachable / wrong credentials → exit 1 fast, password not logged.
func TestWS12_Startup_DatabaseUnavailable(t *testing.T) {
	t.Run("closed port", func(t *testing.T) {
		ws12ExpectFail(t, ws12Start(t, map[string]string{"DB_PORT": "1"}), "infrastructure: postgres: dial tcp")
	})
	t.Run("wrong password", func(t *testing.T) {
		pw := "ws12-wrong-password-SENTINEL"
		p := ws12Start(t, map[string]string{"DB_PASSWORD": pw})
		ws12ExpectFail(t, p, "password authentication failed")
		if strings.Contains(p.Log(), pw) {
			t.Errorf("FINDING: DB password echoed in startup error")
		}
	})
	t.Run("unknown database", func(t *testing.T) {
		ws12ExpectFail(t, ws12Start(t, map[string]string{"DB_NAME": "ws12_nope"}), "ws12_nope")
	})
}

// FINDING WS12-2: an unroutable DB host (packets dropped) blocks startup on
// the OS connect timeout (~75s on macOS) — no connect_timeout in the DSN.
func TestWS12_Startup_UnroutableDBFailsFast(t *testing.T) {
	p := ws12Start(t, map[string]string{"DB_HOST": "10.255.255.1"})
	if !p.Exited && p.URL == "" {
		t.Fatalf("FINDING WS12-2: server with DB_HOST=10.255.255.1 neither failed nor started within %s (blocks on TCP connect timeout)\n%s", ws12StartupWindow, p.Log())
	}
	ws12ExpectFail(t, p, "postgres")
}

// (6) Redis unreachable → no fail-fast: boots, gateway works, rate limiting
// fails open and the response cache is bypassed. Latency is asserted in the
// outage test (FINDING WS12-3); here we document the degraded mode.
func TestWS12_Startup_RedisUnavailableDegrades(t *testing.T) {
	p := ws12Start(t, map[string]string{"REDIS_ADDR": "127.0.0.1:1"})
	ws12ExpectUp(t, p)
	f := FX(t)
	gw := Bearer(p.URL+"/v1", f.Personas["gw_key"].Secret)
	r := gw.Post(t, "/chat/completions", Chat("e2e-ok", Uniq("ws12 redis-down"), false))
	if r.Status != 200 {
		t.Fatalf("gateway with Redis down: %d %s", r.Status, r.Body)
	}
	t.Logf("gateway with Redis down: 200 in %s, X-RateLimit-Limit=%q (rate limit fails open)", r.Elapsed.Round(time.Millisecond), r.Header.Get("X-RateLimit-Limit"))
	if r.Header.Get("X-RateLimit-Limit") != "" {
		t.Errorf("rate-limit headers present although Redis is down: %v", r.Header)
	}
	if r.Elapsed > 10*time.Second {
		t.Errorf("gateway call took %s with Redis down (> 10s)", r.Elapsed)
	}
	admin := Bearer(p.URL+"/api/v1", f.Personas["admin_key"].Secret)
	c := admin.Delete(t, "/gateway/cache")
	t.Logf("DELETE /gateway/cache with Redis down: %d %s", c.Status, c.Body)
	if c.Status < 500 || !strings.HasPrefix(strings.TrimSpace(string(c.Body)), "{") {
		t.Errorf("cache invalidation with Redis down: %d %s, want 5xx JSON", c.Status, c.Body)
	}
	if !strings.Contains(p.Log(), "rate limit check failed") {
		t.Errorf("no 'rate limit check failed' log line; fail-open not visible to operators")
	}
	if strings.Contains(p.Log(), "redis") && strings.Contains(p.Log(), "FreeRouter starting") {
		lines := strings.SplitN(p.Log(), "FreeRouter starting", 2)
		if strings.Contains(lines[0], "redis") {
			t.Logf("startup warned about Redis")
		} else {
			t.Logf("note: startup did not warn that Redis is unreachable")
		}
	}
}

// (7) WEBHOOK_ALLOW_PRIVATE=true is ignored outside APP_ENV=test.
func TestWS12_Startup_AllowPrivateIgnoredInProduction(t *testing.T) {
	p := ws12Start(t, map[string]string{"APP_ENV": "production", "WEBHOOK_ALLOW_PRIVATE": "true"})
	ws12ExpectUp(t, p)
	if !strings.Contains(p.Log(), "env=production") {
		t.Errorf("startup log does not show env=production:\n%s", p.Log())
	}
	f := FX(t)
	admin := Bearer(p.URL+"/api/v1", f.Personas["admin_key"].Secret)
	path := "/hook/" + Uniq("ws12-x")
	cr := admin.Post(t, "/webhooks", map[string]any{"url": f.URLs.WebhookSink + path, "events": []string{"request.completed"}})
	if cr.Status != 200 && cr.Status != 201 {
		t.Fatalf("create webhook: %d %s", cr.Status, cr.Body)
	}
	var hook struct{ ID string }
	cr.JSON(t, &hook)
	// Delete promptly (cascades deliveries): the live test-mode server's retry
	// worker would otherwise pick up the pending delivery after 30s.
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/webhooks/"+hook.ID) })

	if r := Bearer(p.URL+"/v1", f.Personas["gw_key"].Secret).Post(t, "/chat/completions", Chat("e2e-ok", Uniq("ws12 ssrf"), false)); r.Status != 200 {
		t.Fatalf("gateway call: %d %s", r.Status, r.Body)
	}
	var last map[string]any
	Eventually(t, 10*time.Second, func() bool {
		var page struct{ Items []map[string]any }
		admin.Get(t, "/webhooks/"+hook.ID+"/deliveries").JSON(t, &page)
		if len(page.Items) > 0 && page.Items[0]["attempts"].(float64) >= 1 {
			last = page.Items[0]
			return true
		}
		return false
	}, "a delivery attempt to be recorded")
	if e, _ := last["last_error"].(string); !strings.Contains(e, "webhook destination is not public") || last["status"] == "success" {
		t.Errorf("delivery to loopback in production: status=%v last_error=%q, want SSRF rejection", last["status"], e)
	}
	for _, d := range SinkDeliveries(t) {
		if d["path"] == path {
			t.Errorf("sink received a delivery on %s although APP_ENV=production", path)
		}
	}
	AdminAPI(t).Delete(t, "/webhooks/"+hook.ID)
}
