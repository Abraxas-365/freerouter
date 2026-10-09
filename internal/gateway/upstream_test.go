package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

// sseServer replays the given SSE data payloads.
func sseServer(t *testing.T, events ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range events {
			_, _ = w.Write([]byte("data: " + e + "\n\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func collectStream(t *testing.T, route *RouteResult) []string {
	t.Helper()
	var got []string
	status, err := NewUpstream(time.Second).Stream(context.Background(), route, chatBody, func(c []byte) error {
		got = append(got, strings.TrimSuffix(strings.TrimPrefix(string(c), "data: "), "\n\n"))
		return nil
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("stream: status=%d err=%v", status, err)
	}
	return got
}

func TestUpstream_StreamKeepsGeminiFinalChunk(t *testing.T) {
	url := sseServer(t,
		`{"candidates":[{"content":{"parts":[{"text":"Hello "}]}}]}`,
		`{"candidates":[{"content":{"parts":[{"text":"world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":4,"totalTokenCount":9}}`,
	)
	got := collectStream(t, &RouteResult{Protocol: "google", ExternalID: "m", BaseURL: url, Token: "k"})
	if len(got) != 3 || got[2] != "[DONE]" {
		t.Fatalf("want 2 chunks + [DONE], got %q", got)
	}
	if !strings.Contains(got[1], `"world"`) || !strings.Contains(got[1], `"finish_reason":"stop"`) || !strings.Contains(got[1], `"prompt_tokens":5`) {
		t.Fatalf("final chunk lost text/finish_reason/usage: %s", got[1])
	}
}

func TestUpstream_StreamAlwaysEndsWithDone(t *testing.T) {
	url := sseServer(t,
		`{"type":"message-start","id":"x"}`,
		`{"type":"content-delta","delta":{"message":{"content":{"text":"hi"}}}}`,
		`{"type":"message-end","delta":{"finish_reason":"COMPLETE"}}`,
	)
	got := collectStream(t, &RouteResult{Protocol: "cohere", ExternalID: "m", BaseURL: url, Token: "k"})
	if len(got) == 0 || got[len(got)-1] != "[DONE]" {
		t.Fatalf("cohere stream must end with [DONE], got %q", got)
	}

	// An upstream that sends its own [DONE] must not produce two.
	url = sseServer(t, `{"choices":[{"delta":{"content":"hi"}}]}`, `[DONE]`)
	got = collectStream(t, openAIRoute(url))
	if len(got) != 2 || got[1] != "[DONE]" {
		t.Fatalf("openai stream: want chunk + single [DONE], got %q", got)
	}
}
