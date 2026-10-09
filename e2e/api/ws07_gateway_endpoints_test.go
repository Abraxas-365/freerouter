//go:build e2e

package api

// WS-07 — gateway endpoints, protocol translation and request validation.

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ── /v1/chat/completions ─────────────────────────────────────────────

func TestWS07_ChatSync_OpenAIShape(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-sync")
	r := gw.Post(t, "/chat/completions", map[string]any{
		"model": "e2e-ok", "temperature": 0.3, "max_tokens": 33,
		"messages": []map[string]string{{"role": "system", "content": "sys"}, {"role": "user", "content": marker}},
	})
	if r.Status != 200 {
		t.Fatalf("want 200, got %d %s", r.Status, r.Body)
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q", ct)
	}
	m := r.Map(t)
	if m["object"] != "chat.completion" || m["model"] != "e2e-ok" {
		t.Errorf("object/model = %v/%v (want chat.completion / e2e-ok canonical name)", m["object"], m["model"])
	}
	if got := ws07ChatContent(t, m); got != "Hello from fakellm!" {
		t.Errorf("content %q", got)
	}
	u, _ := m["usage"].(map[string]any)
	if ws07Num(u["prompt_tokens"]) != 5 || ws07Num(u["completion_tokens"]) != 4 || ws07Num(u["total_tokens"]) != 9 {
		t.Errorf("usage %v", u)
	}
	// Upstream got the mapping's external id and the client's params, with Bearer auth.
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 1 {
		t.Fatalf("want 1 upstream call, got %d", len(recs))
	}
	rec := recs[0]
	if rec.Path != "/openai/v1/chat/completions" || rec.Headers["Authorization"] != "Bearer sk-e2e-openai" {
		t.Errorf("upstream path/auth = %s / %q", rec.Path, rec.Headers["Authorization"])
	}
	b := rec.BodyMap(t)
	if b["model"] != "fake-ok" || ws07Num(b["temperature"]) != 0.3 || ws07Num(b["max_tokens"]) != 33 {
		t.Errorf("upstream body %s", rec.Body)
	}
	// Usage is logged with tokens and cost (input 1.0 / output 2.0 per 1M).
	rows := ws07WaitUsage(t, "e2e-ok", 1)
	var found bool
	for _, row := range rows {
		if row["streamed"] == false && ws07Num(row["total_tokens"]) == 9 && math.Abs(ws07Num(row["total_cost"])-0.000013) < 1e-9 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no non-streamed usage row with 9 tokens / $0.000013 among %d rows", len(rows))
	}
}

func TestWS07_ChatStream_OpenAI(t *testing.T) {
	gw := ws07NewGateway(t)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok", Uniq("ws07-stream"), true), 15*time.Second)
	if s.Status != 200 {
		t.Fatalf("want 200, got %d %s", s.Status, s.Raw)
	}
	if ct := s.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type %q", ct)
	}
	chunks := s.DataJSON(t)
	if got := ws07StreamText(t, chunks); got != "Hello from fakellm! " {
		t.Errorf("streamed text %q", got)
	}
	if !s.HasDone() {
		t.Errorf("stream did not end with [DONE]:\n%s", s.Raw)
	}
	for _, c := range chunks {
		if c["object"] != "chat.completion.chunk" {
			t.Errorf("chunk object %v", c["object"])
		}
	}
}

// FINDING WS07-6: the chunk "model" leaks the upstream external id.
func TestWS07_ChatStream_ModelIsCanonicalName(t *testing.T) {
	gw := ws07NewGateway(t)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok", Uniq("ws07-stream-model"), true), 15*time.Second)
	for _, c := range s.DataJSON(t) {
		if m, ok := c["model"]; ok && m != "e2e-ok" {
			t.Fatalf("stream chunk model = %v, want e2e-ok (sync responses rewrite it; streams leak the upstream external_id)", m)
		}
	}
}

