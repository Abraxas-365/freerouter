package gateway

import (
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{"empty", "", 0, 0},
		{"short", "hi", 1, 1},
		{"medium", "Hello, how are you doing today?", 5, 15},
		{"long", "The quick brown fox jumps over the lazy dog. This is a longer sentence with more words to test the estimation.", 20, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := EstimateTokens(tt.text)
			if tokens < tt.min || tokens > tt.max {
				t.Errorf("EstimateTokens(%q) = %d, expected between %d and %d", tt.text, tokens, tt.min, tt.max)
			}
		})
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is 2+2?"},
	}

	tokens := EstimateMessageTokens(messages)

	// System: ~7 chars/4 = ~7 tokens + 4 overhead = ~11
	// User: ~12 chars/4 = ~3 tokens + 4 overhead = ~7
	// + 2 reply priming = ~20
	if tokens < 10 {
		t.Errorf("expected at least 10 tokens, got %d", tokens)
	}
	if tokens > 50 {
		t.Errorf("expected at most 50 tokens, got %d", tokens)
	}
}

func TestEstimateMessageTokens_Empty(t *testing.T) {
	tokens := EstimateMessageTokens(nil)
	if tokens != 2 { // just reply priming
		t.Errorf("expected 2 tokens for empty messages, got %d", tokens)
	}
}
