package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

func newAppointmentBody(customerID, professionalID, serviceID string) map[string]any {
	starts := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	return map[string]any{
		"customer_id":     customerID,
		"professional_id": professionalID,
		"service_id":      serviceID,
		"starts_at":       starts.Format(time.RFC3339),
		"ends_at":         starts.Add(30 * time.Minute).Format(time.RFC3339),
	}
}

func TestCreateAppointment_StaffHappyPath(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, manager),
		newAppointmentBody("cust-1", "prof-1", "svc-1"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.Appointment
	decodeBody(t, rec, &got)
	if got.Status != domain.StatusScheduled {
		t.Fatalf("expected status scheduled, got %q", got.Status)
	}
	if _, ok := store.appointments["t1"][got.ID]; !ok {
		t.Fatal("expected appointment to be persisted under tenant t1")
	}
}

func TestCreateAppointment_ClientSelfScheduling(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", SelfSchedulingEnabled: true, AutoConfirmAppointments: true}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	body := newAppointmentBody("someone-else", "prof-1", "svc-1") // customer_id must be forced to claims.CustomerID
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, client), body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.Appointment
	decodeBody(t, rec, &got)
	if got.CustomerID != "cust-1" {
		t.Fatalf("expected customer_id forced to claims.CustomerID, got %q", got.CustomerID)
	}
	if got.Status != domain.StatusConfirmed {
		t.Fatalf("expected auto-confirmed status, got %q", got.Status)
	}
}

func TestCreateAppointment_ClientSelfSchedulingNotAutoConfirmed(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", SelfSchedulingEnabled: true, AutoConfirmAppointments: false}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, client),
		newAppointmentBody("cust-1", "prof-1", "svc-1"))

	var got domain.Appointment
	decodeBody(t, rec, &got)
	if got.Status != domain.StatusScheduled {
		t.Fatalf("expected scheduled (not auto-confirmed), got %q", got.Status)
	}
}

func TestCreateAppointment_ClientSelfSchedulingDisabled(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", SelfSchedulingEnabled: false}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, client),
		newAppointmentBody("cust-1", "prof-1", "svc-1"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when self-scheduling disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAppointment_Unauthenticated(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", "", newAppointmentBody("c", "p", "s"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Authorization header, got %d", rec.Code)
	}
}

func TestCreateAppointment_InvalidToken(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", "Bearer garbage", newAppointmentBody("c", "p", "s"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with an invalid token, got %d", rec.Code)
	}
}

func TestCreateAppointment_ValidationError(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	body := newAppointmentBody("cust-1", "prof-1", "svc-1")
	delete(body, "customer_id")
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, manager), body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing customer_id, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "validation_error" {
		t.Fatalf("expected validation_error code, got %q", code)
	}
}

func TestCreateAppointment_MalformedJSON(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	req := doRawRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, manager), `{"customer_id": `)
	if req.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", req.Code)
	}
	if code := errorCode(t, req); code != "invalid_json" {
		t.Fatalf("expected invalid_json code, got %q", code)
	}
}

func TestCreateAppointment_UnknownField(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	req := doRawRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, manager), `{"unexpected_field": true}`)
	if req.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field (DisallowUnknownFields), got %d", req.Code)
	}
}

// TestCreateAppointment_DomainErrorMapping exercises every domain error
// sentinel CreateAppointment can plausibly return, asserting the exact
// status/code pair from handleError.
func TestCreateAppointment_DomainErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"schedule conflict", domain.ErrScheduleConflict, http.StatusConflict, "schedule_conflict"},
		{"outside working hours", domain.ErrOutsideWorkingHours, http.StatusConflict, "outside_working_hours"},
		{"time blocked", domain.ErrTimeBlocked, http.StatusConflict, "time_blocked"},
		{"service not offered", domain.ErrServiceNotOffered, http.StatusConflict, "service_not_offered"},
		{"holiday blocked", domain.ErrHolidayBlocked, http.StatusConflict, "holiday_blocked"},
		{"generic conflict", domain.ErrConflict, http.StatusConflict, "conflict"},
		{"unmapped error", errUnmapped, http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.errs["CreateAppointment"] = tc.err
			handler, tok, _ := newTestServer(store)
			manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

			rec := doRequest(t, handler, http.MethodPost, "/api/v1/appointments", authHeader(t, tok, manager),
				newAppointmentBody("cust-1", "prof-1", "svc-1"))

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
			if code := errorCode(t, rec); code != tc.wantCode {
				t.Fatalf("expected code %q, got %q", tc.wantCode, code)
			}
		})
	}
}

