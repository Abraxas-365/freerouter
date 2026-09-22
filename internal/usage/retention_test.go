package usage

import "testing"

func TestDefaultRetentionConfig(t *testing.T) {
	cfg := DefaultRetentionConfig()

	if cfg.RetentionDays != DefaultRetentionDays {
		t.Errorf("expected retention_days %d, got %d", DefaultRetentionDays, cfg.RetentionDays)
	}
	if !cfg.RetainMessages {
		t.Error("expected retain_messages true by default")
	}
	if !cfg.RetainResponseBody {
		t.Error("expected retain_response_body true by default")
	}
}

func TestUpsertRetention_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     UpsertRetention
		wantErr bool
	}{
		{
			name: "valid positive retention days",
			cmd:  UpsertRetention{RetentionDays: intPtr(30)},
		},
		{
			name: "zero retention days means retain forever, valid",
			cmd:  UpsertRetention{RetentionDays: intPtr(0)},
		},
		{
			name:    "negative retention days is invalid",
			cmd:     UpsertRetention{RetentionDays: intPtr(-1)},
			wantErr: true,
		},
		{
			name: "nil retention days is valid (no change)",
			cmd:  UpsertRetention{RetainMessages: boolPtr(false)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }
