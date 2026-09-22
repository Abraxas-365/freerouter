package providerkeyinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/providerkey"
)

// ── Anthropic OAuth Refresher ───────────────────────────────────────

const (
	anthropicTokenURL = "https://platform.claude.com/v1/oauth/token"
	anthropicClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

// AnthropicRefresher refreshes Anthropic OAuth tokens.
type AnthropicRefresher struct {
	client   *http.Client
	tokenURL string
	clientID string
}

var _ providerkey.TokenRefresher = (*AnthropicRefresher)(nil)

// NewAnthropicRefresher creates a refresher for Anthropic OAuth tokens.
func NewAnthropicRefresher() *AnthropicRefresher {
	return &AnthropicRefresher{
		client:   &http.Client{Timeout: 30 * time.Second},
		tokenURL: anthropicTokenURL,
		clientID: anthropicClientID,
	}
}

func (r *AnthropicRefresher) Protocol() string { return "claude-code" }

func (r *AnthropicRefresher) Refresh(ctx context.Context, refreshToken string) (*providerkey.OAuthData, error) {
	// Anthropic uses JSON-encoded token requests.
	body := fmt.Sprintf(`{"grant_type":"refresh_token","refresh_token":%q,"client_id":%q}`,
		refreshToken, r.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL,
		strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint http %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // default 1 hour
	}

	return &providerkey.OAuthData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

// ── OpenAI/ChatGPT OAuth Refresher ─────────────────────────────────

const (
	openaiTokenURL = "https://auth.openai.com/oauth/token"
	openaiClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

// OpenAIRefresher refreshes OpenAI/ChatGPT OAuth tokens.
type OpenAIRefresher struct {
	client   *http.Client
	tokenURL string
	clientID string
}

var _ providerkey.TokenRefresher = (*OpenAIRefresher)(nil)

// NewOpenAIRefresher creates a refresher for OpenAI OAuth tokens.
func NewOpenAIRefresher() *OpenAIRefresher {
	return &OpenAIRefresher{
		client:   &http.Client{Timeout: 30 * time.Second},
		tokenURL: openaiTokenURL,
		clientID: openaiClientID,
	}
}

func (r *OpenAIRefresher) Protocol() string { return "codex" }

func (r *OpenAIRefresher) Refresh(ctx context.Context, refreshToken string) (*providerkey.OAuthData, error) {
	// OpenAI uses form-encoded token requests.
	form := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s",
		refreshToken, r.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL,
		strings.NewReader(form))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint http %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // default 1 hour
	}

	return &providerkey.OAuthData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}