func TestUpdateStatus_ManagerHappyPath(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, ProfessionalID: "prof-1", CustomerID: "cust-1"},
	}
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, manager),
		map[string]string{"status": domain.StatusConfirmed})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.appointments["t1"]["a1"].Status != domain.StatusConfirmed {
		t.Fatalf("expected persisted status confirmed, got %q", store.appointments["t1"]["a1"].Status)
	}
}

func TestUpdateStatus_ProfessionalRestrictedToOwn(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, ProfessionalID: "prof-other", CustomerID: "cust-1"},
	}
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "prof-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, prof),
		map[string]string{"status": domain.StatusCompleted})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when professional isn't the appointment owner, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_ProfessionalOwnAppointment(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusConfirmed, ProfessionalID: "prof-1", CustomerID: "cust-1"},
	}
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "prof-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, prof),
		map[string]string{"status": domain.StatusNoShow})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when professional owns the appointment, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_ClientCanOnlyCancel(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, CustomerID: "cust-1"},
	}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, client),
		map[string]string{"status": domain.StatusCompleted})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when client attempts a non-cancel transition, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_ClientCancelOthersAppointment(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, CustomerID: "cust-other", StartsAt: time.Now().Add(48 * time.Hour)},
	}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, client),
		map[string]string{"status": domain.StatusCancelled})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when client cancels someone else's appointment, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_ClientCancelOutsideWindow(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", CancellationWindowHours: 24}
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, CustomerID: "cust-1", StartsAt: time.Now().Add(2 * time.Hour)},
	}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, client),
		map[string]string{"status": domain.StatusCancelled})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when cancelling inside the cancellation window, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_ClientCancelWithinWindow(t *testing.T) {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", CancellationWindowHours: 24}
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusScheduled, CustomerID: "cust-1", StartsAt: time.Now().Add(48 * time.Hour)},
	}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, client),
		map[string]string{"status": domain.StatusCancelled})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when cancelling with enough notice, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_InvalidTransitionMapping(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", Status: domain.StatusCompleted, ProfessionalID: "prof-1", CustomerID: "cust-1"},
	}
	store.errs["UpdateAppointmentStatus"] = domain.ErrInvalidTransition
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/a1/status", authHeader(t, tok, manager),
		map[string]string{"status": domain.StatusConfirmed})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "invalid_transition" {
		t.Fatalf("expected invalid_transition code, got %q", code)
	}
}

func TestUpdateStatus_AppointmentNotFound(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodPatch, "/api/v1/appointments/missing/status", authHeader(t, tok, manager),
		map[string]string{"status": domain.StatusConfirmed})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListAppointments_ClientSeesOnlyOwn(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", CustomerID: "cust-1", StartsAt: time.Now().Add(time.Hour)},
		"a2": {ID: "a2", CustomerID: "cust-2", StartsAt: time.Now().Add(2 * time.Hour)},
	}
	handler, tok, _ := newTestServer(store)
	client := domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient, CustomerID: "cust-1"}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/appointments", authHeader(t, tok, client), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []domain.Appointment
	decodeBody(t, rec, &got)
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("expected only the client's own appointment, got %+v", got)
	}
}

func TestListAppointments_ProfessionalSeesOnlyOwn(t *testing.T) {
	store := newFakeStore()
	store.appointments["t1"] = map[string]domain.Appointment{
		"a1": {ID: "a1", ProfessionalID: "prof-1", StartsAt: time.Now().Add(time.Hour)},
		"a2": {ID: "a2", ProfessionalID: "prof-2", StartsAt: time.Now().Add(2 * time.Hour)},
	}
	handler, tok, _ := newTestServer(store)
	prof := domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "prof-1"}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/appointments", authHeader(t, tok, prof), nil)

	var got []domain.Appointment
	decodeBody(t, rec, &got)
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("expected only the professional's own appointments, got %+v", got)
	}
}

func TestListAppointments_InvalidDateRange(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/appointments?from=not-a-date", authHeader(t, tok, manager), nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed from param, got %d", rec.Code)
	}
}
