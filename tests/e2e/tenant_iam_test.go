package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/iam/auth/authinfra"
	"github.com/Abraxas-365/freerouter/internal/iam/invitation"
	"github.com/Abraxas-365/freerouter/internal/iam/invitation/invitationinfra"
	"github.com/Abraxas-365/freerouter/internal/iam/user"
	"github.com/Abraxas-365/freerouter/internal/iam/user/userinfra"
	"github.com/google/uuid"
)

func TestTenantIAMLifecycle(t *testing.T) {
	s := NewSuite(t)
	ctx := context.Background()
	users := userinfra.NewPostgresUserRepository(s.DB)
	invitations := invitationinfra.NewPostgresInvitationRepository(s.DB)
	acceptor := authinfra.NewPostgresInvitationAcceptor(s.DB)
	var before int
	if err := s.DB.Get(&before, `SELECT current_users FROM tenants WHERE id=$1`, s.TenantID); err != nil {
		t.Fatal(err)
	}
	newInvitation := func(email string, roleID *string) *invitation.Invitation {
		inv := &invitation.Invitation{ID: uuid.NewString(), TenantID: s.TenantID, Email: email, Token: uuid.NewString(), Scopes: []string{"gateway:chat"}, RoleID: roleID, Status: invitation.InvitationStatusPending, InvitedBy: s.UserID, ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := invitations.Save(ctx, *inv); err != nil {
			t.Fatal(err)
		}
		return inv
	}
	inv := newInvitation("join@example.com", nil)
	candidate := user.User{Email: inv.Email, Name: "New member", TenantID: s.TenantID, EmailVerified: true, OTPEnabled: true}
	if _, _, err := acceptor.Accept(ctx, inv.Token, user.User{Email: inv.Email}); err == nil {
		t.Fatal("unverified membership accepted")
	}
	member, _, err := acceptor.Accept(ctx, inv.Token, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !member.CanLoginWithOTP() {
		t.Fatal("verified membership cannot log in")
	}
	if _, _, err := acceptor.Accept(ctx, inv.Token, candidate); err == nil {
		t.Fatal("invitation reused")
	}
	accepted, err := invitations.FindByToken(ctx, inv.Token)
	if err != nil || accepted.Status != invitation.InvitationStatusAccepted {
		t.Fatalf("acceptance missing: %v", err)
	}
	var count int
	if err := s.DB.Get(&count, `SELECT current_users FROM tenants WHERE id=$1`, s.TenantID); err != nil || count != before+1 {
		t.Fatalf("membership count=%d, want=%d: %v", count, before+1, err)
	}
	// Force role validation failure after membership writes to prove transaction rollback.
	roleID := uuid.NewString()
	_, err = s.DB.Exec(`INSERT INTO roles(id,tenant_id,name,scopes,created_at,updated_at) VALUES ($1,$2,'retired-role',ARRAY['platform:admin'],NOW(),NOW())`, roleID, s.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	rejected := newInvitation("rollback@example.com", &roleID)
	candidate.Email = rejected.Email
	if _, _, err := acceptor.Accept(ctx, rejected.Token, candidate); err == nil {
		t.Fatal("retired platform role accepted")
	}
	if _, err := users.FindByEmail(ctx, rejected.Email, s.TenantID); err == nil {
		t.Fatal("failed role left membership behind")
	}
	rejected, err = invitations.FindByToken(ctx, rejected.Token)
	if err != nil || rejected.Status != invitation.InvitationStatusPending {
		t.Fatalf("failed transaction consumed invitation: %v", err)
	}
	if err := s.DB.Get(&count, `SELECT current_users FROM tenants WHERE id=$1`, s.TenantID); err != nil || count != before+1 {
		t.Fatal("failed acceptance changed capacity")
	}
	// Suspension invalidates old JWT credentials permanently, including reinstatement.
	if err := s.IAM.UserService.SuspendUser(ctx, s.UserID, s.TenantID, "security test"); err != nil {
		t.Fatal(err)
	}
	if err := s.IAM.UserService.ReinstateUser(ctx, s.UserID, s.TenantID); err != nil {
		t.Fatal(err)
	}
	resp, _ := s.Do(s.Request("GET", "/api/v1/billing/balance", nil))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old credentials survived reinstatement: %d", resp.StatusCode)
	}
	current, err := users.FindByID(ctx, s.UserID, s.TenantID)
	if err != nil || current.CredentialVersion != 1 {
		t.Fatalf("generation not advanced: %+v %v", current, err)
	}
	// Membership deletion decrements capacity in the same statement.
	if err := users.Delete(ctx, member.ID, s.TenantID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Get(&count, `SELECT current_users FROM tenants WHERE id=$1`, s.TenantID); err != nil || count != before {
		t.Fatal("deletion did not release tenant capacity")
	}
}