// FINDING WS07-5: SSE responses are fully buffered (Content-Length set, no chunking).
func TestWS07_ChatStream_IsIncremental(t *testing.T) {
	gw := ws07NewGateway(t)
	up := ws07NewLocal(t, func(w http.ResponseWriter, r *http.Request, _ *ws07LocalReq) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for i, word := range []string{"one ", "two ", "three"} {
			if i > 0 {
				time.Sleep(700 * time.Millisecond)
			}
			b, _ := json.Marshal(map[string]any{"id": "c", "object": "chat.completion.chunk", "model": "x",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": word}}}})
			_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
			fl.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		fl.Flush()
	})
	p := ws07NewProvider(t, "openai", Uniq("incr"), up.URL+"/v1")
	mid, name := ws07NewModel(t, "incr")
	ws07Map(t, mid, p.ID, "local-stream", 1, 1, nil)

	j, _ := json.Marshal(Chat(name, "x", true))
	req, _ := http.NewRequest(http.MethodPost, gw.Base+"/chat/completions", bytes.NewReader(j))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gw.Secret)
	start := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := make([]byte, 1)
	if _, err := res.Body.Read(buf); err != nil {
		t.Fatal(err)
	}
	first := time.Since(start)
	if first > 1000*time.Millisecond {
		t.Fatalf("first SSE byte arrived after %s (upstream sent its first chunk immediately and the rest over 1.4s); "+
			"Content-Length=%q — the gateway buffers the whole stream", first, res.Header.Get("Content-Length"))
	}
}

// ── Protocol translation (assert what the upstream received) ─────────

func TestWS07_Translation_Anthropic(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-anth")
	r := gw.Post(t, "/chat/completions", map[string]any{
		"model": "e2e-ok-anthropic", "max_tokens": 20, "temperature": 0.5,
		"messages": []map[string]string{{"role": "system", "content": "be brief"}, {"role": "user", "content": marker}},
	})
	if r.Status != 200 {
		t.Fatalf("want 200, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["object"] != "chat.completion" || ws07ChatContent(t, m) != "Hello from fakellm!" || m["model"] != "e2e-ok-anthropic" {
		t.Errorf("response not normalized to OpenAI shape: %s", r.Body)
	}
	if ch := m["choices"].([]any)[0].(map[string]any); ch["finish_reason"] != "stop" {
		t.Errorf("finish_reason %v (end_turn should map to stop)", ch["finish_reason"])
	}
	if u := m["usage"].(map[string]any); ws07Num(u["total_tokens"]) != 9 {
		t.Errorf("usage %v", u)
	}
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 1 {
		t.Fatalf("want 1 upstream call, got %d", len(recs))
	}
	rec := recs[0]
	if rec.Path != "/anthropic/v1/messages" {
		t.Errorf("path %s", rec.Path)
	}
	if rec.Headers["X-Api-Key"] != "sk-ant-e2e" || rec.Headers["Anthropic-Version"] != "2023-06-01" {
		t.Errorf("auth headers %v", rec.Headers)
	}
	if _, ok := rec.Headers["Authorization"]; ok {
		t.Errorf("anthropic upstream must not get Authorization header: %v", rec.Headers)
	}
	b := rec.BodyMap(t)
	if b["model"] != "fake-ok" || ws07Num(b["max_tokens"]) != 20 || ws07Num(b["temperature"]) != 0.5 {
		t.Errorf("body %s", rec.Body)
	}
	if b["system"] == nil {
		t.Errorf("system prompt not hoisted to top-level 'system': %s", rec.Body)
	}
	msgs, _ := b["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["role"] != "user" {
		t.Errorf("messages should contain only the user turn: %s", rec.Body)
	}
}

func TestWS07_Translation_Google(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-goog")
	r := gw.Post(t, "/chat/completions", map[string]any{
		"model": "e2e-ok-google", "max_tokens": 20, "temperature": 0.5,
		"messages": []map[string]string{{"role": "system", "content": "be brief"}, {"role": "user", "content": marker}},
	})
	if r.Status != 200 {
		t.Fatalf("want 200, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["object"] != "chat.completion" || ws07ChatContent(t, m) != "Hello from fakellm!" {
		t.Errorf("response not normalized: %s", r.Body)
	}
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 1 {
		t.Fatalf("want 1 upstream call, got %d", len(recs))
	}
	rec := recs[0]
	if rec.Path != "/google/v1beta/models/fake-ok:generateContent" {
		t.Errorf("path %s", rec.Path)
	}
	if rec.Headers["X-Goog-Api-Key"] != "AIza-e2e" {
		t.Errorf("auth headers %v", rec.Headers)
	}
	if _, ok := rec.Headers["Authorization"]; ok {
		t.Errorf("google upstream must not get Authorization: %v", rec.Headers)
	}
	b := rec.BodyMap(t)
	if _, ok := b["contents"]; !ok {
		t.Errorf("no contents: %s", rec.Body)
	}
	if _, ok := b["systemInstruction"]; !ok {
		t.Errorf("no systemInstruction: %s", rec.Body)
	}
	gc, _ := b["generationConfig"].(map[string]any)
	if ws07Num(gc["maxOutputTokens"]) != 20 || ws07Num(gc["temperature"]) != 0.5 {
		t.Errorf("generationConfig %v", gc)
	}
	if _, ok := b["model"]; ok {
		t.Errorf("google body should not carry model (it is in the path): %s", rec.Body)
	}
}

// FINDING WS07-21: Gemini streams lose the final chunk (last text + finish_reason + usage).
func TestWS07_Translation_GoogleStream(t *testing.T) {
	gw := ws07NewGateway(t)
	sm := Uniq("ws07-goog-s")
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok-google", sm, true), 15*time.Second)
	srecs := ws07UpstreamWith(t, sm)
	if len(srecs) != 1 || srecs[0].Path != "/google/v1beta/models/fake-ok:streamGenerateContent" || srecs[0].Query != "alt=sse" {
		t.Errorf("google stream upstream %+v", srecs)
	}
	if s.Status != 200 || !s.HasDone() {
		t.Fatalf("google stream: %d %s", s.Status, s.Raw)
	}
	if got := ws07StreamText(t, s.DataJSON(t)); got != "Hello from fakellm! " {
		t.Errorf("google stream text %q, want %q (upstream sent 3 chunks; the one with finishReason+usage is dropped)", got, "Hello from fakellm! ")
	}
}

// FINDING WS07-22: Cohere streams never emit `data: [DONE]`.
func TestWS07_Translation_CohereStream(t *testing.T) {
	gw := ws07NewGateway(t)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok-cohere", Uniq("ws07-coh-s"), true), 15*time.Second)
	if s.Status != 200 || ws07StreamText(t, s.DataJSON(t)) != "Hello from fakellm! " {
		t.Fatalf("cohere stream: %d %s", s.Status, s.Raw)
	}
	if !s.HasDone() {
		t.Errorf("cohere stream did not terminate with data: [DONE] (OpenAI clients wait for it)")
	}
}

func TestWS07_Translation_Cohere(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-coh")
	r := gw.Post(t, "/chat/completions", map[string]any{
		"model": "e2e-ok-cohere", "max_tokens": 20,
		"messages": []map[string]string{{"role": "user", "content": marker}},
	})
	if r.Status != 200 {
		t.Fatalf("want 200, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["object"] != "chat.completion" || ws07ChatContent(t, m) != "Hello from fakellm!" {
		t.Errorf("response not normalized: %s", r.Body)
	}
	if ch := m["choices"].([]any)[0].(map[string]any); ch["finish_reason"] != "stop" {
		t.Errorf("finish_reason %v (COMPLETE should map to stop)", ch["finish_reason"])
	}
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 1 {
		t.Fatalf("want 1 upstream call, got %d", len(recs))
	}
	rec := recs[0]
	if rec.Path != "/cohere/v2/chat" || rec.Headers["Authorization"] != "Bearer co-e2e" {
		t.Errorf("path/auth %s %v", rec.Path, rec.Headers)
	}
	if b := rec.BodyMap(t); b["model"] != "fake-ok" || ws07Num(b["max_tokens"]) != 20 {
		t.Errorf("body %s", rec.Body)
	}
}

func TestWS07_Translation_AnthropicStream(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-anth-s")
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok-anthropic", marker, true), 15*time.Second)
	if s.Status != 200 {
		t.Fatalf("status %d", s.Status)
	}
	chunks := s.DataJSON(t)
	if got := ws07StreamText(t, chunks); got != "Hello from fakellm! " {
		t.Errorf("text %q", got)
	}
	if !s.HasDone() {
		t.Error("no [DONE]")
	}
	var finish any
	for _, c := range chunks {
		if ch, _ := c["choices"].([]any); len(ch) > 0 {
			if f := ch[0].(map[string]any)["finish_reason"]; f != nil {
				finish = f
			}
		}
	}
	if finish != "stop" {
		t.Errorf("final finish_reason %v", finish)
	}
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 1 || recs[0].BodyMap(t)["stream"] != true || recs[0].Headers["Accept"] != "text/event-stream" {
		t.Errorf("anthropic stream upstream %+v", recs)
	}
}

// FINDING WS07-7: Anthropic streaming usage is split across message_start
// (input) and message_delta (output) and never totalled, so stream usage logs 0 tokens.
func TestWS07_Translation_AnthropicStreamUsageLogged(t *testing.T) {
	gw := ws07NewGateway(t)
	_, p := ws07SimpleModel(t, "anthropic", Uniq("anthu"), "fake-ok")
	_ = p
	mid, name := ws07NewModel(t, "anthu2")
	ws07Map(t, mid, p.ID, "fake-ok", 1, 2, nil)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat(name, "x", true), 15*time.Second)
	if s.Status != 200 {
		t.Fatalf("status %d", s.Status)
	}
	rows := ws07WaitUsage(t, name, 1)
	if ws07Num(rows[0]["prompt_tokens"]) != 5 || ws07Num(rows[0]["completion_tokens"]) != 4 || ws07Num(rows[0]["total_tokens"]) != 9 {
		t.Fatalf("anthropic stream usage row: prompt=%v completion=%v total=%v cost=%v (upstream reported 5 in / 4 out)",
			rows[0]["prompt_tokens"], rows[0]["completion_tokens"], rows[0]["total_tokens"], rows[0]["total_cost"])
	}
}

// ── /v1/messages (Anthropic-shaped inbound) ──────────────────────────

func TestWS07_Messages(t *testing.T) {
	gw := ws07NewGateway(t)
	t.Run("sync via openai upstream", func(t *testing.T) {
		marker := Uniq("ws07-msg")
		r := gw.Post(t, "/messages", map[string]any{
			"model": "e2e-ok", "max_tokens": 50, "system": "sys-prompt",
			"messages": []map[string]any{{"role": "user", "content": marker}},
		})
		if r.Status != 200 {
			t.Fatalf("want 200, got %d %s", r.Status, r.Body)
		}
		m := r.Map(t)
		if m["type"] != "message" || m["role"] != "assistant" || m["model"] != "e2e-ok" || m["stop_reason"] != "end_turn" {
			t.Errorf("not anthropic-shaped: %s", r.Body)
		}
		content, _ := m["content"].([]any)
		if len(content) != 1 || content[0].(map[string]any)["text"] != "Hello from fakellm!" {
			t.Errorf("content %v", m["content"])
		}
		u, _ := m["usage"].(map[string]any)
		if ws07Num(u["input_tokens"]) != 5 || ws07Num(u["output_tokens"]) != 4 {
			t.Errorf("usage %v", u)
		}
		recs := ws07UpstreamWith(t, marker)
		if len(recs) != 1 || recs[0].Path != "/openai/v1/chat/completions" {
			t.Fatalf("upstream %+v", recs)
		}
		b := recs[0].BodyMap(t)
		msgs, _ := b["messages"].([]any)
		if b["model"] != "fake-ok" || len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || ws07Num(b["max_tokens"]) != 50 {
			t.Errorf("upstream body %s", recs[0].Body)
		}
	})
	t.Run("stream event sequence", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/messages", map[string]any{
			"model": "e2e-ok", "max_tokens": 50, "stream": true,
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		}, 15*time.Second)
		if s.Status != 200 {
			t.Fatalf("status %d", s.Status)
		}
		names := strings.Join(s.EventNames(), ",")
		want := "message_start,content_block_start,content_block_delta,content_block_delta,content_block_delta,content_block_stop,message_delta,message_stop"
		if names != want {
			t.Errorf("events = %s\nwant     %s", names, want)
		}
	})
	t.Run("stream upstream error is an anthropic error event", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/messages", map[string]any{
			"model": "e2e-500", "max_tokens": 10, "stream": true,
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		}, 20*time.Second)
		if len(s.Events) == 0 || s.Events[len(s.Events)-1].Event != "error" {
			t.Errorf("want terminal 'error' event, got %v\n%s", s.EventNames(), s.Raw)
		}
	})
	t.Run("stream fallback succeeds", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/messages", map[string]any{
			"model": "e2e-fallback", "max_tokens": 10, "stream": true,
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		}, 20*time.Second)
		names := s.EventNames()
		if len(names) == 0 || names[len(names)-1] != "message_stop" {
			t.Errorf("fallback stream events %v", names)
		}
	})
	t.Run("errors are anthropic-shaped", func(t *testing.T) {
		cases := []struct {
			name   string
			body   any
			status int
			etype  string
		}{
			{"unknown model", map[string]any{"model": "ws07-nope", "max_tokens": 5, "messages": []any{map[string]any{"role": "user", "content": "x"}}}, 404, "api_error"},
			{"malformed json", "{bad", 400, "invalid_request_error"},
			{"missing model", map[string]any{"max_tokens": 5, "messages": []any{}}, 400, "invalid_request_error"},
		}
		for _, tc := range cases {
			r := gw.Post(t, "/messages", tc.body)
			if r.Status != tc.status {
				t.Errorf("%s: status %d want %d (%s)", tc.name, r.Status, tc.status, r.Body)
				continue
			}
			m := r.Map(t)
			e, _ := m["error"].(map[string]any)
			if m["type"] != "error" || e["type"] != tc.etype || e["message"] == "" {
				t.Errorf("%s: body %s", tc.name, r.Body)
			}
		}
	})
}

