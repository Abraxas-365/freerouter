package gatewayhttp

import "testing"

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
