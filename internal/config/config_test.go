package config

import (
	"strings"
	"testing"
)

func validIAMKit() IAMKit {
	return IAMKit{
		BaseURL:       "http://localhost:8080",
		EnvironmentID: "env",
		ApplicationID: "app",
		ResourceID:    "res",
		JWTIssuer:     "http://localhost:8080",
		Audience:      "https://freerouter.local",
	}
}

func TestIAMKitValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*IAMKit)
		wantErr string
	}{
		{"valid without backend account", func(*IAMKit) {}, ""},
		{"valid with backend account", func(c *IAMKit) { c.ServiceSecret = "ik_svc_x"; c.OrganizationID = "org" }, ""},
		{"missing audience", func(c *IAMKit) { c.Audience = "" }, "IAMKIT_AUDIENCE"},
		{"missing several", func(c *IAMKit) { c.EnvironmentID = ""; c.ResourceID = " " }, "IAMKIT_ENVIRONMENT_ID, IAMKIT_RESOURCE_ID"},
		{"management key rejected", func(c *IAMKit) { c.ServiceSecret = "ik_mgmt_x"; c.OrganizationID = "org" }, "ik_svc_"},
		{"backend account needs organization", func(c *IAMKit) { c.ServiceSecret = "ik_svc_x" }, "IAMKIT_ORGANIZATION_ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validIAMKit()
			tc.mutate(&c)
			err := c.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_WebhookAllowPrivateOnlyInTestEnv(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE", "true")

	t.Setenv("APP_ENV", "production")
	if Load().Webhook.AllowPrivate {
		t.Fatal("WEBHOOK_ALLOW_PRIVATE must be ignored outside APP_ENV=test")
	}
	t.Setenv("APP_ENV", "development")
	if Load().Webhook.AllowPrivate {
		t.Fatal("WEBHOOK_ALLOW_PRIVATE must be ignored outside APP_ENV=test")
	}
	t.Setenv("APP_ENV", "test")
	if !Load().Webhook.AllowPrivate {
		t.Fatal("WEBHOOK_ALLOW_PRIVATE should be honored when APP_ENV=test")
	}
}

func TestConfigValidate_AuthModes(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"none on loopback", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "127.0.0.1"}}, ""},
		{"none on ::1", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "::1"}}, ""},
		{"none on [::1]", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "[::1]"}}, ""},
		{"none on localhost", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "localhost"}}, ""},
		{"none on all interfaces", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: ""}}, "loopback"},
		{"none on 0.0.0.0", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "0.0.0.0"}}, "loopback"},
		{"none ignores IAMKit", Config{Auth: Auth{Mode: AuthNone}, Server: Server{Host: "127.0.0.1"}, IAMKit: IAMKit{ServiceSecret: "ik_mgmt_x"}}, ""},
		{"token with token", Config{Auth: Auth{Mode: AuthToken, AdminToken: strings.Repeat("a", 32)}}, ""},
		{"token without token", Config{Auth: Auth{Mode: AuthToken}}, "FREEROUTER_ADMIN_TOKEN"},
		{"token too short", Config{Auth: Auth{Mode: AuthToken, AdminToken: strings.Repeat("a", 31)}}, "at least 32"},
		{"token ignores IAMKit", Config{Auth: Auth{Mode: AuthToken, AdminToken: strings.Repeat("a", 32)}}, ""},
		{"iam valid", Config{Auth: Auth{Mode: AuthIAM}, IAMKit: validIAMKit()}, ""},
		{"iam needs IAMKit", Config{Auth: Auth{Mode: AuthIAM}}, "IAMKIT_ENVIRONMENT_ID"},
		{"unknown mode", Config{Auth: Auth{Mode: "open"}}, "none, token or iam"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_AuthModeDefaults(t *testing.T) {
	t.Setenv("FREEROUTER_AUTH", "")
	t.Setenv("SERVER_HOST", "")
	t.Setenv("IAMKIT_ENVIRONMENT_ID", "")
	if got := Load().Auth.Mode; got != AuthToken {
		t.Fatalf("unset FREEROUTER_AUTH without IAMKit = %q, want token", got)
	}
	t.Setenv("IAMKIT_ENVIRONMENT_ID", "env")
	if got := Load().Auth.Mode; got != AuthIAM {
		t.Fatalf("unset FREEROUTER_AUTH with IAMKit configured = %q, want iam (existing deployments)", got)
	}
	t.Setenv("FREEROUTER_AUTH", "none")
	cfg := Load()
	if cfg.Auth.Mode != AuthNone || cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("none mode = %q on %q, want none on 127.0.0.1", cfg.Auth.Mode, cfg.Server.Host)
	}
	if cfg.Validate() != nil {
		t.Fatalf("none mode defaults must validate: %v", cfg.Validate())
	}
}

func TestServerAddrAndNetwork(t *testing.T) {
	cases := []struct{ host, addr, network string }{
		{"", ":3000", "tcp4"},
		{"127.0.0.1", "127.0.0.1:3000", "tcp4"},
		{"localhost", "localhost:3000", "tcp4"},
		{"::1", "[::1]:3000", "tcp6"},
		{"[::1]", "[::1]:3000", "tcp6"},
	}
	for _, tc := range cases {
		s := Server{Host: tc.host, Port: "3000"}
		if s.Addr() != tc.addr || s.Network() != tc.network {
			t.Errorf("host %q: Addr=%q Network=%q, want %q %q", tc.host, s.Addr(), s.Network(), tc.addr, tc.network)
		}
	}
}

func TestLoad_TrimsAdminToken(t *testing.T) {
	t.Setenv("FREEROUTER_AUTH", "token")
	t.Setenv("FREEROUTER_ADMIN_TOKEN", "  "+strings.Repeat("a", 32)+"\n")
	if got := Load().Auth.AdminToken; got != strings.Repeat("a", 32) {
		t.Fatalf("AdminToken = %q, want trimmed", got)
	}
}
