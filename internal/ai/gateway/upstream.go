package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
)

// Upstream handles HTTP calls to upstream LLM providers
type Upstream struct {
	client *http.Client
}

func NewUpstream() *Upstream {
	return &Upstream{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// StreamCallback is called for each SSE chunk received from the upstream provider
type StreamCallback func(chunk []byte) error

// Call makes a non-streaming request to the upstream provider
func (u *Upstream) Call(ctx context.Context, route *RouteResult, body []byte) (*ChatResponse, int, error) {
	translator := GetTranslator(route.ProviderID.String())

	// Transform request to provider-native format
	providerBody, err := translator.TransformRequest(body, route.ExternalID)
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to transform request", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}

	req, err := u.buildRequest(ctx, route, providerBody, false)
	if err != nil {
		return nil, 0, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID).
			WithDetail("status", fmt.Sprintf("%d", resp.StatusCode))
	}

	// Transform response from provider-native to OpenAI format
	chatResp, err := translator.TransformResponse(respBody)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to transform upstream response", errx.TypeInternal)
	}

	return chatResp, resp.StatusCode, nil
}

// Stream makes a streaming request and calls the callback for each SSE data line.
// Returns the upstream HTTP status code (0 if the request never reached the provider).
func (u *Upstream) Stream(ctx context.Context, route *RouteResult, body []byte, onChunk StreamCallback) (int, error) {
	translator := GetTranslator(route.ProviderID.String())

	// Transform request to provider-native format
	providerBody, err := translator.TransformRequest(body, route.ExternalID)
	if err != nil {
		return 0, errx.Wrap(err, "failed to transform request", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}

	req, err := u.buildRequest(ctx, route, providerBody, true)
	if err != nil {
		return 0, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return 0, errx.Wrap(err, "upstream streaming request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID).
			WithDetail("status", fmt.Sprintf("%d", resp.StatusCode))
	}

	// Read SSE stream line by line
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // Up to 1MB lines

	for scanner.Scan() {
		line := scanner.Bytes()

		// Skip empty lines (SSE event separator)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		// Parse SSE data lines
		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))

			// Transform the event through the translator
			transformed, done, err := translator.TransformStreamEvent(data)
			if err != nil {
				return http.StatusOK, errx.Wrap(err, "failed to transform stream event", errx.TypeInternal)
			}

			if done {
				break
			}

			// Skip events that produce no output (e.g. ping, content_block_stop)
			if transformed == nil {
				continue
			}

			if err := onChunk(transformed); err != nil {
				return http.StatusOK, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return http.StatusOK, errx.Wrap(err, "error reading upstream stream", errx.TypeInternal)
	}

	return http.StatusOK, nil
}

// CallImage makes a non-streaming request to the upstream provider's image
// generation endpoint. Unlike chat, image requests are proxied directly
// without translation (OpenAI-compatible format only).
func (u *Upstream) CallImage(ctx context.Context, route *RouteResult, body []byte) (*ImageResponse, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointImages, false)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build image request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream image request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream image response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID).
			WithDetail("status", fmt.Sprintf("%d", resp.StatusCode))
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(respBody, &imageResp); err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to parse image response", errx.TypeInternal)
	}

	return &imageResp, resp.StatusCode, nil
}

// CallEmbedding makes a non-streaming request to the upstream provider's
// embeddings endpoint. Proxied directly in OpenAI-compatible format.
func (u *Upstream) CallEmbedding(ctx context.Context, route *RouteResult, body []byte) (*EmbeddingResponse, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointEmbeddings, false)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build embedding request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream embedding request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream embedding response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID).
			WithDetail("status", fmt.Sprintf("%d", resp.StatusCode))
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to parse embedding response", errx.TypeInternal)
	}

	return &embResp, resp.StatusCode, nil
}

func (u *Upstream) buildRequest(ctx context.Context, route *RouteResult, body []byte, stream bool) (*http.Request, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointChat, stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errx.Wrap(err, "failed to build upstream request", errx.TypeInternal)
	}

	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	return req, nil
}

// CallTranscription forwards a multipart transcription request (STT).
// The multipart body is streamed as-is with the model field already rewritten.
func (u *Upstream) CallTranscription(ctx context.Context, route *RouteResult, body []byte, contentType string) ([]byte, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointTranscription, false)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build transcription request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", contentType)
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream transcription request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read transcription response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID)
	}

	return respBody, resp.StatusCode, nil
}

// CallSpeech forwards a TTS request and returns the raw audio bytes and the
// upstream Content-Type.
func (u *Upstream) CallSpeech(ctx context.Context, route *RouteResult, body []byte) ([]byte, string, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointSpeech, false)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, "", 0, errx.Wrap(err, "failed to build speech request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, "", 0, errx.Wrap(err, "upstream speech request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", resp.StatusCode, errx.Wrap(err, "failed to read speech response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID)
	}

	return respBody, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

// CallModeration forwards a moderation request (OpenAI-compatible).
func (u *Upstream) CallModeration(ctx context.Context, route *RouteResult, body []byte) (*ModerationResponse, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointModeration, false)

	respBody, status, err := u.postJSON(ctx, profile, route, url, body, "moderation")
	if err != nil {
		return nil, status, err
	}

	var modResp ModerationResponse
	if err := json.Unmarshal(respBody, &modResp); err != nil {
		return nil, status, errx.Wrap(err, "failed to parse moderation response", errx.TypeInternal)
	}
	return &modResp, status, nil
}

// CallRerank forwards a rerank request (Cohere v2-compatible).
func (u *Upstream) CallRerank(ctx context.Context, route *RouteResult, body []byte) (*RerankResponse, int, error) {
	profile := GetProfile(route.ProviderID.String())
	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointRerank, false)

	respBody, status, err := u.postJSON(ctx, profile, route, url, body, "rerank")
	if err != nil {
		return nil, status, err
	}

	var rrResp RerankResponse
	if err := json.Unmarshal(respBody, &rrResp); err != nil {
		return nil, status, errx.Wrap(err, "failed to parse rerank response", errx.TypeInternal)
	}
	return &rrResp, status, nil
}

// postJSON performs a JSON POST to the upstream and returns the raw body.
func (u *Upstream) postJSON(ctx context.Context, profile ProviderProfile, route *RouteResult, url string, body []byte, kind string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build "+kind+" request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream "+kind+" request failed", errx.TypeInternal).
			WithDetail("provider", route.ProviderID)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read "+kind+" response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeInternal,
		).WithDetail("provider", route.ProviderID)
	}

	return respBody, resp.StatusCode, nil
}
