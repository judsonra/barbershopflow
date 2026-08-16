package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

// validTestCPF is a commonly used check-digit-valid Brazilian CPF for tests
// (not a real person's document).
const validTestCPF = "11144477735"

func TestCreateProfessional_HappyPath(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals", authHeader(t, tok, manager),
		domain.Professional{Name: "Barbeiro", Email: "barbeiro@x.com", CPF: validTestCPF})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProfessional_RoleGuard(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals", authHeader(t, tok, client),
		domain.Professional{Name: "Barbeiro"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCreateProfessional_InvalidEmail(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals", authHeader(t, tok, manager),
		domain.Professional{Name: "Barbeiro", Email: "not-an-email"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid e-mail, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProfessional_InvalidCPF(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals", authHeader(t, tok, manager),
		domain.Professional{Name: "Barbeiro", CPF: "00000000000"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CPF, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProfessional_EmptyName(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals", authHeader(t, tok, manager), domain.Professional{Name: "  "})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a blank name, got %d", rec.Code)
	}
}

func TestSetProfessionalSchedule_OwnerAllowed(t *testing.T) {
	store := newFakeStore()
	store.professionals["t1"] = map[string]domain.Professional{"p1": {ID: "p1", Name: "Barbeiro"}}
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "u1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "p1"}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/schedule", authHeader(t, tok, prof), map[string]any{
		"entries": []map[string]int{{"weekday": 1, "start_minute": 480, "end_minute": 1080}},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when a professional sets their own schedule, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetProfessionalSchedule_OtherProfessionalForbidden(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "u1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "someone-else"}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/schedule", authHeader(t, tok, prof), map[string]any{
		"entries": []map[string]int{{"weekday": 1, "start_minute": 480, "end_minute": 1080}},
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when a professional edits someone else's schedule, got %d", rec.Code)
	}
}

func TestSetProfessionalSchedule_ManagerAllowedForAnyone(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/schedule", authHeader(t, tok, manager), map[string]any{
		"entries": []map[string]int{{"weekday": 1, "start_minute": 480, "end_minute": 1080}},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetProfessionalSchedule_InvalidWeekday(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/schedule", authHeader(t, tok, manager), map[string]any{
		"entries": []map[string]int{{"weekday": 9, "start_minute": 480, "end_minute": 1080}},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an out-of-range weekday, got %d", rec.Code)
	}
}

func TestSetProfessionalSchedule_DuplicateWeekday(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/schedule", authHeader(t, tok, manager), map[string]any{
		"entries": []map[string]int{
			{"weekday": 1, "start_minute": 480, "end_minute": 1080},
			{"weekday": 1, "start_minute": 0, "end_minute": 100},
		},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a repeated weekday, got %d", rec.Code)
	}
}

func TestCreateTimeOff_OwnerAllowed(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "u1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "p1"}

	starts := time.Now().Add(24 * time.Hour)
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/time-off", authHeader(t, tok, prof), map[string]any{
		"starts_at": starts.Format(time.RFC3339),
		"ends_at":   starts.Add(time.Hour).Format(time.RFC3339),
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTimeOff_InvalidRange(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	starts := time.Now().Add(24 * time.Hour)
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/time-off", authHeader(t, tok, manager), map[string]any{
		"starts_at": starts.Format(time.RFC3339),
		"ends_at":   starts.Add(-time.Hour).Format(time.RFC3339), // ends before it starts
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ends_at before starts_at, got %d", rec.Code)
	}
}

func TestSetProfessionalServices_HappyPath(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	price := int64(4000)
	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/services", authHeader(t, tok, manager), map[string]any{
		"entries": []domain.ProfessionalService{{ServiceID: "s1", PriceCentsOverride: &price}},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetProfessionalServices_NegativePriceOverride(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	price := int64(-100)
	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/services", authHeader(t, tok, manager), map[string]any{
		"entries": []domain.ProfessionalService{{ServiceID: "s1", PriceCentsOverride: &price}},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative price override, got %d", rec.Code)
	}
}

func TestSetProfessionalServices_DuplicateService(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/services", authHeader(t, tok, manager), map[string]any{
		"entries": []domain.ProfessionalService{{ServiceID: "s1"}, {ServiceID: "s1"}},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a repeated service_id, got %d", rec.Code)
	}
}

func TestGrantProfessionalAccess_NewIdentity(t *testing.T) {
	store := newFakeStore()
	store.professionals["t1"] = map[string]domain.Professional{"p1": {ID: "p1", Name: "Barbeiro"}}
	handler, tok, notifier := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/credentials", authHeader(t, tok, manager),
		map[string]string{"phone": "+5511988887777"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("expected the notifier to send the new password once, got %+v", notifier.sent)
	}
}

func TestGrantProfessionalAccess_RoleGuard(t *testing.T) {
	store := newFakeStore()
	store.professionals["t1"] = map[string]domain.Professional{"p1": {ID: "p1", Name: "Barbeiro"}}
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "u1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "p1"}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/credentials", authHeader(t, tok, prof),
		map[string]string{"phone": "+5511988887777"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 - only managers can grant professional access, got %d", rec.Code)
	}
}

func TestGrantProfessionalAccess_ProfessionalNotFound(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/missing/credentials", authHeader(t, tok, manager),
		map[string]string{"phone": "+5511988887777"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
