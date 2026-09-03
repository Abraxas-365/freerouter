package gateway

import "math"

// ============================================================================
// Audio Transcription (OpenAI-compatible, POST /v1/audio/transcriptions)
// ============================================================================

// TranscriptionResponse is the OpenAI-compatible transcription response.
// json and verbose_json formats are parsed; other formats (text, srt, vtt)
// are proxied as-is.
type TranscriptionResponse struct {
	Text     string              `json:"text"`
	Language string              `json:"language,omitempty"`
	Duration float64             `json:"duration,omitempty"` // seconds (verbose_json)
	Usage    *TranscriptionUsage `json:"usage,omitempty"`    // token-billed STT models
}

// TranscriptionUsageType identifies how the upstream bills a transcription
// request. Typed for safe comparisons, but not validated on parse: unknown
// values from newer upstream API versions must not break proxying.
type TranscriptionUsageType string

const (
	TranscriptionUsageTokens   TranscriptionUsageType = "tokens"   // token-billed (e.g. gpt-4o-transcribe)
	TranscriptionUsageDuration TranscriptionUsageType = "duration" // duration-billed (e.g. whisper-1)
)

// TranscriptionUsage is returned by token-billed transcription models
// (e.g. gpt-4o-transcribe).
type TranscriptionUsage struct {
	Type         TranscriptionUsageType `json:"type,omitempty"`
	InputTokens  int                    `json:"input_tokens,omitempty"`
	OutputTokens int                    `json:"output_tokens,omitempty"`
	TotalTokens  int                    `json:"total_tokens,omitempty"`
	Seconds      float64                `json:"seconds,omitempty"`
}

// CalculateTranscriptionCost bills by audio duration when the mapping has
// per-minute pricing, falling back to token pricing for token-billed models.
func CalculateTranscriptionCost(route *RouteResult, durationSeconds float64, u *TranscriptionUsage) float64 {
	if route.AudioPricePerMinute != nil {
		seconds := durationSeconds
		if seconds == 0 && u != nil && u.Seconds > 0 {
			seconds = u.Seconds
		}
		if seconds > 0 {
			return seconds / 60.0 * *route.AudioPricePerMinute
		}
	}
	if u != nil && u.Type == "tokens" {
		cost := 0.0
		if route.InputPrice != nil {
			cost += float64(u.InputTokens) * *route.InputPrice / 1_000_000
		}
		if route.OutputPrice != nil {
			cost += float64(u.OutputTokens) * *route.OutputPrice / 1_000_000
		}
		return cost
	}
	return 0
}

// ============================================================================
// Speech Synthesis (OpenAI-compatible, POST /v1/audio/speech)
// ============================================================================

// SpeechRequest is the OpenAI-compatible TTS request.
type SpeechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	Instructions   string   `json:"instructions,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"` // mp3, opus, aac, flac, wav, pcm
	Speed          *float64 `json:"speed,omitempty"`
}

// CalculateSpeechCost bills TTS by input characters.
func CalculateSpeechCost(route *RouteResult, inputChars int) float64 {
	if route.SpeechPricePer1kChars == nil || inputChars == 0 {
		return 0
	}
	return float64(inputChars) / 1000.0 * *route.SpeechPricePer1kChars
}

// speechContentType maps a TTS response_format to its MIME type.
func SpeechContentType(format string) string {
	switch format {
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/mpeg" // mp3
	}
}

// ============================================================================
// Moderation (OpenAI-compatible, POST /v1/moderations)
// ============================================================================

// ModerationRequest is the OpenAI-compatible moderation request.
type ModerationRequest struct {
	Input any    `json:"input"` // string, []string, or multimodal array
	Model string `json:"model,omitempty"`
}

// ModerationResponse is the OpenAI-compatible moderation response.
type ModerationResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Results []map[string]any `json:"results"`
}

// ============================================================================
// Rerank (Cohere/Jina-compatible, POST /v1/rerank)
// ============================================================================

// RerankRequest is the Cohere v2-compatible rerank request.
type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	MaxTokensPerDoc *int     `json:"max_tokens_per_doc,omitempty"`
}

// RerankResponse is the Cohere v2-compatible rerank response.
type RerankResponse struct {
	ID      string         `json:"id,omitempty"`
	Results []RerankResult `json:"results"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// RerankResult is one ranked document.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// CalculateRerankCost bills rerank by search units: one unit per 100
// documents (rounded up), priced per 1000 units.
func CalculateRerankCost(route *RouteResult, numDocuments int) float64 {
	if route.RerankPricePer1k == nil || numDocuments == 0 {
		return 0
	}
	units := math.Ceil(float64(numDocuments) / 100.0)
	return units * *route.RerankPricePer1k / 1000.0
}
