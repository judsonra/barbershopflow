package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestListServices_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.services["t1"] = map[string]domain.Service{"s1": {ID: "s1", Name: "Corte", DurationMinutes: 30, PriceCents: 5000, Active: true}}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/services", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []domain.Service
	decodeBody(t, rec, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got))
	}
}

func TestCreateService_HappyPath(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/services", authHeader(t, tok, manager),
		domain.Service{Name: "Barba", DurationMinutes: 20, PriceCents: 3000})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateService_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/services", authHeader(t, tok, prof),
		domain.Service{Name: "Barba", DurationMinutes: 20, PriceCents: 3000})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-manager creating a service, got %d", rec.Code)
	}
}

func TestCreateService_ValidationErrors(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	cases := []struct {
		name    string
		service domain.Service
	}{
		{"empty name", domain.Service{Name: "", DurationMinutes: 30, PriceCents: 100}},
		{"duration too short", domain.Service{Name: "Corte", DurationMinutes: 4, PriceCents: 100}},
		{"duration too long", domain.Service{Name: "Corte", DurationMinutes: 481, PriceCents: 100}},
		{"negative price", domain.Service{Name: "Corte", DurationMinutes: 30, PriceCents: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, handler, http.MethodPost, "/api/v1/services", authHeader(t, tok, manager), tc.service)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdateService_NotFound(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/services/missing", authHeader(t, tok, manager),
		domain.Service{Name: "Corte", DurationMinutes: 30, PriceCents: 100})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateService_ConflictMapping(t *testing.T) {
	store := newFakeStore()
	store.errs["CreateService"] = domain.ErrConflict
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/services", authHeader(t, tok, manager),
		domain.Service{Name: "Corte", DurationMinutes: 30, PriceCents: 100})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
