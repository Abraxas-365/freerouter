package webhookpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/testutil"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/Abraxas-365/freerouter/internal/webhook/adapters/webhookpg"
)

// ── WebhookConfig ────────────────────────────────────────────────────

func TestRepository_CreateFindUpdateDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := webhook.WebhookConfig{
		ID:        identity.NewWebhookID(),
		URL:       "https://example.com/hook",
		Secret:    "whsec_test",
		Events:    []string{webhook.EventRequestCompleted, webhook.EventRequestFailed},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	found, err := repo.Find(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if found.URL != cfg.URL || found.Secret != cfg.Secret || !found.Enabled {
		t.Fatalf("unexpected webhook: %+v", found)
	}
	if len(found.Events) != 2 {
		t.Fatalf("expected 2 events, got %+v", found.Events)
	}

	newEvents := []string{webhook.EventKeyBlacklisted}
	updated := found
	updated.Events = newEvents
	updated.Enabled = false
	updated.UpdatedAt = now.Add(time.Minute)
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	after, err := repo.Find(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if after.Enabled {
		t.Fatalf("expected webhook disabled after update")
	}
	if len(after.Events) != 1 || after.Events[0] != webhook.EventKeyBlacklisted {
		t.Fatalf("expected updated events, got %+v", after.Events)
	}

	if err := repo.Delete(ctx, cfg.ID); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := repo.Find(ctx, cfg.ID); !isNotFound(err) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestRepository_Find_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)

	_, err := repo.Find(context.Background(), identity.NewWebhookID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_Update_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)

	cfg := webhook.WebhookConfig{ID: identity.NewWebhookID(), URL: "https://x.example.com", Events: []string{webhook.EventRequestCompleted}}
	err := repo.Update(context.Background(), cfg)
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)

	err := repo.Delete(context.Background(), identity.NewWebhookID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_FindEnabledByEvent(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	mustCreateWebhook(t, repo, webhook.WebhookConfig{
		URL: "https://a.example.com", Events: []string{webhook.EventRequestCompleted}, Enabled: true,
	})
	mustCreateWebhook(t, repo, webhook.WebhookConfig{
		URL: "https://b.example.com", Events: []string{webhook.EventRequestCompleted}, Enabled: false,
	})
	mustCreateWebhook(t, repo, webhook.WebhookConfig{
		URL: "https://c.example.com", Events: []string{webhook.EventRequestFailed}, Enabled: true,
	})

	matches, err := repo.FindEnabledByEvent(ctx, webhook.EventRequestCompleted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 || matches[0].URL != "https://a.example.com" {
		t.Fatalf("expected only the enabled+subscribed webhook, got %+v", matches)
	}
}

func TestRepository_List_Pagination(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		mustCreateWebhook(t, repo, webhook.WebhookConfig{
			URL: "https://example.com/hook", Events: []string{webhook.EventRequestCompleted}, Enabled: true,
		})
	}

	page, err := repo.List(ctx, query.Pagination{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 || page.Page.Total != 3 {
		t.Fatalf("expected 2 items of 3 total, got %d items, total %d", len(page.Items), page.Page.Total)
	}
}

func TestRepository_Create_DuplicateID_Conflicts(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	id := identity.NewWebhookID()
	cfg := webhook.WebhookConfig{ID: id, URL: "https://example.com", Events: []string{webhook.EventRequestCompleted}}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("unexpected first create error: %v", err)
	}

	err := repo.Create(ctx, cfg)
	var appErr *errx.Error
	if !errx.As(err, &appErr) || appErr.Type != errx.TypeConflict {
		t.Fatalf("expected conflict error for duplicate ID, got %v", err)
	}
}

// ── WebhookDelivery ──────────────────────────────────────────────────

func TestRepository_SaveUpdateListDeliveries(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	webhookID := mustCreateWebhook(t, repo, webhook.WebhookConfig{
		URL: "https://example.com/hook", Events: []string{webhook.EventRequestCompleted}, Enabled: true,
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	delivery := webhook.WebhookDelivery{
		ID:        identity.NewWebhookDeliveryID(),
		WebhookID: webhookID,
		EventType: webhook.EventRequestCompleted,
		Payload:   `{"hello":"world"}`,
		Status:    webhook.DeliveryPending,
		Attempts:  0,
		CreatedAt: now,
	}
	if err := repo.SaveDelivery(ctx, delivery); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	page, err := repo.ListDeliveries(ctx, webhookID, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != delivery.ID {
		t.Fatalf("unexpected deliveries: %+v", page.Items)
	}
	if page.Items[0].Status != webhook.DeliveryPending {
		t.Fatalf("expected pending status, got %s", page.Items[0].Status)
	}

	statusCode := 200
	completedAt := now.Add(time.Second)
	delivery.Status = webhook.DeliverySuccess
	delivery.StatusCode = &statusCode
	delivery.Attempts = 1
	delivery.CompletedAt = &completedAt
	if err := repo.UpdateDelivery(ctx, delivery); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	page, err = repo.ListDeliveries(ctx, webhookID, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if page.Items[0].Status != webhook.DeliverySuccess {
		t.Fatalf("expected success status after update, got %s", page.Items[0].Status)
	}
	if page.Items[0].StatusCode == nil || *page.Items[0].StatusCode != 200 {
		t.Fatalf("expected status code 200, got %+v", page.Items[0].StatusCode)
	}
}

func TestRepository_FindPendingDeliveries(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := webhookpg.New(db)
	ctx := context.Background()

	webhookID := mustCreateWebhook(t, repo, webhook.WebhookConfig{
		URL: "https://example.com/hook", Events: []string{webhook.EventRequestCompleted}, Enabled: true,
	})

	now := time.Now().UTC().Truncate(time.Microsecond)

	pending := webhook.WebhookDelivery{
		ID: identity.NewWebhookDeliveryID(), WebhookID: webhookID, EventType: webhook.EventRequestCompleted,
		Payload: `{}`, Status: webhook.DeliveryPending, CreatedAt: now,
	}
	if err := repo.SaveDelivery(ctx, pending); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	// A future-scheduled retry should NOT be returned as pending-now.
	future := now.Add(time.Hour)
	scheduled := webhook.WebhookDelivery{
		ID: identity.NewWebhookDeliveryID(), WebhookID: webhookID, EventType: webhook.EventRequestCompleted,
		Payload: `{}`, Status: webhook.DeliveryPending, NextRetryAt: &future, CreatedAt: now,
	}
	if err := repo.SaveDelivery(ctx, scheduled); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	// A completed delivery should never show up as pending.
	completed := webhook.WebhookDelivery{
		ID: identity.NewWebhookDeliveryID(), WebhookID: webhookID, EventType: webhook.EventRequestCompleted,
		Payload: `{}`, Status: webhook.DeliverySuccess, CreatedAt: now,
	}
	if err := repo.SaveDelivery(ctx, completed); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	items, err := repo.FindPendingDeliveries(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != pending.ID {
		t.Fatalf("expected only the immediately-pending delivery, got %+v", items)
	}
}

// ── helpers ──────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	var appErr *errx.Error
	return errx.As(err, &appErr) && appErr.Type == errx.TypeNotFound
}

func mustCreateWebhook(t *testing.T, repo *webhookpg.Repository, cfg webhook.WebhookConfig) identity.WebhookID {
	t.Helper()
	if cfg.ID.IsZero() {
		cfg.ID = identity.NewWebhookID()
	}
	if cfg.Secret == "" {
		cfg.Secret = "whsec_fixture"
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = now
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = now
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create webhook fixture: %v", err)
	}
	return cfg.ID
}