// FINDING WS07-11: /v1/messages accepts an empty messages array and calls upstream.
func TestWS07_Messages_EmptyMessagesRejected(t *testing.T) {
	gw := ws07NewGateway(t)
	name, p := ws07SimpleModel(t, "openai", Uniq("msgempty"), "fake-ok")
	r := gw.Post(t, "/messages", map[string]any{"model": name, "max_tokens": 5, "messages": []any{}})
	if r.Status != 400 {
		t.Errorf("empty messages: want 400 invalid_request_error, got %d %s (upstream calls: %d)", r.Status, r.Body, len(ws07Upstream(t, p.Seg)))
	}
}

// ── /v1/responses ────────────────────────────────────────────────────

func TestWS07_Responses(t *testing.T) {
	gw := ws07NewGateway(t)
	t.Run("sync", func(t *testing.T) {
		marker := Uniq("ws07-resp")
		r := gw.Post(t, "/responses", map[string]any{"model": "e2e-ok", "input": marker, "instructions": "be nice", "max_output_tokens": 40})
		if r.Status != 200 {
			t.Fatalf("want 200, got %d %s", r.Status, r.Body)
		}
		m := r.Map(t)
		if m["object"] != "response" || m["status"] != "completed" || m["model"] != "e2e-ok" {
			t.Errorf("shape %s", r.Body)
		}
		out, _ := m["output"].([]any)
		if len(out) != 1 {
			t.Fatalf("output %v", m["output"])
		}
		o := out[0].(map[string]any)
		c := o["content"].([]any)[0].(map[string]any)
		if o["type"] != "message" || c["type"] != "output_text" || c["text"] != "Hello from fakellm!" {
			t.Errorf("output %v", o)
		}
		u := m["usage"].(map[string]any)
		if ws07Num(u["input_tokens"]) != 5 || ws07Num(u["output_tokens"]) != 4 || ws07Num(u["total_tokens"]) != 9 {
			t.Errorf("usage %v", u)
		}
		recs := ws07UpstreamWith(t, marker)
		if len(recs) != 1 {
			t.Fatalf("upstream calls %d", len(recs))
		}
		b := recs[0].BodyMap(t)
		msgs, _ := b["messages"].([]any)
		if b["model"] != "fake-ok" || len(msgs) != 2 || msgs[0].(map[string]any)["content"] != "be nice" || ws07Num(b["max_tokens"]) != 40 {
			t.Errorf("upstream body %s", recs[0].Body)
		}
	})
	t.Run("stream", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/responses", map[string]any{"model": "e2e-ok", "input": "x", "stream": true}, 15*time.Second)
		names := s.EventNames()
		if s.Status != 200 || len(names) == 0 || names[0] != "response.created" || names[len(names)-1] != "response.completed" {
			t.Errorf("events %v", names)
		}
		var text string
		for i, e := range s.Events {
			if e.Event == "response.output_text.delta" {
				d := s.DataJSON(t)[i]
				text += d["delta"].(string)
			}
		}
		if text != "Hello from fakellm! " {
			t.Errorf("delta text %q", text)
		}
	})
	t.Run("stream upstream failure → response.failed", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/responses", map[string]any{"model": "e2e-500", "input": "x", "stream": true}, 20*time.Second)
		names := s.EventNames()
		if len(names) == 0 || names[len(names)-1] != "response.failed" {
			t.Errorf("events %v", names)
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			body   any
			status int
		}{
			{"unknown model", map[string]any{"model": "ws07-nope", "input": "x"}, 404},
			{"malformed json", "{bad", 400},
			{"missing model", map[string]any{"input": "x"}, 400},
		} {
			r := gw.Post(t, "/responses", tc.body)
			if r.Status != tc.status {
				t.Errorf("%s: %d want %d %s", tc.name, r.Status, tc.status, r.Body)
				continue
			}
			if e, _ := r.Map(t)["error"].(map[string]any); e == nil || e["message"] == "" {
				t.Errorf("%s: not an OpenAI error envelope: %s", tc.name, r.Body)
			}
		}
	})
}

// FINDING WS07-11: /v1/responses with no input sends an empty conversation upstream.
func TestWS07_Responses_MissingInputRejected(t *testing.T) {
	gw := ws07NewGateway(t)
	name, p := ws07SimpleModel(t, "openai", Uniq("respempty"), "fake-ok")
	for _, body := range []map[string]any{{"model": name}, {"model": name, "input": ""}, {"model": name, "input": []any{}}} {
		r := gw.Post(t, "/responses", body)
		if r.Status != 400 {
			t.Errorf("body %v: want 400, got %d %s", body, r.Status, r.Body)
		}
	}
	if n := len(ws07Upstream(t, p.Seg)); n != 0 {
		t.Errorf("upstream was called %d times with an empty conversation", n)
	}
}

// ── /v1/models, /v1/cost/estimate ────────────────────────────────────

func TestWS07_Models(t *testing.T) {
	gw := ws07NewGateway(t)
	mid, name := ws07NewModel(t, "listed")
	_ = mid
	r := gw.Get(t, "/models")
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var list struct {
		Object string
		Data   []map[string]any
	}
	r.JSON(t, &list)
	if list.Object != "list" {
		t.Errorf("object %q", list.Object)
	}
	ids := map[string]bool{}
	for _, m := range list.Data {
		if m["object"] != "model" || m["id"] == "" {
			t.Errorf("bad model object %v", m)
		}
		ids[m["id"].(string)] = true
	}
	for _, want := range []string{"e2e-ok", "e2e-ok-anthropic", "e2e-fallback", name} {
		if !ids[want] {
			t.Errorf("%s not listed", want)
		}
	}
	// Inactive models disappear.
	admin := AdminAPI(t)
	if u := admin.Put(t, "/models/"+mid, map[string]any{"status": "inactive"}); u.Status != 200 && u.Status != 204 {
		t.Fatalf("deactivate: %d %s", u.Status, u.Body)
	}
	r2 := gw.Get(t, "/models")
	r2.JSON(t, &list)
	for _, m := range list.Data {
		if m["id"] == name {
			t.Errorf("inactive model %s still listed", name)
		}
	}
	// And are not routable.
	if c := gw.Post(t, "/chat/completions", Chat(name, "x", false)); c.Status != 404 {
		t.Errorf("chat to inactive model: %d %s", c.Status, c.Body)
	}
	// Anonymous → 401.
	if a := Anon(FX(t).URLs.Gateway).Get(t, "/models"); a.Status != 401 {
		t.Errorf("anon /models: %d", a.Status)
	}
}

