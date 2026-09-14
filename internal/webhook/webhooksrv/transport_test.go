package webhooksrv

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
)

func TestWebhookRejectsNonPublicDestinations(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "::1", "::ffff:127.0.0.1", "fe80::1", "fc00::1", "0.0.0.0", "2002:7f00:1::"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("unsafe destination allowed: %s", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("public address rejected: %s", raw)
		}
	}
	var reached atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Store(true) }))
	defer server.Close()
	client := publicWebhookClient()
	defer client.CloseIdleConnections()
	if response, err := client.Get(server.URL); err == nil {
		response.Body.Close()
		t.Fatal("loopback request accepted")
	}
	if reached.Load() {
		t.Fatal("blocked destination was contacted")
	}
	if err := client.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatal("redirects are enabled")
	}
}
