package e2e

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
)

func TestGatewayDenialStopsUpstream(t *testing.T) {
	s := NewSuite(t)
	var calls atomic.Int32
	s.MockUpstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(200) })
	ctx := context.Background()
	cfg := &gateway.RateLimitConfig{TenantID: s.TenantID, RPM: 1, MaxConcurrent: 10}
	if err := s.DB.QueryRow(`INSERT INTO rate_limit_configs (tenant_id,rpm,max_concurrent) VALUES ($1,1,10) RETURNING rpm`, s.TenantID).Scan(&cfg.RPM); err != nil {
		t.Fatal(err)
	}
	limiter := gateway.NewRateLimiter(s.Redis, *cfg, nil)
	if _, err := limiter.CheckRPM(ctx, s.TenantID.String()); err != nil {
		t.Fatal(err)
	}
	resp, _ := s.Do(s.Request("POST", "/v1/chat/completions", map[string]any{"model": "gpt-4o", "messages": []map[string]string{{"role": "user", "content": "hello"}}}))
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want 429 got %d", resp.StatusCode)
	}
	if calls.Load() != 0 {
		t.Fatal("denied request reached upstream")
	}
	// API-key authentication cannot widen its own model restriction.
	var id string
	if err := s.DB.Get(&id, `SELECT id FROM api_keys WHERE tenant_id=$1 LIMIT 1`, s.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE api_keys SET scopes=ARRAY['api_keys:write'], allowed_models=ARRAY['gpt-4o'] WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	resp, _ = s.Do(s.requestWith(s.APIKey, "PUT", "/api/v1/api-keys/"+id, map[string]any{"allowed_models": []string{}}))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("key management wanted 403 got %d", resp.StatusCode)
	}
}
