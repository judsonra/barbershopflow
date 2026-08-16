package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestGetReport_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/reports", authHeader(t, tok, prof), nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 - only managers see reports, got %d", rec.Code)
	}
}

func TestGetReport_HappyPathDefaultsToCurrentMonth(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/reports", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetReport_InvalidDateRange(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/reports?from=2026-01-01T00:00:00Z&to=2025-01-01T00:00:00Z", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when to is before from, got %d", rec.Code)
	}
}

func TestGetReport_StoreErrorMapping(t *testing.T) {
	store := newFakeStore()
	store.errs["GetReport"] = errUnmapped
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/reports", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for an unmapped store error, got %d", rec.Code)
	}
}