func TestWS07_CostEstimate(t *testing.T) {
	gw := ws07NewGateway(t)
	r := gw.Post(t, "/cost/estimate", map[string]any{
		"model": "e2e-ok", "max_tokens": 100,
		"messages": []map[string]string{{"role": "user", "content": "hello world 1234"}}, // 16 chars → 4 tok + 4 overhead + 2 priming
	})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["model"] != "e2e-ok" || ws07Num(m["estimated_input_tokens"]) != 10 || ws07Num(m["estimated_output_tokens"]) != 100 {
		t.Errorf("estimate %s", r.Body)
	}
	// e2e-ok: input 1.0 / output 2.0 USD per 1M tokens.
	if math.Abs(ws07Num(m["input_cost_usd"])-0.00001) > 1e-12 || math.Abs(ws07Num(m["output_cost_usd"])-0.0002) > 1e-12 ||
		math.Abs(ws07Num(m["total_cost_usd"])-0.00021) > 1e-12 {
		t.Errorf("costs %s", r.Body)
	}
	// Default output tokens when max_tokens missing.
	d := gw.Post(t, "/cost/estimate", map[string]any{"model": "e2e-ok", "messages": []map[string]string{{"role": "user", "content": "x"}}}).Map(t)
	if ws07Num(d["estimated_output_tokens"]) != 4096 {
		t.Errorf("default output tokens %v", d["estimated_output_tokens"])
	}
	// No upstream call is made.
	if recs := ws07UpstreamWith(t, "hello world 1234"); len(recs) != 0 {
		t.Errorf("cost estimate called upstream %d times", len(recs))
	}
	for _, tc := range []struct {
		body   any
		status int
	}{
		{map[string]any{"model": "ws07-nope"}, 404},
		{map[string]any{}, 400},
		{"{bad", 400},
	} {
		if r := gw.Post(t, "/cost/estimate", tc.body); r.Status != tc.status {
			t.Errorf("%v: %d want %d %s", tc.body, r.Status, tc.status, r.Body)
		}
	}
}

