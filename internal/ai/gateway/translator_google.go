package gateway

import (
	"encoding/json"
	"fmt"
	"time"
)

// GoogleTranslator converts between OpenAI format and Google Gemini API.
//
// Key differences:
//   - Uses contents/parts structure instead of messages
//   - Roles: "user"/"model" (not "assistant")
//   - System instruction is a separate top-level field
//   - generationConfig for parameters (temperature, maxOutputTokens, etc.)
//   - Tool declarations use functionDeclarations
//   - Response uses candidates[].content.parts
//   - Streaming returns complete candidate objects per chunk
type GoogleTranslator struct{}

// ---------- Request types (Gemini native) ----------

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type geminiFuncResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type geminiGenConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	MaxOutputTokens  *int     `json:"maxOutputTokens,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations,omitempty"`
}

type geminiFuncDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ---------- Response types (Gemini native) ----------

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
	ModelVersion  string            `json:"modelVersion,omitempty"`
}

type geminiCandidate struct {
	Content      *geminiContent `json:"content,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	Index        int            `json:"index"`
}

type geminiUsage struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

// ============================================================================
// TransformRequest
// ============================================================================

func (t *GoogleTranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	gr := geminiRequest{}

	// Generation config
	gc := &geminiGenConfig{
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxTokens,
	}
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			gc.StopSequences = []string{v}
		case []any:
			for _, s := range v {
				if str, ok := s.(string); ok {
					gc.StopSequences = append(gc.StopSequences, str)
				}
			}
		}
	}
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		gc.ResponseMimeType = "application/json"
	}
	gr.GenerationConfig = gc

	// Convert messages
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			contentStr := contentToString(msg.Content)
			gr.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: contentStr}},
			}

		case "user":
			contentStr := contentToString(msg.Content)
			gr.Contents = append(gr.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: contentStr}},
			})

		case "assistant":
			content := geminiContent{Role: "model"}

			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					var args any
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
					content.Parts = append(content.Parts, geminiPart{
						FunctionCall: &geminiFunctionCall{
							Name: tc.Function.Name,
							Args: args,
						},
					})
				}
			} else {
				contentStr := contentToString(msg.Content)
				content.Parts = []geminiPart{{Text: contentStr}}
			}
			gr.Contents = append(gr.Contents, content)

		case "tool":
			// Tool result → function response
			var respData any
			contentStr := contentToString(msg.Content)
			if err := json.Unmarshal([]byte(contentStr), &respData); err != nil {
				respData = map[string]string{"result": contentStr}
			}
			gr.Contents = append(gr.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFuncResponse{
						Name:     msg.Name,
						Response: respData,
					},
				}},
			})
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var decls []geminiFuncDecl
		for _, tool := range req.Tools {
			decls = append(decls, geminiFuncDecl{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			})
		}
		gr.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	return json.Marshal(gr)
}

// ============================================================================
// TransformResponse
// ============================================================================

func (t *GoogleTranslator) TransformResponse(body []byte) (*ChatResponse, error) {
	var gr geminiResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	resp := &ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   gr.ModelVersion,
	}

	for _, cand := range gr.Candidates {
		choice := Choice{Index: cand.Index}
		msg := &Message{Role: "assistant"}

		if cand.Content != nil {
			var textParts []string
			var toolCalls []ToolCall

			for _, part := range cand.Content.Parts {
				if part.Text != "" {
					textParts = append(textParts, part.Text)
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					toolCalls = append(toolCalls, ToolCall{
						ID:   fmt.Sprintf("call_%d", len(toolCalls)),
						Type: "function",
						Function: ToolCallFunction{
							Name:      part.FunctionCall.Name,
							Arguments: string(args),
						},
					})
				}
			}

			if len(textParts) > 0 {
				combined := ""
				for i, p := range textParts {
					if i > 0 {
						combined += "\n"
					}
					combined += p
				}
				msg.Content = combined
			}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
		}

		choice.Message = msg
		choice.FinishReason = mapGeminiFinishReason(cand.FinishReason)
		resp.Choices = append(resp.Choices, choice)
	}

	if gr.UsageMetadata != nil {
		resp.Usage = &Usage{
			PromptTokens:         gr.UsageMetadata.PromptTokenCount,
			CompletionTokens:     gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:          gr.UsageMetadata.TotalTokenCount,
			CacheReadInputTokens: gr.UsageMetadata.CachedContentTokenCount,
		}
	}

	return resp, nil
}

// ============================================================================
// TransformStreamEvent
// ============================================================================

func (t *GoogleTranslator) TransformStreamEvent(data []byte) ([]byte, bool, error) {
	// Gemini streaming returns complete response objects per chunk.
	// Each chunk contains the full candidates array with accumulated content.
	// We transform each into an OpenAI-style chunk.

	var gr geminiResponse
	if err := json.Unmarshal(data, &gr); err != nil {
		return nil, false, fmt.Errorf("failed to parse Gemini stream chunk: %w", err)
	}

	chunk := ChatStreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   gr.ModelVersion,
	}

	isDone := false

	for _, cand := range gr.Candidates {
		choice := Choice{Index: cand.Index}
		delta := &Message{}

		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if part.Text != "" {
					delta.Content = part.Text
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					delta.ToolCalls = append(delta.ToolCalls, ToolCall{
						ID:   fmt.Sprintf("call_%d", len(delta.ToolCalls)),
						Type: "function",
						Function: ToolCallFunction{
							Name:      part.FunctionCall.Name,
							Arguments: string(args),
						},
					})
				}
			}
		}

		choice.Delta = delta
		if cand.FinishReason != "" {
			choice.FinishReason = mapGeminiFinishReason(cand.FinishReason)
			isDone = true
		}

		chunk.Choices = append(chunk.Choices, choice)
	}

	if gr.UsageMetadata != nil {
		chunk.Usage = &Usage{
			PromptTokens:         gr.UsageMetadata.PromptTokenCount,
			CompletionTokens:     gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:          gr.UsageMetadata.TotalTokenCount,
			CacheReadInputTokens: gr.UsageMetadata.CachedContentTokenCount,
		}
	}

	out, err := json.Marshal(chunk)
	return out, isDone, err
}

// mapGeminiFinishReason converts Gemini finishReason to OpenAI finish_reason
func mapGeminiFinishReason(reason string) *string {
	if reason == "" {
		return nil
	}
	var mapped string
	switch reason {
	case "STOP":
		mapped = "stop"
	case "MAX_TOKENS":
		mapped = "length"
	case "SAFETY":
		mapped = "content_filter"
	case "RECITATION":
		mapped = "content_filter"
	default:
		mapped = "stop"
	}
	return &mapped
}

// contentToString extracts a string from message content (which can be string or []contentPart)
func contentToString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		// Multimodal content parts — extract text parts
		var texts []string
		for _, part := range v {
			if m, ok := part.(map[string]any); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		result := ""
		for i, t := range texts {
			if i > 0 {
				result += "\n"
			}
			result += t
		}
		return result
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}
