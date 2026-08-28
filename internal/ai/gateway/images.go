package gateway

import "encoding/json"

// ============================================================================
// Image Generation Request/Response (OpenAI-compatible)
// ============================================================================

// ImageRequest is the OpenAI-compatible image generation request.
// See: https://platform.openai.com/docs/api-reference/images/create
type ImageRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model"`                     // dall-e-2, dall-e-3, gpt-image-1, etc.
	N              *int   `json:"n,omitempty"`               // Number of images (1-10)
	Size           string `json:"size,omitempty"`            // 256x256, 512x512, 1024x1024, 1792x1024, 1024x1792
	Quality        string `json:"quality,omitempty"`         // standard, hd
	Style          string `json:"style,omitempty"`           // vivid, natural
	ResponseFormat string `json:"response_format,omitempty"` // url, b64_json
	User           string `json:"user,omitempty"`
}

// ImageResponse is the OpenAI-compatible image generation response.
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
	Usage   *ImageUsage `json:"usage,omitempty"`
}

// ImageData holds a single generated image.
type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageUsage holds usage info for image generation (returned by some models).
type ImageUsage struct {
	InputTokens        int `json:"input_tokens,omitempty"`
	OutputTokens       int `json:"output_tokens,omitempty"`
	TotalTokens        int `json:"total_tokens,omitempty"`
	InputTokensDetails any `json:"input_tokens_details,omitempty"`
}

// ImagePricing holds per-image fixed pricing by size/quality.
// Image models charge per image, not per token.
type ImagePricing struct {
	PerImage float64 // USD per image generated
}

// DefaultImagePricing returns the per-image price for a given model, size, and quality.
// Prices from https://openai.com/api/pricing/
func DefaultImagePricing(model, size, quality string) float64 {
	switch model {
	case "dall-e-3":
		if quality == "hd" {
			switch size {
			case "1024x1024":
				return 0.080
			case "1024x1792", "1792x1024":
				return 0.120
			default:
				return 0.080
			}
		}
		switch size {
		case "1024x1024":
			return 0.040
		case "1024x1792", "1792x1024":
			return 0.080
		default:
			return 0.040
		}
	case "dall-e-2":
		switch size {
		case "256x256":
			return 0.016
		case "512x512":
			return 0.018
		case "1024x1024":
			return 0.020
		default:
			return 0.020
		}
	case "gpt-image-1":
		if quality == "hd" {
			switch size {
			case "1024x1024":
				return 0.167
			case "1024x1536", "1536x1024":
				return 0.250
			default:
				return 0.167
			}
		}
		switch size {
		case "1024x1024":
			return 0.040
		case "1024x1536", "1536x1024":
			return 0.060
		default:
			return 0.040
		}
	default:
		return 0.040 // fallback
	}
}

// ToJSON marshals the image response to JSON.
func (r *ImageResponse) ToJSON() (json.RawMessage, error) {
	return json.Marshal(r)
}