// FINDING WS07-12: negative max_tokens is silently replaced by 4096 instead of rejected.
func TestWS07_CostEstimate_NegativeMaxTokensRejected(t *testing.T) {
	gw := ws07NewGateway(t)
	r := gw.Post(t, "/cost/estimate", map[string]any{"model": "e2e-ok", "max_tokens": -5, "messages": []map[string]string{{"role": "user", "content": "x"}}})
	if r.Status != 400 {
		t.Errorf("max_tokens=-5: want 400, got %d %s", r.Status, r.Body)
	}
}

// ── Modalities ───────────────────────────────────────────────────────

func TestWS07_Modalities_Passthrough(t *testing.T) {
	gw := ws07NewGateway(t)
	// Own provider so we can see exactly what the upstream got.
	seg := Uniq("modal")
	p := ws07NewProvider(t, "openai", seg)
	mid, name := ws07NewModel(t, "modal")
	ws07Map(t, mid, p.ID, "fake-ok", 1, 2, map[string]any{"audio": true, "speech": true, "moderation": true, "rerank": true})

	cases := []struct {
		path, upstream string
		body           map[string]any
		check          func(m map[string]any) bool
	}{
		{"/embeddings", "/embeddings", map[string]any{"model": name, "input": "hello"}, func(m map[string]any) bool {
			d, _ := m["data"].([]any)
			return m["object"] == "list" && len(d) == 1
		}},
		{"/moderations", "/moderations", map[string]any{"model": name, "input": "hello"}, func(m map[string]any) bool {
			res, _ := m["results"].([]any)
			return len(res) == 1
		}},
		{"/rerank", "/rerank", map[string]any{"model": name, "query": "q", "documents": []string{"a", "b"}}, func(m map[string]any) bool {
			res, _ := m["results"].([]any)
			return len(res) == 1
		}},
		{"/images/generations", "/images/generations", map[string]any{"model": name, "prompt": "a cat"}, func(m map[string]any) bool {
			d, _ := m["data"].([]any)
			return len(d) == 1
		}},
	}
	for _, tc := range cases {
		t.Run(strings.TrimPrefix(tc.path, "/"), func(t *testing.T) {
			before := len(ws07Upstream(t, seg))
			r := gw.Post(t, tc.path, tc.body)
			if r.Status != 200 {
				t.Fatalf("%d %s", r.Status, r.Body)
			}
			if !tc.check(r.Map(t)) {
				t.Errorf("unexpected body %s", r.Body)
			}
			recs := ws07Upstream(t, seg)[before:]
			if len(recs) != 1 {
				t.Fatalf("upstream calls %d", len(recs))
			}
			if recs[0].Path != "/ws07/"+seg+"/v1"+tc.upstream || recs[0].Headers["Authorization"] != "Bearer "+p.Token {
				t.Errorf("upstream path/auth %s %v", recs[0].Path, recs[0].Headers)
			}
		})
	}
	t.Run("errors", func(t *testing.T) {
		for _, path := range []string{"/embeddings", "/moderations", "/rerank", "/images/generations", "/audio/speech"} {
			if r := gw.Post(t, path, map[string]any{"input": "x"}); r.Status != 400 {
				t.Errorf("%s missing model: %d %s", path, r.Status, r.Body)
			}
			if r := gw.Post(t, path, "nope"); r.Status != 400 {
				t.Errorf("%s malformed: %d %s", path, r.Status, r.Body)
			}
			if r := gw.Post(t, path, map[string]any{"model": "ws07-nope", "input": "x"}); r.Status != 404 {
				t.Errorf("%s unknown model: %d %s", path, r.Status, r.Body)
			}
		}
	})
}

