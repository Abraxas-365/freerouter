package gateway

import (
	"net/http"
	"strings"
)

// ============================================================================
// Provider Profiles
// ============================================================================

// Endpoint identifies an upstream API surface.
type Endpoint string

const (
	EndpointChat          Endpoint = "chat"
	EndpointEmbeddings    Endpoint = "embeddings"
	EndpointImages        Endpoint = "images"
	EndpointTranscription Endpoint = "transcription"
	EndpointSpeech        Endpoint = "speech"
	EndpointModeration    Endpoint = "moderation"
	EndpointRerank        Endpoint = "rerank"
)

// AuthStyle defines how credentials are attached to upstream requests.
type AuthStyle int

const (
	AuthBearer         AuthStyle = iota // Authorization: Bearer <token>
	AuthAnthropic                       // x-api-key + anthropic-version
	AuthAnthropicOAuth                  // Bearer + anthropic-version + Claude Code attribution
	AuthGoogleKey                       // x-goog-api-key
	AuthAzureKey                        // api-key
)

const azureAPIVersion = "2024-10-21"

// URLBuilder constructs the full upstream URL for an endpoint.
type URLBuilder func(base, externalID string, endpoint Endpoint, stream bool) string

// ProviderProfile describes how to call one upstream provider.
type ProviderProfile struct {
	ID             string
	DefaultBaseURL string
	Auth           AuthStyle
	Translator     ProviderTranslator
	BuildURL       URLBuilder
}

// ── URL builders ─────────────────────────────────────────────────────

func openAIStylePath(endpoint Endpoint) string {
	switch endpoint {
	case EndpointChat:
		return "/chat/completions"
	case EndpointEmbeddings:
		return "/embeddings"
	case EndpointImages:
		return "/images/generations"
	case EndpointTranscription:
		return "/audio/transcriptions"
	case EndpointSpeech:
		return "/audio/speech"
	case EndpointModeration:
		return "/moderations"
	case EndpointRerank:
		return "/rerank"
	default:
		return "/chat/completions"
	}
}

func openAIStyleURL(base, _ string, endpoint Endpoint, _ bool) string {
	return strings.TrimSuffix(base, "/") + openAIStylePath(endpoint)
}

func anthropicURL(base, _ string, _ Endpoint, _ bool) string {
	return strings.TrimSuffix(base, "/") + "/messages"
}

func googleURL(base, externalID string, _ Endpoint, stream bool) string {
	b := strings.TrimSuffix(base, "/")
	if stream {
		return b + "/models/" + externalID + ":streamGenerateContent?alt=sse"
	}
	return b + "/models/" + externalID + ":generateContent"
}

func azureURL(base, externalID string, endpoint Endpoint, _ bool) string {
	b := strings.TrimSuffix(base, "/")
	return b + "/openai/deployments/" + externalID + openAIStylePath(endpoint) + "?api-version=" + azureAPIVersion
}

func cohereURL(base, _ string, endpoint Endpoint, _ bool) string {
	b := strings.TrimSuffix(base, "/")
	switch endpoint {
	case EndpointChat:
		return b + "/chat"
	case EndpointEmbeddings:
		return b + "/embed"
	case EndpointRerank:
		return b + "/rerank"
	default:
		return b + "/chat"
	}
}

func codexURL(base, _ string, _ Endpoint, _ bool) string {
	return strings.TrimSuffix(base, "/") + "/codex/responses"
}

// ── Profile registry ─────────────────────────────────────────────────
// Keys match provider.Protocol values exactly.

var providerProfiles = map[string]ProviderProfile{
	"openai": {
		ID: "openai", DefaultBaseURL: "https://api.openai.com/v1",
		Auth: AuthBearer, Translator: &OpenAITranslator{}, BuildURL: openAIStyleURL,
	},
	"anthropic": {
		ID: "anthropic", DefaultBaseURL: "https://api.anthropic.com/v1",
		Auth: AuthAnthropic, Translator: &AnthropicTranslator{}, BuildURL: anthropicURL,
	},
	"claude-code": {
		ID: "claude-code", DefaultBaseURL: "https://api.anthropic.com/v1",
		Auth: AuthAnthropicOAuth, Translator: &AnthropicOAuthTranslator{}, BuildURL: anthropicURL,
	},
	"google": {
		ID: "google", DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Auth: AuthGoogleKey, Translator: &GoogleTranslator{}, BuildURL: googleURL,
	},
	"azure": {
		ID: "azure", DefaultBaseURL: "",
		Auth: AuthAzureKey, Translator: &OpenAITranslator{}, BuildURL: azureURL,
	},
	"cohere": {
		ID: "cohere", DefaultBaseURL: "https://api.cohere.com/v2",
		Auth: AuthBearer, Translator: &CohereTranslator{}, BuildURL: cohereURL,
	},
	"codex": {
		ID: "codex", DefaultBaseURL: "https://chatgpt.com/backend-api",
		Auth: AuthBearer, Translator: &OpenAITranslator{}, BuildURL: codexURL,
	},
}

var defaultProfile = ProviderProfile{
	Auth:       AuthBearer,
	Translator: &OpenAITranslator{},
	BuildURL:   openAIStyleURL,
}

// GetProfile returns the profile for a provider, falling back to an
// OpenAI-compatible default for unknown providers.
func GetProfile(providerID string) ProviderProfile {
	if p, ok := providerProfiles[providerID]; ok {
		return p
	}
	p := defaultProfile
	p.ID = providerID
	return p
}

// SetAuth attaches the provider's auth headers to an upstream request.
func (p ProviderProfile) SetAuth(req *http.Request, token string) {
	switch p.Auth {
	case AuthAnthropic:
		req.Header.Set("x-api-key", token)
		req.Header.Set("anthropic-version", "2023-06-01")
	case AuthAnthropicOAuth:
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("anthropic-version", "2023-06-01")
		// Claude Code attribution headers — required for subscription billing.
		req.Header.Set("anthropic-beta", "claude-code-20250219,oauth-2025-04-20")
		req.Header.Set("User-Agent", "claude-cli/2.1.195 (external, sdk-cli)")
		req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
		req.Header.Set("x-app", "cli")
	case AuthGoogleKey:
		req.Header.Set("x-goog-api-key", token)
	case AuthAzureKey:
		req.Header.Set("api-key", token)
	default:
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
