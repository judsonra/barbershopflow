package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestListTenantsAdmin_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/admin/tenants", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 - only superadmin, got %d", rec.Code)
	}
}

func TestListTenantsAdmin_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia"}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/admin/tenants", authHeader(t, tok, super), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImpersonateTenant_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia"}
	store.memberships["m1"] = domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/admin/tenants/t1/impersonate", authHeader(t, tok, super), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(store.audits) != 1 {
		t.Fatalf("expected the impersonation to be recorded in the audit trail")
	}
}

func TestImpersonateTenant_NoManager(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia"}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/admin/tenants/t1/impersonate", authHeader(t, tok, super), nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when the tenant has no manager, got %d", rec.Code)
	}
}

func TestAdminSearchCustomers_EmptyQueryReturnsEmpty(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{"c1": {ID: "c1", Name: "Cliente"}}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/admin/customers", authHeader(t, tok, super), nil)

	var got []domain.AdminCustomerMatch
	decodeBody(t, rec, &got)
	if len(got) != 0 {
		t.Fatalf("expected no rows for an empty query (no full-platform dump), got %d", len(got))
	}
}

func TestAdminSearchCustomers_MatchesAcrossTenants(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Barbearia A"}
	store.customers["t1"] = map[string]domain.Customer{"c1": {ID: "c1", Name: "João Silva"}}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/admin/customers?q=Jo%C3%A3o", authHeader(t, tok, super), nil)

	var got []domain.AdminCustomerMatch
	decodeBody(t, rec, &got)
	if len(got) != 1 || got[0].TenantName != "Barbearia A" {
		t.Fatalf("expected 1 match with tenant context, got %+v", got)
	}
}

func TestPromoteToSuperAdmin_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.identities["id-1"] = domain.Identity{ID: "id-1", Email: "gestor@x.com"}
	store.memberships["m-1"] = domain.User{ID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleManager}
	store.membershipOptions["id-1"] = []domain.MembershipOption{{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleManager}}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/admin/promote", authHeader(t, tok, super), map[string]string{"email": "gestor@x.com"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.memberships["m-1"].Role != domain.RoleSuperAdmin {
		t.Fatalf("expected the membership's role to be promoted, got %q", store.memberships["m-1"].Role)
	}
}

func TestPromoteToSuperAdmin_UnknownEmail(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/admin/promote", authHeader(t, tok, super), map[string]string{"email": "nobody@x.com"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPromoteToSuperAdmin_AmbiguousIdentity(t *testing.T) {
	store := newFakeStore()
	store.identities["id-1"] = domain.Identity{ID: "id-1", Email: "multi@x.com"}
	store.membershipOptions["id-1"] = []domain.MembershipOption{
		{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1"},
		{MembershipID: "m-2", IdentityID: "id-1", TenantID: "t2"},
	}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/admin/promote", authHeader(t, tok, super), map[string]string{"email": "multi@x.com"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for an identity with more than one membership, got %d", rec.Code)
	}
}

func TestAdminListAudit_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.audits = []domain.ImpersonationAudit{{ID: "a1", TenantName: "Barbearia"}}
	handler, tok, _ := newTestServer(store)
	super := domain.User{ID: "s1", Role: domain.RoleSuperAdmin}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/admin/audit", authHeader(t, tok, super), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
