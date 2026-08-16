package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestCheckTenantName_Available(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/availability?name=Nova%20Barbearia", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Available bool `json:"available"`
	}
	decodeBody(t, rec, &body)
	if !body.Available {
		t.Fatal("expected the name to be available")
	}
}

func TestCheckTenantName_Taken(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia do Zé"}
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/availability?name=Barbearia%20do%20Z%C3%A9", "", nil)

	var body struct {
		Available bool `json:"available"`
	}
	decodeBody(t, rec, &body)
	if body.Available {
		t.Fatal("expected the name to be unavailable")
	}
}

func TestCreateTenant_HappyPath(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenants", "", map[string]string{
		"tenant_name": "Barbearia Nova", "slug": "barbearia-nova",
		"manager_name": "Fulano", "email": "fulano@x.com", "password": "s3cret123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	decodeBody(t, rec, &body)
	if body.AccessToken == "" {
		t.Fatal("expected the new manager to be logged in immediately")
	}
}

func TestCreateTenant_InvalidSlug(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenants", "", map[string]string{
		"tenant_name": "Barbearia Nova", "slug": "Slug Inválido!",
		"manager_name": "Fulano", "email": "fulano@x.com", "password": "s3cret123",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateTenant_ShortPassword(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenants", "", map[string]string{
		"tenant_name": "Barbearia Nova", "slug": "barbearia-nova",
		"manager_name": "Fulano", "email": "fulano@x.com", "password": "short",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a short password, got %d", rec.Code)
	}
}

func TestGetTenant_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia do Zé"}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/tenant", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateTenant_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant", authHeader(t, tok, prof), map[string]any{
		"name": "Nova", "slug": "nova",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestUpdateTenant_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Antigo", Slug: "antigo"}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant", authHeader(t, tok, manager), map[string]any{
		"name": "Novo Nome", "slug": "novo-nome", "self_scheduling_enabled": true,
		"auto_confirm_appointments": false, "cancellation_window_hours": 12,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.tenants["t1"].Name != "Novo Nome" {
		t.Fatalf("expected tenant name to be updated, got %q", store.tenants["t1"].Name)
	}
}

func TestUpdateTenant_NegativeCancellationWindow(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Antigo", Slug: "antigo"}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant", authHeader(t, tok, manager), map[string]any{
		"name": "Novo Nome", "slug": "novo-nome", "cancellation_window_hours": -1,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative cancellation window, got %d", rec.Code)
	}
}

func TestHolidaysCRUD(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	create := doRequest(t, handler, http.MethodPost, "/api/v1/tenant/holidays", authHeader(t, tok, manager),
		map[string]string{"date": "2026-12-25", "name": "Natal"})
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", create.Code, create.Body.String())
	}
	var created domain.Holiday
	decodeBody(t, create, &created)

	list := doRequest(t, handler, http.MethodGet, "/api/v1/tenant/holidays", authHeader(t, tok, manager), nil)
	var holidays []domain.Holiday
	decodeBody(t, list, &holidays)
	if len(holidays) != 1 {
		t.Fatalf("expected 1 holiday listed, got %d", len(holidays))
	}

	del := doRequest(t, handler, http.MethodDelete, "/api/v1/tenant/holidays/"+created.ID, authHeader(t, tok, manager), nil)
	if del.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting the holiday, got %d: %s", del.Code, del.Body.String())
	}
}

func TestCreateHoliday_InvalidDate(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenant/holidays", authHeader(t, tok, manager),
		map[string]string{"date": "25/12/2026"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-ISO date, got %d", rec.Code)
	}
}

func TestCreateHoliday_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenant/holidays", authHeader(t, tok, prof),
		map[string]string{"date": "2026-12-25"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDeleteHoliday_NotFound(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodDelete, "/api/v1/tenant/holidays/missing", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