// FINDING WS07-1: modality passthrough forwards the client-facing model name
// instead of the mapping's external_id.
func TestWS07_Modalities_UseExternalID(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("modext")
	p := ws07NewProvider(t, "openai", seg)
	mid, name := ws07NewModel(t, "modext")
	ws07Map(t, mid, p.ID, "fake-ok", 1, 2, nil)
	for _, path := range []string{"/embeddings", "/moderations", "/images/generations"} {
		gw.Post(t, path, map[string]any{"model": name, "input": "x", "prompt": "x"})
	}
	for _, rec := range ws07Upstream(t, seg) {
		if rec.Model != "fake-ok" {
			t.Errorf("%s: upstream got model %q, want external_id %q", rec.Path, rec.Model, "fake-ok")
		}
	}
}

// FINDING WS07-2: modality endpoints ignore upstream errors' retry/fallback and the provider protocol.
func TestWS07_Modalities_ProtocolAware(t *testing.T) {
	gw := ws07NewGateway(t)
	// Embedding against an anthropic-protocol model is POSTed to /messages
	// and the Anthropic chat answer is relayed as a 200 "embedding".
	r := gw.Post(t, "/embeddings", map[string]any{"model": "e2e-ok-anthropic", "input": "hello"})
	if r.Status == 200 {
		m := r.Map(t)
		if _, ok := m["data"]; !ok {
			t.Errorf("anthropic has no embeddings API; gateway returned 200 with a non-embedding body: %s", r.Body)
		}
	}
}

func TestWS07_Modalities_UpstreamErrorPropagates(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("moderr")
	p := ws07NewProvider(t, "openai", seg)
	mid, name := ws07NewModel(t, "moderr")
	// external id fake-500 → but passthrough sends `name` (WS07-1), so use a local upstream returning 500.
	_ = p
	up := ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	})
	p2 := ws07NewProvider(t, "openai", Uniq("moderr2"), up.URL+"/v1")
	ws07Map(t, mid, p2.ID, "x", 1, 1, nil)
	r := gw.Post(t, "/embeddings", map[string]any{"model": name, "input": "x"})
	if r.Status != 502 {
		t.Errorf("upstream 500 on embeddings: want 502, got %d %s", r.Status, r.Body)
	}
}

func TestWS07_AudioSpeech(t *testing.T) {
	gw := ws07NewGateway(t)
	audio := []byte("ID3\x04\x00fake-mp3-bytes\x00\xff")
	up := ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write(audio)
	})
	p := ws07NewProvider(t, "openai", Uniq("tts"), up.URL+"/v1")
	mid, name := ws07NewModel(t, "tts")
	ws07Map(t, mid, p.ID, "tts-ext", 0, 0, map[string]any{"speech": true, "speech_price_per_1k_chars": 15.0})
	r := gw.Post(t, "/audio/speech", map[string]any{"model": name, "input": "hello there", "voice": "alloy", "response_format": "mp3"})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	reqs := up.Reqs()
	if len(reqs) != 1 || reqs[0].Path != "/v1/audio/speech" || reqs[0].Header.Get("Authorization") != "Bearer "+p.Token {
		t.Fatalf("upstream %+v", reqs)
	}
	if !bytes.Equal(r.Body, audio) {
		t.Errorf("audio bytes not relayed verbatim: %q", r.Body)
	}
	// FINDING WS07-3: Content-Type is forced to application/json for binary audio.
	if ct := r.Header.Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("speech Content-Type = %q, want audio/mpeg", ct)
	}
}

