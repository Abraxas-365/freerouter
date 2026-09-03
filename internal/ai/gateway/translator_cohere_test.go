package gateway

import (
	"encoding/json"
	"testing"
)

func TestCohereTransformRequest(t *testing.T) {
	body := []byte(`{
		"model": "cohere-command",
		"messages": [
			{"role": "system", "content": "be brief"},
			{"role": "user", "content": "hi"}
		],
		"max_tokens": 100,
		"temperature": 0.5,
		"stream": true
	}`)

	tr := &CohereTranslator{}
	out, err := tr.TransformRequest(body, "command-a-03-2025")
	if err != nil {
		t.Fatal(err)
	}

	var req cohereRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "command-a-03-2025" {
		t.Errorf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("first role = %q", req.Messages[0].Role)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 100 {
		t.Error("max_tokens not carried over")
	}
	if !req.Stream {
		t.Error("stream not carried over")
	}
}

func TestCohereTransformResponse(t *testing.T) {
	body := []byte(`{
		"id": "resp-1",
		"finish_reason": "COMPLETE",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "hello there"}]
		},
		"usage": {"billed_units": {"input_tokens": 10, "output_tokens": 5}}
	}`)

	tr := &CohereTranslator{}
	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp-1" {
		t.Errorf("id = %q", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q", resp.Object)
	}
	if got := resp.Choices[0].Message.Content; got != "hello there" {
		t.Errorf("content = %v", got)
	}
	if fr := resp.Choices[0].FinishReason; fr == nil || *fr != "stop" {
		t.Errorf("finish_reason = %v", fr)
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestCohereTransformResponseToolCalls(t *testing.T) {
	body := []byte(`{
		"id": "resp-2",
		"finish_reason": "TOOL_CALL",
		"message": {
			"role": "assistant",
			"content": [],
			"tool_calls": [{"id": "tc1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"lima\"}"}}]
		}
	}`)

	tr := &CohereTranslator{}
	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	tcs := resp.Choices[0].Message.ToolCalls
	if len(tcs) != 1 || tcs[0].Function.Name != "get_weather" {
		t.Errorf("tool_calls = %+v", tcs)
	}
	if fr := resp.Choices[0].FinishReason; fr == nil || *fr != "tool_calls" {
		t.Errorf("finish_reason = %v", fr)
	}
}

func TestCohereStreamEvents(t *testing.T) {
	tr := &CohereTranslator{}

	// message-start → role delta
	out, done, err := tr.TransformStreamEvent([]byte(`{"id":"s1","type":"message-start","delta":{"message":{"role":"assistant"}}}`))
	if err != nil || done {
		t.Fatalf("err=%v done=%v", err, done)
	}
	var chunk ChatStreamChunk
	if err := json.Unmarshal(out, &chunk); err != nil {
		t.Fatal(err)
	}
	if chunk.Choices[0].Delta.Role != "assistant" {
		t.Errorf("role = %q", chunk.Choices[0].Delta.Role)
	}

	// content-delta → text
	out, done, err = tr.TransformStreamEvent([]byte(`{"type":"content-delta","index":0,"delta":{"message":{"content":{"type":"text","text":"hi"}}}}`))
	if err != nil || done {
		t.Fatalf("err=%v done=%v", err, done)
	}
	if err := json.Unmarshal(out, &chunk); err != nil {
		t.Fatal(err)
	}
	if chunk.Choices[0].Delta.Content != "hi" {
		t.Errorf("content = %v", chunk.Choices[0].Delta.Content)
	}

	// message-end → finish + usage
	out, done, err = tr.TransformStreamEvent([]byte(`{"type":"message-end","delta":{"finish_reason":"COMPLETE","usage":{"billed_units":{"input_tokens":7,"output_tokens":3}}}}`))
	if err != nil || done {
		t.Fatalf("err=%v done=%v", err, done)
	}
	if err := json.Unmarshal(out, &chunk); err != nil {
		t.Fatal(err)
	}
	if fr := chunk.Choices[0].FinishReason; fr == nil || *fr != "stop" {
		t.Errorf("finish = %v", fr)
	}
	if chunk.Usage == nil || chunk.Usage.PromptTokens != 7 || chunk.Usage.CompletionTokens != 3 {
		t.Errorf("usage = %+v", chunk.Usage)
	}

	// content-start produces no output
	out, done, err = tr.TransformStreamEvent([]byte(`{"type":"content-start","index":0}`))
	if err != nil || done || out != nil {
		t.Errorf("content-start: out=%s done=%v err=%v", out, done, err)
	}
}

func TestModalityCosts(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	// Transcription per minute: 2.5 min * $0.006/min
	route := &RouteResult{AudioPricePerMinute: f(0.006)}
	if got := CalculateTranscriptionCost(route, 150, nil); got != 0.015 {
		t.Errorf("transcription cost = %v", got)
	}

	// Transcription token-billed fallback
	route = &RouteResult{InputPrice: f(2.5), OutputPrice: f(10)}
	u := &TranscriptionUsage{Type: "tokens", InputTokens: 1_000_000, OutputTokens: 100_000}
	if got := CalculateTranscriptionCost(route, 0, u); got != 3.5 {
		t.Errorf("token transcription cost = %v", got)
	}

	// No pricing → 0
	if got := CalculateTranscriptionCost(&RouteResult{}, 60, nil); got != 0 {
		t.Errorf("no-price cost = %v", got)
	}

	// Speech: 2000 chars * $0.015/1k
	route = &RouteResult{SpeechPricePer1kChars: f(0.015)}
	if got := CalculateSpeechCost(route, 2000); got != 0.03 {
		t.Errorf("speech cost = %v", got)
	}

	// Rerank: 250 docs = 3 units, $2.00/1k units
	route = &RouteResult{RerankPricePer1k: f(2.0)}
	if got := CalculateRerankCost(route, 250); got != 0.006 {
		t.Errorf("rerank cost = %v", got)
	}
	// 100 docs = 1 unit
	if got := CalculateRerankCost(route, 100); got != 0.002 {
		t.Errorf("rerank cost 100 docs = %v", got)
	}
}

func TestSpeechContentType(t *testing.T) {
	if SpeechContentType("wav") != "audio/wav" {
		t.Error("wav")
	}
	if SpeechContentType("") != "audio/mpeg" {
		t.Error("default should be mp3")
	}
	if SpeechContentType("opus") != "audio/opus" {
		t.Error("opus")
	}
}
