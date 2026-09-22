package webhooksvc

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

// ── publicAddress ───────────────────────────────────────────────────

func TestPublicAddress_RejectsPrivateAndSpecialRanges(t *testing.T) {
	rejected := []string{
		"127.0.0.1",    // loopback
		"10.0.0.1",     // RFC1918 private
		"172.16.0.1",   // RFC1918 private
		"192.168.1.1",  // RFC1918 private
		"169.254.1.1",  // link-local
		"0.0.0.0",      // this-network
		"100.64.0.1",   // shared address space (CGNAT)
		"192.0.0.1",    // IETF protocol assignments
		"192.0.2.1",    // TEST-NET-1 documentation
		"198.18.0.1",   // benchmarking
		"198.51.100.1", // TEST-NET-2 documentation
		"203.0.113.1",  // TEST-NET-3 documentation
		"240.0.0.1",    // reserved
		"::1",          // IPv6 loopback
		"fe80::1",      // IPv6 link-local
		"fc00::1",      // IPv6 unique local (private)
		"64:ff9b::1",   // IPv4/IPv6 translation
	}
	for _, raw := range rejected {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("failed to parse test address %q: %v", raw, err)
		}
		if publicAddress(addr) {
			t.Errorf("expected %q to be rejected as non-public", raw)
		}
	}
}

func TestPublicAddress_AllowsGlobalUnicast(t *testing.T) {
	allowed := []string{
		"8.8.8.8",              // Google public DNS
		"1.1.1.1",              // Cloudflare public DNS
		"93.184.216.34",        // example.com (public)
		"2606:4700:4700::1111", // Cloudflare IPv6 DNS
	}
	for _, raw := range allowed {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("failed to parse test address %q: %v", raw, err)
		}
		if !publicAddress(addr) {
			t.Errorf("expected %q to be allowed as public", raw)
		}
	}
}

func TestPublicAddress_RejectsMappedPrivateIPv4(t *testing.T) {
	// ::ffff:127.0.0.1 is an IPv4-mapped IPv6 address wrapping a loopback IP;
	// Unmap() must strip the mapping before the private/loopback check.
	addr, err := netip.ParseAddr("::ffff:127.0.0.1")
	if err != nil {
		t.Fatalf("failed to parse mapped address: %v", err)
	}
	if publicAddress(addr) {
		t.Fatalf("expected mapped loopback address to be rejected")
	}
}

// ── publicWebhookClient (SSRF guard at dial time) ──────────────────

func TestPublicWebhookClient_RejectsLoopbackTarget(t *testing.T) {
	// Start a real local server; its address resolves to 127.0.0.1, which
	// must be rejected by the DialContext guard regardless of hostname.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := publicWebhookClient(2 * time.Second)
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatalf("expected request to loopback target to be rejected")
	}
}

func TestPublicWebhookClient_NoAutoRedirectFollow(t *testing.T) {
	client := publicWebhookClient(2 * time.Second)
	if client.CheckRedirect == nil {
		t.Fatalf("expected CheckRedirect to be set to prevent SSRF via redirect")
	}
	if err := client.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("expected CheckRedirect to return http.ErrUseLastResponse, got %v", err)
	}
}

func TestPublicWebhookClient_DialContextRejectsUnresolvableHost(t *testing.T) {
	client := publicWebhookClient(2 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport")
	}

	_, err := transport.DialContext(context.Background(), "tcp", "this-host-does-not-exist.invalid:80")
	if err == nil {
		t.Fatalf("expected DNS resolution failure for invalid host")
	}
}

func TestPublicWebhookClient_DialContextRejectsPrivateIPLiteral(t *testing.T) {
	client := publicWebhookClient(2 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport")
	}

	_, err := transport.DialContext(context.Background(), "tcp", net.JoinHostPort("10.0.0.5", "80"))
	if err == nil {
		t.Fatalf("expected private IP literal to be rejected")
	}
}

// ── sign / generateSecret ───────────────────────────────────────────

func TestSign_IsDeterministicAndPrefixed(t *testing.T) {
	sig1 := sign("payload", "secret")
	sig2 := sign("payload", "secret")
	if sig1 != sig2 {
		t.Fatalf("expected deterministic signature, got %q and %q", sig1, sig2)
	}
	if len(sig1) < 7 || sig1[:7] != "sha256=" {
		t.Fatalf("expected sha256= prefix, got %q", sig1)
	}
}

func TestSign_DifferentSecretsProduceDifferentSignatures(t *testing.T) {
	sig1 := sign("payload", "secret-a")
	sig2 := sign("payload", "secret-b")
	if sig1 == sig2 {
		t.Fatalf("expected different signatures for different secrets")
	}
}

func TestSign_DifferentPayloadsProduceDifferentSignatures(t *testing.T) {
	sig1 := sign("payload-a", "secret")
	sig2 := sign("payload-b", "secret")
	if sig1 == sig2 {
		t.Fatalf("expected different signatures for different payloads")
	}
}

func TestGenerateSecret_HasExpectedPrefixAndUniqueness(t *testing.T) {
	s1 := generateSecret()
	s2 := generateSecret()

	if len(s1) < 6 || s1[:6] != "whsec_" {
		t.Fatalf("expected whsec_ prefix, got %q", s1)
	}
	if s1 == s2 {
		t.Fatalf("expected unique secrets across calls")
	}
}
