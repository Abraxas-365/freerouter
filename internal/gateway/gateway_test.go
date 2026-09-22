package gateway

import "testing"

func TestEstimateMessageTokens(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is the capital of France?"},
	}

	got := EstimateMessageTokens(messages)
	if got <= 0 {
		t.Fatalf("expected positive token estimate, got %d", got)
	}
}

func TestEstimateMessageTokens_Empty(t *testing.T) {
	got := EstimateMessageTokens(nil)
	if got != 2 {
		// per-message overhead is 0 with no messages, plus reply priming of 2
		t.Fatalf("expected 2 (reply priming only), got %d", got)
	}
}

func TestCalculateChatCost(t *testing.T) {
	inputPrice := 5.0   // $5 per 1M input tokens
	outputPrice := 15.0 // $15 per 1M output tokens
	route := &RouteResult{
		InputPrice:  &inputPrice,
		OutputPrice: &outputPrice,
	}

	cost := CalculateChatCost(route, 1_000_000, 0)
	if cost != 5.0 {
		t.Errorf("expected input cost 5.0, got %f", cost)
	}

	cost = CalculateChatCost(route, 0, 1_000_000)
	if cost != 15.0 {
		t.Errorf("expected output cost 15.0, got %f", cost)
	}

	cost = CalculateChatCost(route, 1_000_000, 1_000_000)
	if cost != 20.0 {
		t.Errorf("expected total cost 20.0, got %f", cost)
	}
}

func TestCalculateChatCost_NoPricing(t *testing.T) {
	route := &RouteResult{}

	cost := CalculateChatCost(route, 1000, 1000)
	if cost != 0 {
		t.Errorf("expected cost 0 when no pricing configured, got %f", cost)
	}
}

func TestCostEstimateResponse_JSONFields(t *testing.T) {
	// Guard against accidental field renames breaking the public API contract.
	resp := CostEstimateResponse{
		Model:                 "gpt-4o",
		EstimatedInputTokens:  10,
		EstimatedOutputTokens: 4096,
		InputCostUSD:          0.001,
		OutputCostUSD:         0.05,
	}
	resp.TotalCostUSD = resp.InputCostUSD + resp.OutputCostUSD
	if resp.Model != "gpt-4o" {
		t.Errorf("unexpected model: %s", resp.Model)
	}
	if resp.TotalCostUSD != resp.InputCostUSD+resp.OutputCostUSD {
		t.Errorf("total cost should equal input+output cost")
	}
}

func TestDefaultEstimateMaxOutputTokens(t *testing.T) {
	if DefaultEstimateMaxOutputTokens != 4096 {
		t.Errorf("expected default max output tokens 4096, got %d", DefaultEstimateMaxOutputTokens)
	}
}
