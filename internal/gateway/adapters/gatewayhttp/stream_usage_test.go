package gatewayhttp

import (
	"strings"
	"testing"
)

func TestWithModel(t *testing.T) {
	got := string(withModel([]byte(`data: {"id":"c","model":"fake-ok","choices":[{"index":0,"delta":{"content":"hi"}}]}`+"\n\n"), "e2e-ok"))
	if !strings.Contains(got, `"model":"e2e-ok"`) || strings.Contains(got, "fake-ok") || !strings.Contains(got, `"content":"hi"`) || !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("model not rewritten: %q", got)
	}
	for _, in := range []string{"data: [DONE]\n\n", `data: {"choices":[]}` + "\n\n", "data: not-json\n\n"} {
		if got := string(withModel([]byte(in), "e2e-ok")); got != in {
			t.Errorf("%q changed to %q", in, got)
		}
	}
}

func TestStreamUsage(t *testing.T) {
	t.Run("openai final usage chunk", func(t *testing.T) {
		u := &streamUsage{}
		u.observe([]byte(`data: {"choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n"))
		u.observe([]byte(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		u.observe([]byte(`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9}}` + "\n\n"))
		u.observe([]byte("data: [DONE]\n\n"))
		r := u.response()
		if r == nil || r.Usage == nil || r.Usage.PromptTokens != 5 || r.Usage.CompletionTokens != 4 || r.Usage.TotalTokens != 9 {
			t.Fatalf("usage = %+v", r)
		}
		if r.Choices[0].FinishReason == nil || *r.Choices[0].FinishReason != "stop" {
			t.Fatalf("finish reason = %v", r.Choices[0].FinishReason)
		}
	})

	t.Run("anthropic split input and output", func(t *testing.T) {
		u := &streamUsage{}
		u.observe([]byte(`data: {"choices":[{"index":0,"delta":{"role":"assistant"}}],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":0,"cache_read_input_tokens":2}}` + "\n\n"))
		u.observe([]byte(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":4,"total_tokens":0}}` + "\n\n"))
		r := u.response()
		if r == nil || r.Usage == nil || r.Usage.PromptTokens != 5 || r.Usage.CompletionTokens != 4 || r.Usage.TotalTokens != 9 || r.Usage.CacheReadInputTokens != 2 {
			t.Fatalf("usage = %+v", r.Usage)
		}
	})

	t.Run("no usage", func(t *testing.T) {
		u := &streamUsage{}
		u.observe([]byte(`data: {"choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n"))
		if r := u.response(); r != nil {
			t.Fatalf("want nil, got %+v", r)
		}
	})
}