// FINDING WS07-4: /v1/audio/transcriptions only accepts JSON; multipart (the OpenAI API contract) → 400.
func TestWS07_AudioTranscription_Multipart(t *testing.T) {
	gw := ws07NewGateway(t)
	up := ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"fake transcription"}`))
	})
	p := ws07NewProvider(t, "openai", Uniq("stt"), up.URL+"/v1")
	mid, name := ws07NewModel(t, "stt")
	ws07Map(t, mid, p.ID, "stt-ext", 0, 0, map[string]any{"audio": true, "audio_price_per_minute": 0.006})
	body, ct := ws07Multipart(map[string]string{"model": name, "response_format": "json"}, "file", "hello.wav", []byte("RIFF....WAVEfmt fake"))
	c := &Client{Base: gw.Base, Headers: map[string]string{"Authorization": "Bearer " + gw.Secret, "Content-Type": ct}, HTTP: httpClient}
	r := c.Do(t, http.MethodPost, "/audio/transcriptions", body)
	if r.Status != 200 {
		t.Fatalf("multipart transcription: want 200, got %d %s (upstream calls: %d)", r.Status, r.Body, up.Count())
	}
	if r.Map(t)["text"] != "fake transcription" {
		t.Errorf("body %s", r.Body)
	}
	reqs := up.Reqs()
	if len(reqs) != 1 || reqs[0].Path != "/v1/audio/transcriptions" || !strings.HasPrefix(reqs[0].Header.Get("Content-Type"), "multipart/form-data") {
		t.Errorf("upstream did not receive multipart: %+v", reqs)
	}
}

// ── Request validation ───────────────────────────────────────────────

func TestWS07_ChatValidation(t *testing.T) {
	gw := ws07NewGateway(t)
	cases := []struct {
		name    string
		body    any
		status  int
		msgPart string
	}{
		{"unknown model", Chat("ws07-does-not-exist", "x", false), 404, "model not found"},
		{"unknown model stream", Chat("ws07-does-not-exist", "x", true), 404, "model not found"},
		{"missing model", map[string]any{"messages": []map[string]string{{"role": "user", "content": "x"}}}, 400, "model is required"},
		{"empty model", Chat("", "x", false), 400, "model is required"},
		{"malformed json", "{bad", 400, "invalid request body"},
		{"empty body", "", 400, "invalid request body"},
		{"wrong type model", map[string]any{"model": 123, "messages": []any{}}, 400, "invalid request body"},
		{"empty messages", map[string]any{"model": "e2e-ok", "messages": []any{}}, 400, "messages is required"},
		{"missing messages", map[string]any{"model": "e2e-ok"}, 400, "messages is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gw.Post(t, "/chat/completions", tc.body)
			if r.Status != tc.status {
				t.Fatalf("status %d want %d: %s", r.Status, tc.status, r.Body)
			}
			m := r.Map(t)
			if msg, _ := m["message"].(string); !strings.Contains(msg, tc.msgPart) {
				t.Errorf("message %q does not contain %q", msg, tc.msgPart)
			}
			if int(ws07Num(m["status_code"])) != tc.status {
				t.Errorf("status_code field %v", m["status_code"])
			}
		})
	}
	t.Run("oversized body → 413", func(t *testing.T) {
		big := map[string]any{"model": "e2e-ok", "messages": []map[string]string{{"role": "user", "content": strings.Repeat("a", 5<<20)}}}
		r := gw.Post(t, "/chat/completions", big)
		if r.Status != http.StatusRequestEntityTooLarge {
			t.Fatalf("5MB body: %d %s", r.Status, r.Body)
		}
	})
	t.Run("just under body limit is accepted", func(t *testing.T) {
		marker := Uniq("ws07-big")
		big := map[string]any{"model": "e2e-ok", "messages": []map[string]string{{"role": "user", "content": marker + strings.Repeat("a", 3<<20)}}}
		r := gw.Post(t, "/chat/completions", big)
		if r.Status != 200 {
			t.Fatalf("3MB body: %d %.300s", r.Status, r.Body)
		}
	})
	t.Run("unicode round-trips", func(t *testing.T) {
		marker := Uniq("ws07-uni") + " héllo 你好 🚀 \u202e"
		r := gw.Post(t, "/chat/completions", Chat("e2e-ok", marker, false))
		if r.Status != 200 {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
		recs := ws07UpstreamWith(t, "ws07-uni")
		found := false
		for _, rec := range recs {
			msgs := rec.BodyMap(t)["messages"].([]any)
			if msgs[0].(map[string]any)["content"] == marker {
				found = true
			}
		}
		if !found {
			t.Errorf("unicode content not forwarded verbatim")
		}
	})
	t.Run("unknown route", func(t *testing.T) {
		if r := gw.Post(t, "/does-not-exist", map[string]any{}); r.Status != 404 {
			t.Errorf("%d", r.Status)
		}
	})
	t.Run("unauthenticated", func(t *testing.T) {
		if r := Anon(FX(t).URLs.Gateway).Post(t, "/chat/completions", Chat("e2e-ok", "x", false)); r.Status != 401 {
			t.Errorf("%d", r.Status)
		}
	})
}

// FINDING WS07-13: a non-JSON Content-Type yields "invalid request body: Unprocessable Entity".
func TestWS07_ChatValidation_TextPlainContentType(t *testing.T) {
	gw := ws07NewGateway(t)
	c := &Client{Base: gw.Base, Headers: map[string]string{"Authorization": "Bearer " + gw.Secret, "Content-Type": "text/plain"}, HTTP: httpClient}
	r := c.Do(t, http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"e2e-ok","messages":[{"role":"user","content":"x"}]}`))
	if r.Status != 415 && r.Status != 200 {
		t.Errorf("JSON body with Content-Type text/plain: want 415 (or accept it), got %d %s", r.Status, r.Body)
	}
}
