package e2e

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/iam/apikey/apikeyinfra"
	"github.com/Abraxas-365/freerouter/internal/iam/otp"
	"github.com/Abraxas-365/freerouter/internal/iam/otp/otpinfra"
	"github.com/Abraxas-365/freerouter/internal/iam/otp/otpsrv"
	"github.com/Abraxas-365/freerouter/internal/iam/role"
	"github.com/Abraxas-365/freerouter/internal/iam/role/roleinfra"
	"github.com/Abraxas-365/freerouter/internal/iam/user/userinfra"
	"github.com/google/uuid"
)

func TestConcurrentIAMMutations(t *testing.T) {
	s := NewSuite(t)
	ctx := context.Background()
	users := userinfra.NewPostgresUserRepository(s.DB)
	old, err := users.FindByID(ctx, s.UserID, s.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	current := *old
	current.Scopes = []string{"gateway:chat"}
	if err := users.Save(ctx, current); err != nil {
		t.Fatal(err)
	}
	old.Name = "stale metadata"
	if err := users.Save(ctx, *old); err == nil {
		t.Fatal("stale user restored revoked scopes")
	}
	keys := apikeyinfra.NewPostgresAPIKeyRepository(s.DB)
	all, err := keys.FindByTenant(ctx, s.TenantID)
	if err != nil || len(all) == 0 {
		t.Fatal("missing fixture key", err)
	}
	stale := *all[0]
	if err := keys.UpdateLastUsed(ctx, stale.ID); err != nil {
		t.Fatal(err)
	}
	fresh := stale
	fresh.Scopes = []string{"gateway:chat"}
	if err := keys.Save(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if err := keys.Save(ctx, stale); err == nil {
		t.Fatal("stale key restored revoked scopes")
	}
	if err := keys.Revoke(ctx, stale.ID, s.TenantID); err != nil {
		t.Fatal(err)
	}
	if err := keys.Delete(ctx, stale.ID, s.TenantID); err != nil {
		t.Fatal(err)
	}
	if err := keys.Save(ctx, stale); err == nil {
		t.Fatal("deleted key resurrected")
	}
	roles := roleinfra.NewPostgresRoleRepository(s.DB)
	r := role.Role{ID: uuid.NewString(), TenantID: s.TenantID, Name: "race-role", Scopes: []string{"gateway:chat"}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := roles.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	snapshot, err := roles.FindByID(ctx, r.ID, s.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	newer := *snapshot
	newer.Scopes = []string{"*"}
	if err := roles.Save(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if err := roles.AssignToUser(ctx, role.UserRole{UserID: s.UserID, RoleID: r.ID, TenantID: s.TenantID, AssignedAt: time.Now().UTC()}, snapshot.Version); err == nil {
		t.Fatal("changed role assigned using stale authorization")
	}
	// Exactly one parallel verifier may consume a challenge.
	repo := otpinfra.NewPostgresOTPRepository(s.DB)
	challenge := &otp.OTP{ID: uuid.NewString(), Contact: "race@example.com", Code: "123456", Purpose: otp.OTPPurposeLogin, MaxAttempts: 5, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatal(err)
	}
	service := otpsrv.NewOTPService(repo, nil, nil)
	var successes atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := service.VerifyOTP(ctx, challenge.Contact, challenge.Code, challenge.Purpose); err == nil {
				successes.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("OTP consumed %d times", successes.Load())
	}
	challenge.Attempts = 1
	if err := repo.Update(ctx, challenge); err == nil {
		t.Fatal("stale guess reopened verified OTP")
	}
}

func TestParallelWrongOTPAttempts(t *testing.T) {
	s := NewSuite(t)
	ctx := context.Background()
	repo := otpinfra.NewPostgresOTPRepository(s.DB)
	o := &otp.OTP{ID: uuid.NewString(), Contact: "wrong@example.com", Code: "123456", Purpose: otp.OTPPurposeLogin, MaxAttempts: 5, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); snapshot := *o; snapshot.Attempts = 1; _ = repo.Update(ctx, &snapshot) }()
	}
	wg.Wait()
	final, err := repo.GetLatestByContact(ctx, o.Contact, o.Purpose)
	if err != nil {
		t.Fatal(err)
	}
	if final.Attempts != 5 {
		t.Fatalf("lost parallel guesses: got %d attempts", final.Attempts)
	}
	if _, err := otpsrv.NewOTPService(repo, nil, nil).VerifyOTP(ctx, o.Contact, o.Code, o.Purpose); err == nil {
		t.Fatal("exhausted challenge accepted")
	}
}
