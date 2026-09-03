package gateway

import (
	"net/http"
	"strings"
)

// ============================================================================
// Provider Profiles
//
// A ProviderProfile describes everything the gateway needs to talk to an
// upstream provider: default base URL, auth header style, request/response
// translator, and endpoint URL construction. Adding a new provider means
// adding one entry to the registry below instead of editing switches across
// the router, upstream, and translator.
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
	AuthBearer    AuthStyle = iota // Authorization: Bearer <token>
	AuthAnthropic                  // x-api-key + anthropic-version
	AuthGoogleKey                  // x-goog-api-key
	AuthAzureKey                   // api-key
)

// azureAPIVersion is the API version appended to Azure OpenAI requests.
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

// openAIStylePath returns the standard OpenAI-compatible path for an endpoint.
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

// azureURL builds Azure OpenAI URLs: the model's external ID is the Azure
// deployment name and every call carries an api-version query parameter.
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
		return b + openAIStylePath(endpoint)
	}
}

// openAICompatProfile builds a profile for a provider that speaks the OpenAI
// API with bearer auth — only the base URL differs.
func openAICompatProfile(id, baseURL string) ProviderProfile {
	return ProviderProfile{
		ID:             id,
		DefaultBaseURL: baseURL,
		Auth:           AuthBearer,
		Translator:     &OpenAITranslator{},
		BuildURL:       openAIStyleURL,
	}
}

var providerProfiles = map[string]ProviderProfile{
	"openai":     openAICompatProfile("openai", "https://api.openai.com/v1"),
	"mistral":    openAICompatProfile("mistral", "https://api.mistral.ai/v1"),
	"groq":       openAICompatProfile("groq", "https://api.groq.com/openai/v1"),
	"together":   openAICompatProfile("together", "https://api.together.xyz/v1"),
	"deepseek":   openAICompatProfile("deepseek", "https://api.deepseek.com/v1"),
	"xai":        openAICompatProfile("xai", "https://api.x.ai/v1"),
	"fireworks":  openAICompatProfile("fireworks", "https://api.fireworks.ai/inference/v1"),
	"perplexity": openAICompatProfile("perplexity", "https://api.perplexity.ai"),
	"openrouter": openAICompatProfile("openrouter", "https://openrouter.ai/api/v1"),
	"cerebras":   openAICompatProfile("cerebras", "https://api.cerebras.ai/v1"),
	"deepinfra":  openAICompatProfile("deepinfra", "https://api.deepinfra.com/v1/openai"),

	"anthropic": {
		ID:             "anthropic",
		DefaultBaseURL: "https://api.anthropic.com/v1",
		Auth:           AuthAnthropic,
		Translator:     &AnthropicTranslator{},
		BuildURL:       anthropicURL,
	},
	"google": {
		ID:             "google",
		DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Auth:           AuthGoogleKey,
		Translator:     &GoogleTranslator{},
		BuildURL:       googleURL,
	},
	"google-ai-studio": {
		ID:             "google-ai-studio",
		DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Auth:           AuthGoogleKey,
		Translator:     &GoogleTranslator{},
		BuildURL:       googleURL,
	},
	"google-vertex": {
		ID:             "google-vertex",
		DefaultBaseURL: "",
		Auth:           AuthGoogleKey,
		Translator:     &GoogleTranslator{},
		BuildURL:       googleURL,
	},
	// Azure OpenAI has no default base URL: each key must carry its resource
	// endpoint (https://<resource>.openai.azure.com) via provider_keys.base_url.
	"azure-openai": {
		ID:             "azure-openai",
		DefaultBaseURL: "",
		Auth:           AuthAzureKey,
		Translator:     &OpenAITranslator{},
		BuildURL:       azureURL,
	},
	"cohere": {
		ID:             "cohere",
		DefaultBaseURL: "https://api.cohere.com/v2",
		Auth:           AuthBearer,
		Translator:     &CohereTranslator{},
		BuildURL:       cohereURL,
	},
}

// defaultProfile is used for unknown providers: OpenAI-compatible with bearer
// auth and no default base URL (must come from the provider key).
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
	case AuthGoogleKey:
		req.Header.Set("x-goog-api-key", token)
	case AuthAzureKey:
		req.Header.Set("api-key", token)
	default:
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
