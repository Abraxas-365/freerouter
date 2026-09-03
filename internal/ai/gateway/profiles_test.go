package gateway

import (
	"net/http"
	"testing"
)

func TestGetProfileKnownProviders(t *testing.T) {
	tests := []struct {
		providerID string
		baseURL    string
	}{
		{"openai", "https://api.openai.com/v1"},
		{"groq", "https://api.groq.com/openai/v1"},
		{"together", "https://api.together.xyz/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"xai", "https://api.x.ai/v1"},
		{"fireworks", "https://api.fireworks.ai/inference/v1"},
		{"perplexity", "https://api.perplexity.ai"},
		{"openrouter", "https://openrouter.ai/api/v1"},
		{"cerebras", "https://api.cerebras.ai/v1"},
		{"deepinfra", "https://api.deepinfra.com/v1/openai"},
		{"anthropic", "https://api.anthropic.com/v1"},
		{"cohere", "https://api.cohere.com/v2"},
		{"azure-openai", ""},
	}
	for _, tt := range tests {
		p := GetProfile(tt.providerID)
		if p.DefaultBaseURL != tt.baseURL {
			t.Errorf("%s: base URL = %q, want %q", tt.providerID, p.DefaultBaseURL, tt.baseURL)
		}
		if p.Translator == nil {
			t.Errorf("%s: nil translator", tt.providerID)
		}
		if p.BuildURL == nil {
			t.Errorf("%s: nil BuildURL", tt.providerID)
		}
	}
}

func TestGetProfileUnknownFallsBackToOpenAI(t *testing.T) {
	p := GetProfile("some-new-provider")
	if p.ID != "some-new-provider" {
		t.Errorf("ID = %q", p.ID)
	}
	if _, ok := p.Translator.(*OpenAITranslator); !ok {
		t.Errorf("expected OpenAI passthrough translator, got %T", p.Translator)
	}
	url := p.BuildURL("https://example.com/v1", "m", EndpointChat, false)
	if url != "https://example.com/v1/chat/completions" {
		t.Errorf("url = %q", url)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		base       string
		externalID string
		endpoint   Endpoint
		stream     bool
		want       string
	}{
		{"openai chat", "openai", "https://api.openai.com/v1", "gpt-4o", EndpointChat, false, "https://api.openai.com/v1/chat/completions"},
		{"openai transcription", "openai", "https://api.openai.com/v1", "whisper-1", EndpointTranscription, false, "https://api.openai.com/v1/audio/transcriptions"},
		{"openai speech", "openai", "https://api.openai.com/v1", "tts-1", EndpointSpeech, false, "https://api.openai.com/v1/audio/speech"},
		{"openai moderation", "openai", "https://api.openai.com/v1", "omni-moderation-latest", EndpointModeration, false, "https://api.openai.com/v1/moderations"},
		{"anthropic chat", "anthropic", "https://api.anthropic.com/v1", "claude-sonnet-4-5", EndpointChat, false, "https://api.anthropic.com/v1/messages"},
		{"google chat", "google-ai-studio", "https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-pro", EndpointChat, false, "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:generateContent"},
		{"google stream", "google-ai-studio", "https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-pro", EndpointChat, true, "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse"},
		{"azure chat", "azure-openai", "https://myres.openai.azure.com", "my-deployment", EndpointChat, false, "https://myres.openai.azure.com/openai/deployments/my-deployment/chat/completions?api-version=" + azureAPIVersion},
		{"azure embeddings", "azure-openai", "https://myres.openai.azure.com/", "embed-dep", EndpointEmbeddings, false, "https://myres.openai.azure.com/openai/deployments/embed-dep/embeddings?api-version=" + azureAPIVersion},
		{"cohere chat", "cohere", "https://api.cohere.com/v2", "command-a-03-2025", EndpointChat, false, "https://api.cohere.com/v2/chat"},
		{"cohere rerank", "cohere", "https://api.cohere.com/v2", "rerank-v3.5", EndpointRerank, false, "https://api.cohere.com/v2/rerank"},
		{"cohere embed", "cohere", "https://api.cohere.com/v2", "embed-v4.0", EndpointEmbeddings, false, "https://api.cohere.com/v2/embed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetProfile(tt.providerID)
			got := p.BuildURL(tt.base, tt.externalID, tt.endpoint, tt.stream)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetAuth(t *testing.T) {
	tests := []struct {
		providerID string
		header     string
		value      string
	}{
		{"openai", "Authorization", "Bearer tok"},
		{"groq", "Authorization", "Bearer tok"},
		{"cohere", "Authorization", "Bearer tok"},
		{"anthropic", "x-api-key", "tok"},
		{"google-ai-studio", "x-goog-api-key", "tok"},
		{"azure-openai", "api-key", "tok"},
	}
	for _, tt := range tests {
		req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
		GetProfile(tt.providerID).SetAuth(req, "tok")
		if got := req.Header.Get(tt.header); got != tt.value {
			t.Errorf("%s: header %s = %q, want %q", tt.providerID, tt.header, got, tt.value)
		}
	}
}

func TestAnthropicAuthSetsVersion(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	GetProfile("anthropic").SetAuth(req, "tok")
	if req.Header.Get("anthropic-version") == "" {
		t.Error("anthropic-version header not set")
	}
}
