package gateway

import "encoding/json"

// AnthropicOAuthTranslator wraps AnthropicTranslator, injecting the
// Claude Code billing attribution block required for subscription-based
// OAuth access. Without this header the API rejects requests.
type AnthropicOAuthTranslator struct {
	AnthropicTranslator
}

const billingSystemBlock = "x-anthropic-billing-header: cc_version=2.1.195; cc_entrypoint=cli; cch=00000;"

func (t *AnthropicOAuthTranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	// First, run the normal Anthropic transformation.
	transformed, err := t.AnthropicTranslator.TransformRequest(body, model)
	if err != nil {
		return nil, err
	}

	// Inject the billing system block as the first item in the system array.
	var raw map[string]any
	if err := json.Unmarshal(transformed, &raw); err != nil {
		return nil, err
	}

	billingBlock := map[string]any{
		"type": "text",
		"text": billingSystemBlock,
	}

	switch existing := raw["system"].(type) {
	case string:
		// system was a plain string — convert to block array with billing first.
		raw["system"] = []any{
			billingBlock,
			map[string]any{"type": "text", "text": existing},
		}
	case []any:
		// system was already a block array — prepend billing.
		raw["system"] = append([]any{billingBlock}, existing...)
	default:
		// No system prompt — just set the billing block.
		raw["system"] = []any{billingBlock}
	}

	return json.Marshal(raw)
}
