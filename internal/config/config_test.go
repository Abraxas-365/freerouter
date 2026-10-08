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
