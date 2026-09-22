package providerpg_test

import (
	"context"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/provider/adapters/providerpg"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/testutil"
)

// ════════════════════════════════════════════════════════════════════
// Provider Repository
// ════════════════════════════════════════════════════════════════════

func TestProviderRepo_CreateFindUpdateDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)
	ctx := context.Background()

	id := identity.NewProviderID()
	create := provider.Create{
		Name:        "OpenAI",
		Protocol:    provider.ProtocolOpenAI,
		Description: "OpenAI provider",
		Website:     "https://openai.com",
		BaseURL:     "https://api.openai.com/v1",
		Streaming:   true,
	}

	if err := repo.Create(ctx, id, create); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	found, err := repo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if found.Name != "OpenAI" || found.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected provider data: %+v", found)
	}
	if found.Status != provider.ProviderStatusActive {
		t.Fatalf("expected default status active, got %s", found.Status)
	}
	if !found.Streaming {
		t.Fatalf("expected streaming true")
	}

	newName := "OpenAI Inc."
	newStatus := provider.ProviderStatusInactive
	if err := repo.Update(ctx, id, provider.Update{Name: &newName, Status: &newStatus}); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	updated, err := repo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find-after-update error: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name %q, got %q", newName, updated.Name)
	}
	if updated.Status != provider.ProviderStatusInactive {
		t.Fatalf("expected status inactive, got %s", updated.Status)
	}
	// Unmodified fields should be preserved via COALESCE.
	if updated.Protocol != provider.ProtocolOpenAI {
		t.Fatalf("expected protocol to be preserved, got %q", updated.Protocol)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	if _, err := repo.Find(ctx, id); !isNotFound(err) {
		t.Fatalf("expected not-found error after delete, got %v", err)
	}
}

