package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// closedPortURL returns a base URL nothing listens on.
func closedPortURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr + "/v1"
}

func openAIRoute(baseURL string) *RouteResult {
	return &RouteResult{Protocol: "openai", ExternalID: "m", BaseURL: baseURL, Token: "k"}
}

var chatBody = []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)

func TestUpstream_TransportErrorsAreRetryable(t *testing.T) {
	u := NewUpstream(time.Second)
	route := openAIRoute(closedPortURL(t))

	_, status, err := u.Call(context.Background(), route, chatBody)
	if err == nil || status != http.StatusBadGateway || !IsRetryable(status) {
		t.Fatalf("Call to closed port: status=%d err=%v, want retryable 502", status, err)
	}
	status, err = u.Stream(context.Background(), route, chatBody, func([]byte) error { return nil })
	if err == nil || status != http.StatusBadGateway {
		t.Fatalf("Stream to closed port: status=%d err=%v, want 502", status, err)
	}
	_, status, err = u.CallRaw(context.Background(), route, chatBody)
	if err == nil || status != http.StatusBadGateway {
		t.Fatalf("CallRaw to closed port: status=%d err=%v, want 502", status, err)
	}
}

func TestUpstream_HungUpstreamTimesOut(t *testing.T) {
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer hung.Close()
	defer close(release)

	u := NewUpstream(300 * time.Millisecond)
	start := time.Now()
	_, status, err := u.Call(context.Background(), openAIRoute(hung.URL+"/v1"), chatBody)
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("hung upstream took %s, want ~300ms", el)
	}
	if err == nil || status != http.StatusGatewayTimeout || !IsRetryable(status) {
		t.Fatalf("hung upstream: status=%d err=%v, want retryable 504", status, err)
	}
}

func TestUpstream_CancelledByCallerIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, status, err := NewUpstream(time.Second).Call(ctx, openAIRoute(closedPortURL(t)), chatBody)
	if err == nil || IsRetryable(status) {
		t.Fatalf("cancelled request: status=%d err=%v, want non-retryable error", status, err)
	}
}
