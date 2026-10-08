// Command fakellm is a scripted upstream LLM used by the end-to-end suite.
//
// It speaks the OpenAI, Anthropic, Google and Cohere request shapes well
// enough for the gateway's translators, and decides what to do from the
// model name it receives (the mapping's external_id):
//
//	fake-ok              200 with a short completion
//	fake-500             500 once per request (retryable)
//	fake-429             429 with Retry-After
//	fake-400             400 non-retryable error
//	fake-slow            sleeps FAKE_SLOW_SECONDS (default 20) before answering
//	fake-stream-abort    starts an SSE stream and closes the connection mid-way
//	fake-malformed       200 with invalid JSON
//	fake-nousage         200 without a usage block
//	fake-flaky-N         fails N times then succeeds (per model name, resets on /_reset)
//
// Every request is recorded; GET /_requests returns them, DELETE /_requests
// (or /_reset) clears them. GET /_health is the readiness probe.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type recorded struct {
	At      time.Time         `json:"at"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	Model   string            `json:"model"`
}

type server struct {
	mu       sync.Mutex
	requests []recorded
	flaky    map[string]int
}

func main() {
	addr := ":" + envOr("PORT", "9100")
	s := &server{flaky: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/_health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/_requests", s.handleRequests)
	mux.HandleFunc("/_reset", s.handleReset)
	mux.HandleFunc("/", s.handle)
	log.Printf("fakellm listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) handleRequests(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodDelete {
		s.requests = nil
		w.WriteHeader(204)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.requests)
}

func (s *server) handleReset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.requests = nil
	s.flaky = map[string]int{}
	s.mu.Unlock()
	w.WriteHeader(204)
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	model := modelFrom(r, body)
	stream := wantsStream(r, body)

	headers := map[string]string{}
	for _, h := range []string{"Authorization", "X-Api-Key", "Anthropic-Version", "X-Goog-Api-Key", "Api-Key", "Content-Type", "Accept", "User-Agent"} {
		if v := r.Header.Get(h); v != "" {
			headers[h] = v
		}
	}
	rec := recorded{At: time.Now(), Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: headers, Model: model}
	if json.Valid(body) {
		rec.Body = body
	} else {
		rec.Body, _ = json.Marshal(string(body))
	}
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	s.mu.Unlock()

	flavor := strings.TrimPrefix(model, "fake-")
	switch {
	case flavor == "500":
		errorJSON(w, 500, "simulated upstream failure")
		return
	case flavor == "429":
		w.Header().Set("Retry-After", "1")
		errorJSON(w, 429, "simulated rate limit")
		return
	case flavor == "400":
		errorJSON(w, 400, "simulated bad request")
		return
	case flavor == "slow":
		secs, _ := strconv.Atoi(envOr("FAKE_SLOW_SECONDS", "20"))
		select {
		case <-time.After(time.Duration(secs) * time.Second):
		case <-r.Context().Done():
			return
		}
	case flavor == "malformed":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "chatcmpl-broken", "choices": [`))
		return
	case strings.HasPrefix(flavor, "flaky-"):
		n, _ := strconv.Atoi(strings.TrimPrefix(flavor, "flaky-"))
		s.mu.Lock()
		seen := s.flaky[model]
		s.flaky[model] = seen + 1
		s.mu.Unlock()
		if seen < n {
			errorJSON(w, 503, fmt.Sprintf("flaky failure %d/%d", seen+1, n))
			return
		}
	}

	shape := shapeFor(r)
	if stream {
		s.streamResponse(w, r, shape, model, flavor == "stream-abort")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(syncResponse(shape, r.URL.Path, model, flavor != "nousage"))
}

// shapeFor picks the wire format from the path / auth headers.
func shapeFor(r *http.Request) string {
	p := r.URL.Path
	switch {
	case strings.HasSuffix(p, "/messages") || r.Header.Get("Anthropic-Version") != "":
		return "anthropic"
	case strings.Contains(p, ":generateContent") || strings.Contains(p, ":streamGenerateContent"):
		return "google"
	case r.Header.Get("X-Goog-Api-Key") != "":
		return "google"
	case strings.HasSuffix(p, "/chat") || strings.HasSuffix(p, "/embed") || (strings.HasSuffix(p, "/rerank") && !strings.Contains(p, "/v1/rerank")):
		return "cohere"
	default:
		return "openai"
	}
}

func modelFrom(r *http.Request, body []byte) string {
	var m struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &m)
	if m.Model != "" {
		return m.Model
	}
	// Google puts the model in the path: /models/<id>:generateContent
	if i := strings.Index(r.URL.Path, "/models/"); i >= 0 {
		rest := r.URL.Path[i+len("/models/"):]
		if j := strings.Index(rest, ":"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}

func wantsStream(r *http.Request, body []byte) bool {
	var m struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &m)
	return m.Stream || strings.Contains(r.URL.Path, ":streamGenerateContent")
}

const answer = "Hello from fakellm!"