func TestProviderRepo_Find_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)

	_, err := repo.Find(context.Background(), identity.NewProviderID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestProviderRepo_Update_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)

	name := "x"
	err := repo.Update(context.Background(), identity.NewProviderID(), provider.Update{Name: &name})
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestProviderRepo_Delete_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)

	err := repo.Delete(context.Background(), identity.NewProviderID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestProviderRepo_List_FilterByStatusAndSearch(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)
	ctx := context.Background()

	mustCreateProvider(t, repo, provider.Create{Name: "Anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://a.example.com"})
	inactiveID := mustCreateProvider(t, repo, provider.Create{Name: "Cohere", Protocol: provider.ProtocolCohere, BaseURL: "https://c.example.com"})
	inactive := provider.ProviderStatusInactive
	if err := repo.Update(ctx, inactiveID, provider.Update{Status: &inactive}); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	active := provider.ProviderStatusActive
	page, err := repo.List(ctx, provider.Filter{Status: &active}, query.Pagination{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	for _, p := range page.Items {
		if p.Status != provider.ProviderStatusActive {
			t.Fatalf("expected only active providers, got %+v", p)
		}
	}

	search := "anthro"
	searched, err := repo.List(ctx, provider.Filter{Search: &search}, query.Pagination{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(searched.Items) != 1 || searched.Items[0].Name != "Anthropic" {
		t.Fatalf("expected single anthropic match, got %+v", searched.Items)
	}
	if searched.Page.Total != 1 {
		t.Fatalf("expected total 1, got %d", searched.Page.Total)
	}
}

func TestProviderRepo_List_Pagination(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewProvider(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		mustCreateProvider(t, repo, provider.Create{
			Name: "Provider" + string(rune('A'+i)),
			Protocol: provider.ProtocolOpenAI, BaseURL: "https://example.com",
		})
	}

	page1, err := repo.List(ctx, provider.Filter{}, query.Pagination{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page1.Items) != 2 || page1.Page.Total != 5 {
		t.Fatalf("expected 2 items of 5 total, got %d items, total %d", len(page1.Items), page1.Page.Total)
	}

	page2, err := repo.List(ctx, provider.Filter{}, query.Pagination{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("expected 2 items on second page, got %d", len(page2.Items))
	}
	if page1.Items[0].ID == page2.Items[0].ID {
		t.Fatalf("expected different items across pages")
	}
}

// ════════════════════════════════════════════════════════════════════
// Model Repository
// ════════════════════════════════════════════════════════════════════

func TestModelRepo_CreateFindByNameUpdateDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewModel(db)
	ctx := context.Background()

	id := identity.NewModelID()
	create := provider.CreateModel{Name: "gpt-4o", Description: "flagship", Family: "gpt", Free: false}
	if err := repo.Create(ctx, id, create); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	byID, err := repo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if byID.Name != "gpt-4o" {
		t.Fatalf("unexpected model: %+v", byID)
	}

	byName, err := repo.FindByName(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected find-by-name error: %v", err)
	}
	if byName.ID != id {
		t.Fatalf("expected same ID via FindByName")
	}

	newFamily := "gpt-updated"
	if err := repo.Update(ctx, id, provider.UpdateModel{Family: &newFamily}); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	updated, err := repo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if updated.Family != newFamily {
		t.Fatalf("expected updated family %q, got %q", newFamily, updated.Family)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := repo.Find(ctx, id); !isNotFound(err) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestModelRepo_FindByName_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewModel(db)

	_, err := repo.FindByName(context.Background(), "does-not-exist")
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestModelRepo_List_FilterByFamily(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := providerpg.NewModel(db)
	ctx := context.Background()

	mustCreateModel(t, repo, provider.CreateModel{Name: "gpt-4o", Family: "gpt"})
	mustCreateModel(t, repo, provider.CreateModel{Name: "claude-3", Family: "claude"})

	family := "gpt"
	page, err := repo.List(ctx, provider.ModelFilter{Family: &family}, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Family != "gpt" {
		t.Fatalf("expected single gpt-family model, got %+v", page.Items)
	}
}

// ════════════════════════════════════════════════════════════════════
// Mapping Repository
// ════════════════════════════════════════════════════════════════════

func TestMappingRepo_CreateFindListActiveUpdateDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	providerRepo := providerpg.NewProvider(db)
	modelRepo := providerpg.NewModel(db)
	mappingRepo := providerpg.NewMapping(db)
	ctx := context.Background()

	providerID := mustCreateProvider(t, providerRepo, provider.Create{Name: "OpenAI", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com"})
	modelID := mustCreateModel(t, modelRepo, provider.CreateModel{Name: "gpt-4o-mapping", Family: "gpt"})

	inputPrice := 2.5
	id := identity.NewMappingID()
	create := provider.CreateMapping{
		ModelID:    modelID,
		ProviderID: providerID,
		ExternalID: "gpt-4o",
		InputPrice: &inputPrice,
		Streaming:  true,
	}
	if err := mappingRepo.Create(ctx, id, create); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	found, err := mappingRepo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if found.ExternalID != "gpt-4o" || found.InputPrice == nil || *found.InputPrice != inputPrice {
		t.Fatalf("unexpected mapping: %+v", found)
	}
	if found.Status != provider.ModelStatusActive {
		t.Fatalf("expected default status active, got %s", found.Status)
	}

	active, err := mappingRepo.ListActiveByModel(ctx, modelID)
	if err != nil {
		t.Fatalf("unexpected list active error: %v", err)
	}
	if len(active) != 1 || active[0].ID != id {
		t.Fatalf("expected single active mapping, got %+v", active)
	}

	newExternal := "gpt-4o-v2"
	if err := mappingRepo.Update(ctx, id, provider.UpdateMapping{ExternalID: &newExternal}); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	updated, err := mappingRepo.Find(ctx, id)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if updated.ExternalID != newExternal {
		t.Fatalf("expected updated external id %q, got %q", newExternal, updated.ExternalID)
	}

	if err := mappingRepo.Delete(ctx, id); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := mappingRepo.Find(ctx, id); !isNotFound(err) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestMappingRepo_List_FilterByProvider(t *testing.T) {
	db := testutil.PostgresDB(t)
	providerRepo := providerpg.NewProvider(db)
	modelRepo := providerpg.NewModel(db)
	mappingRepo := providerpg.NewMapping(db)
	ctx := context.Background()

	providerA := mustCreateProvider(t, providerRepo, provider.Create{Name: "A", Protocol: provider.ProtocolOpenAI, BaseURL: "https://a.example.com"})
	providerB := mustCreateProvider(t, providerRepo, provider.Create{Name: "B", Protocol: provider.ProtocolOpenAI, BaseURL: "https://b.example.com"})
	modelID := mustCreateModel(t, modelRepo, provider.CreateModel{Name: "shared-model", Family: "fam"})

	mustCreateMapping(t, mappingRepo, provider.CreateMapping{ModelID: modelID, ProviderID: providerA, ExternalID: "ext-a"})
	mustCreateMapping(t, mappingRepo, provider.CreateMapping{ModelID: modelID, ProviderID: providerB, ExternalID: "ext-b"})

	page, err := mappingRepo.List(ctx, provider.MappingFilter{ProviderID: &providerA}, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ProviderID != providerA {
		t.Fatalf("expected single mapping for providerA, got %+v", page.Items)
	}
}

// ════════════════════════════════════════════════════════════════════
// Fallback Repository
// ════════════════════════════════════════════════════════════════════

func TestFallbackRepo_CreateFindByModelDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	modelRepo := providerpg.NewModel(db)
	fallbackRepo := providerpg.NewFallback(db)
	ctx := context.Background()

	primary := mustCreateModel(t, modelRepo, provider.CreateModel{Name: "primary-model", Family: "fam"})
	fallback := mustCreateModel(t, modelRepo, provider.CreateModel{Name: "fallback-model", Family: "fam"})

	id := identity.NewModelFallbackID()
	create := provider.CreateFallback{ModelID: primary, FallbackModelID: fallback, Priority: 1}
	if err := fallbackRepo.Create(ctx, id, create); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	items, err := fallbackRepo.FindByModel(ctx, primary)
	if err != nil {
		t.Fatalf("unexpected find-by-model error: %v", err)
	}
	if len(items) != 1 || items[0].FallbackModelID != fallback {
		t.Fatalf("unexpected fallback items: %+v", items)
	}
	if !items[0].Enabled {
		t.Fatalf("expected fallback enabled by default")
	}

	if err := fallbackRepo.Delete(ctx, id); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	items, err = fallbackRepo.FindByModel(ctx, primary)
	if err != nil {
		t.Fatalf("unexpected find-by-model error after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no fallbacks after delete, got %+v", items)
	}
}

func TestFallbackRepo_Delete_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	fallbackRepo := providerpg.NewFallback(db)

	err := fallbackRepo.Delete(context.Background(), identity.NewModelFallbackID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ── helpers ──────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	var appErr *errx.Error
	return errx.As(err, &appErr) && appErr.Type == errx.TypeNotFound
}

func mustCreateProvider(t *testing.T, repo *providerpg.ProviderRepo, create provider.Create) identity.ProviderID {
	t.Helper()
	id := identity.NewProviderID()
	if err := repo.Create(context.Background(), id, create); err != nil {
		t.Fatalf("failed to create provider fixture: %v", err)
	}
	return id
}

func mustCreateModel(t *testing.T, repo *providerpg.ModelRepo, create provider.CreateModel) identity.ModelID {
	t.Helper()
	id := identity.NewModelID()
	if err := repo.Create(context.Background(), id, create); err != nil {
		t.Fatalf("failed to create model fixture: %v", err)
	}
	return id
}

func mustCreateMapping(t *testing.T, repo *providerpg.MappingRepo, create provider.CreateMapping) identity.MappingID {
	t.Helper()
	id := identity.NewMappingID()
	if err := repo.Create(context.Background(), id, create); err != nil {
		t.Fatalf("failed to create mapping fixture: %v", err)
	}
	return id
}
