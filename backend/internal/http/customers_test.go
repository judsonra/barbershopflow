package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestListCustomers_RequireStaff(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/customers", authHeader(t, tok, client), nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 - clients can't list customers, got %d", rec.Code)
	}
}

func TestListCustomers_ProfessionalAllowed(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/customers", authHeader(t, tok, prof), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 - staff (professional included) can list customers, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListCustomers_NoParamsReturnsEverything(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{
		"c1": {ID: "c1", Name: "Ana"}, "c2": {ID: "c2", Name: "Bruno"}, "c3": {ID: "c3", Name: "Carla"},
	}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/customers", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.CustomerPage
	decodeBody(t, rec, &got)
	if len(got.Items) != 3 || got.Total != 3 {
		t.Fatalf("expected all 3 customers with total=3, got %+v", got)
	}
}

func TestListCustomers_Paginated(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{
		"c1": {ID: "c1", Name: "Ana"}, "c2": {ID: "c2", Name: "Bruno"}, "c3": {ID: "c3", Name: "Carla"},
	}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/customers?page=1&limit=2", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.CustomerPage
	decodeBody(t, rec, &got)
	if len(got.Items) != 2 || got.Total != 3 {
		t.Fatalf("expected 2 items (page 1 of limit 2) with total=3, got %+v", got)
	}
	if got.Items[0].Name != "Ana" || got.Items[1].Name != "Bruno" {
		t.Fatalf("expected alphabetical order Ana, Bruno on page 1, got %+v", got.Items)
	}

	rec2 := doRequest(t, handler, http.MethodGet, "/api/v1/customers?page=2&limit=2", authHeader(t, tok, manager), nil)
	var got2 domain.CustomerPage
	decodeBody(t, rec2, &got2)
	if len(got2.Items) != 1 || got2.Items[0].Name != "Carla" {
		t.Fatalf("expected page 2 to have just Carla, got %+v", got2.Items)
	}
}

func TestListCustomers_InvalidPagination(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	cases := []string{"?limit=0", "?limit=101", "?limit=abc", "?limit=10&page=0", "?limit=10&page=abc"}
	for _, qs := range cases {
		t.Run(qs, func(t *testing.T) {
			rec := doRequest(t, handler, http.MethodGet, "/api/v1/customers"+qs, authHeader(t, tok, manager), nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for query %q, got %d", qs, rec.Code)
			}
		})
	}
}

func TestCreateCustomer_HappyPath(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/customers", authHeader(t, tok, manager),
		domain.Customer{Name: "Cliente"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCustomer_EmptyName(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/customers", authHeader(t, tok, manager), domain.Customer{Name: ""})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateCustomer_NotFound(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/customers/missing", authHeader(t, tok, manager), domain.Customer{Name: "X"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGrantCustomerAccess_UsesExistingPhoneWhenOmitted(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{"c1": {ID: "c1", Name: "Cliente", Phone: "+5511977776666"}}
	handler, tok, notifier := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/customers/c1/credentials", authHeader(t, tok, manager), map[string]string{})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(notifier.sent) != 1 || notifier.sent[0].Phone != "+5511977776666" {
		t.Fatalf("expected the notifier to use the customer's own phone, got %+v", notifier.sent)
	}
}

func TestGrantCustomerAccess_NoPhoneAvailable(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{"c1": {ID: "c1", Name: "Cliente"}}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/customers/c1/credentials", authHeader(t, tok, manager), map[string]string{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no phone is available, got %d", rec.Code)
	}
}

func TestGrantCustomerAccess_ExistingIdentityAttachesMembership(t *testing.T) {
	store := newFakeStore()
	store.customers["t1"] = map[string]domain.Customer{"c1": {ID: "c1", Name: "Cliente"}}
	store.identities["id-1"] = domain.Identity{ID: "id-1", Name: "Cliente", Phone: "+5511977776666"}
	handler, tok, notifier := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/customers/c1/credentials", authHeader(t, tok, manager),
		map[string]string{"phone": "+5511977776666"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("expected no new password sent when attaching to an existing identity, got %+v", notifier.sent)
	}
	if len(store.membershipOptions["id-1"]) != 1 {
		t.Fatalf("expected a membership to be attached to the existing identity")
	}
}