func syncResponse(shape, path, model string, withUsage bool) any {
	now := time.Now().Unix()
	switch {
	case strings.HasSuffix(path, "/embeddings") || strings.HasSuffix(path, "/embed"):
		if shape == "cohere" {
			return map[string]any{"id": "emb-fake", "embeddings": map[string]any{"float": [][]float64{{0.1, 0.2, 0.3}}}, "meta": map[string]any{"billed_units": map[string]any{"input_tokens": 3}}}
		}
		return map[string]any{"object": "list", "model": model, "data": []any{map[string]any{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3}}}, "usage": map[string]any{"prompt_tokens": 3, "total_tokens": 3}}
	case strings.HasSuffix(path, "/moderations"):
		return map[string]any{"id": "modr-fake", "model": model, "results": []any{map[string]any{"flagged": false, "categories": map[string]any{}, "category_scores": map[string]any{}}}}
	case strings.HasSuffix(path, "/rerank"):
		return map[string]any{"id": "rerank-fake", "results": []any{map[string]any{"index": 0, "relevance_score": 0.9}}, "meta": map[string]any{"billed_units": map[string]any{"search_units": 1}}}
	case strings.HasSuffix(path, "/images/generations"):
		return map[string]any{"created": now, "data": []any{map[string]any{"url": "https://example.com/fake.png"}}}
	case strings.HasSuffix(path, "/audio/transcriptions"):
		return map[string]any{"text": "fake transcription"}
	case strings.HasSuffix(path, "/audio/speech"):
		return map[string]any{"note": "fakellm returns JSON for speech; real providers return audio bytes"}
	}

	switch shape {
	case "anthropic":
		resp := map[string]any{
			"id": "msg_fake", "type": "message", "role": "assistant", "model": model,
			"content":     []any{map[string]any{"type": "text", "text": answer}},
			"stop_reason": "end_turn",
		}
		if withUsage {
			resp["usage"] = map[string]any{"input_tokens": 5, "output_tokens": 4}
		}
		return resp
	case "google":
		resp := map[string]any{
			"candidates": []any{map[string]any{
				"content":      map[string]any{"role": "model", "parts": []any{map[string]any{"text": answer}}},
				"finishReason": "STOP", "index": 0,
			}},
		}
		if withUsage {
			resp["usageMetadata"] = map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 4, "totalTokenCount": 9}
		}
		return resp
	case "cohere":
		resp := map[string]any{
			"id": "cohere-fake", "finish_reason": "COMPLETE",
			"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": answer}}},
		}
		if withUsage {
			resp["usage"] = map[string]any{"billed_units": map[string]any{"input_tokens": 5, "output_tokens": 4}, "tokens": map[string]any{"input_tokens": 5, "output_tokens": 4}}
		}
		return resp
	default:
		resp := map[string]any{
			"id": "chatcmpl-fake", "object": "chat.completion", "created": now, "model": model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": answer}, "finish_reason": "stop"}},
		}
		if withUsage {
			resp["usage"] = map[string]any{"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
		}
		return resp
	}
}

func (s *server) streamResponse(w http.ResponseWriter, r *http.Request, shape, model string, abort bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fl, _ := w.(http.Flusher)
	words := strings.Split(answer, " ")
	write := func(event string, v any) {
		b, _ := json.Marshal(v)
		if event != "" {
			fmt.Fprintf(w, "event: %s\n", event)
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
		if fl != nil {
			fl.Flush()
		}
	}
	switch shape {
	case "anthropic":
		write("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_fake", "type": "message", "role": "assistant", "model": model, "content": []any{}, "usage": map[string]any{"input_tokens": 5, "output_tokens": 0}}})
		write("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
		for i, wd := range words {
			if abort && i == 1 {
				hijackClose(w)
				return
			}
			write("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": wd + " "}})
		}
		write("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
		write("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]any{"output_tokens": 4}})
		write("message_stop", map[string]any{"type": "message_stop"})
	case "google":
		for i, wd := range words {
			if abort && i == 1 {
				hijackClose(w)
				return
			}
			chunk := map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": wd + " "}}}, "index": 0}}}
			if i == len(words)-1 {
				chunk["candidates"].([]any)[0].(map[string]any)["finishReason"] = "STOP"
				chunk["usageMetadata"] = map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 4, "totalTokenCount": 9}
			}
			write("", chunk)
		}
	case "cohere":
		write("", map[string]any{"type": "message-start", "id": "cohere-fake"})
		for i, wd := range words {
			if abort && i == 1 {
				hijackClose(w)
				return
			}
			write("", map[string]any{"type": "content-delta", "index": 0, "delta": map[string]any{"message": map[string]any{"content": map[string]any{"text": wd + " "}}}})
		}
		write("", map[string]any{"type": "message-end", "delta": map[string]any{"finish_reason": "COMPLETE", "usage": map[string]any{"billed_units": map[string]any{"input_tokens": 5, "output_tokens": 4}}}})
	default:
		now := time.Now().Unix()
		for i, wd := range words {
			if abort && i == 1 {
				hijackClose(w)
				return
			}
			write("", map[string]any{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": now, "model": model,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": wd + " "}, "finish_reason": nil}}})
		}
		write("", map[string]any{"id": "chatcmpl-fake", "object": "chat.completion.chunk", "created": now, "model": model,
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}})
		fmt.Fprint(w, "data: [DONE]\n\n")
		if fl != nil {
			fl.Flush()
		}
	}
	_ = r
}

// hijackClose drops the TCP connection without a terminating chunk, so the
// client sees an aborted stream.
func hijackClose(w http.ResponseWriter) {
	if h, ok := w.(http.Hijacker); ok {
		conn, _, err := h.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}
}

func errorJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": msg, "type": "fake_error", "code": status}})
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
