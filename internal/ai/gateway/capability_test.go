package gateway

import (
	"testing"

	"github.com/Abraxas-365/freerouter/internal/ai/provider"
)

func TestHasCapability(t *testing.T) {
	m := &provider.ModelProviderMapping{Audio: true, Rerank: true}

	tests := []struct {
		cap  Capability
		want bool
	}{
		{CapabilityAudio, true},
		{CapabilityRerank, true},
		{CapabilitySpeech, false},
		{CapabilityModeration, false},
		{Capability(""), true},        // empty = no restriction
		{Capability("unknown"), true}, // unknown capability doesn't block
	}
	for _, tt := range tests {
		if got := hasCapability(m, tt.cap); got != tt.want {
			t.Errorf("hasCapability(%q) = %v, want %v", tt.cap, got, tt.want)
		}
	}
}

func TestFilterByCapability(t *testing.T) {
	mappings := []*provider.ModelProviderMapping{
		{ExternalID: "chat-only"},
		{ExternalID: "stt", Audio: true},
		{ExternalID: "tts", Speech: true},
		{ExternalID: "stt-2", Audio: true},
	}

	// Empty capability keeps everything
	if got := filterByCapability(mappings, ""); len(got) != 4 {
		t.Errorf("empty cap: got %d mappings, want 4", len(got))
	}

	// Audio keeps only the two STT mappings
	got := filterByCapability(mappings, CapabilityAudio)
	if len(got) != 2 || got[0].ExternalID != "stt" || got[1].ExternalID != "stt-2" {
		t.Errorf("audio filter: got %+v", got)
	}

	// Speech keeps only tts
	got = filterByCapability(mappings, CapabilitySpeech)
	if len(got) != 1 || got[0].ExternalID != "tts" {
		t.Errorf("speech filter: got %+v", got)
	}

	// Rerank matches nothing
	if got := filterByCapability(mappings, CapabilityRerank); len(got) != 0 {
		t.Errorf("rerank filter: got %d mappings, want 0", len(got))
	}
}
