package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
)

// Upstream handles HTTP calls to upstream LLM providers.
type Upstream struct {
	client *http.Client
}

// NewUpstream creates an upstream client with a 5-minute timeout.
func NewUpstream() *Upstream {
	return &Upstream{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// StreamCallback is called for each SSE chunk received from the upstream provider.
type StreamCallback func(chunk []byte) error

// Call makes a non-streaming request to the upstream provider.
func (u *Upstream) Call(ctx context.Context, route *RouteResult, body []byte) (*ChatResponse, int, error) {
	translator := GetTranslator(route.ProfileSlug())

	providerBody, err := translator.TransformRequest(body, route.ExternalID)
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to transform request", errx.TypeInternal)
	}

	req, err := u.buildRequest(ctx, route, providerBody, false)
	if err != nil {
		return nil, 0, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream request failed", errx.TypeInternal)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeExternal,
		)
	}

	chatResp, err := translator.TransformResponse(respBody)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to transform upstream response", errx.TypeInternal)
	}

	return chatResp, resp.StatusCode, nil
}

// Stream makes a streaming request and calls the callback for each SSE data line.
// Returns the upstream HTTP status code.
func (u *Upstream) Stream(ctx context.Context, route *RouteResult, body []byte, onChunk StreamCallback) (int, error) {
	translator := GetTranslator(route.ProfileSlug())

	providerBody, err := translator.TransformRequest(body, route.ExternalID)
	if err != nil {
		return 0, errx.Wrap(err, "failed to transform request", errx.TypeInternal)
	}

	req, err := u.buildRequest(ctx, route, providerBody, true)
	if err != nil {
		return 0, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return 0, errx.Wrap(err, "upstream streaming request failed", errx.TypeInternal)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeExternal,
		)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "" {
			continue
		}

		transformed, done, tErr := translator.TransformStreamEvent([]byte(data))
		if tErr != nil {
			continue
		}
		if done {
			if err := onChunk([]byte("data: [DONE]\n\n")); err != nil {
				return resp.StatusCode, nil
			}
			break
		}
		if transformed == nil {
			continue
		}

		sseChunk := fmt.Sprintf("data: %s\n\n", string(transformed))
		if err := onChunk([]byte(sseChunk)); err != nil {
			return resp.StatusCode, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return resp.StatusCode, errx.Wrap(err, "error reading upstream stream", errx.TypeInternal)
	}

	return resp.StatusCode, nil
}

// CallRaw makes a non-streaming request and returns the raw response body.
// Used for passthrough endpoints (embeddings, images, speech, moderation, rerank).
func (u *Upstream) CallRaw(ctx context.Context, route *RouteResult, body []byte) ([]byte, int, error) {
	profile := GetProfile(route.ProfileSlug())

	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointChat, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream request failed", errx.TypeInternal)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeExternal,
		)
	}

	return respBody, resp.StatusCode, nil
}

// CallRawWithEndpoint is like CallRaw but accepts an explicit endpoint type.
func (u *Upstream) CallRawWithEndpoint(ctx context.Context, route *RouteResult, body []byte, endpoint Endpoint) ([]byte, int, error) {
	profile := GetProfile(route.ProfileSlug())

	url := profile.BuildURL(route.BaseURL, route.ExternalID, endpoint, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errx.Wrap(err, "failed to build request", errx.TypeInternal)
	}
	req.Header.Set("Content-Type", "application/json")
	profile.SetAuth(req, route.Token)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, errx.Wrap(err, "upstream request failed", errx.TypeInternal)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errx.Wrap(err, "failed to read upstream response", errx.TypeInternal)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errx.New(
			fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
			errx.TypeExternal,
		)
	}

	return respBody, resp.StatusCode, nil
}

func (u *Upstream) buildRequest(ctx context.Context, route *RouteResult, body []byte, stream bool) (*http.Request, error) {
	profile := GetProfile(route.ProfileSlug())

	url := profile.BuildURL(route.BaseURL, route.ExternalID, EndpointChat, stream)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errx.Wrap(err, "failed to build upstream request", errx.TypeInternal)
	}

	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	profile.SetAuth(req, route.Token)

	// Ensure stream flag is set in body
	if stream {
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err == nil {
			if _, ok := raw["stream"]; !ok {
				raw["stream"] = true
				if newBody, err := json.Marshal(raw); err == nil {
					req.Body = io.NopCloser(bytes.NewReader(newBody))
					req.ContentLength = int64(len(newBody))
				}
			}
		}
	}

	return req, nil
}
