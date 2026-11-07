package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

func commissionStore() *fakeStore {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Junin", Slug: "junin", Timezone: domain.DefaultTimezone, Active: true}
	store.tenants["t2"] = domain.Tenant{ID: "t2", Name: "Outra", Slug: "outra", Timezone: domain.DefaultTimezone, Active: true}
	store.professionals["t1"] = map[string]domain.Professional{"p1": {ID: "p1", Name: "Ana"}, "p2": {ID: "p2", Name: "Bruno"}}
	store.professionals["t2"] = map[string]domain.Professional{}
	store.services["t1"] = map[string]domain.Service{"s1": {ID: "s1", Name: "Corte", PriceCents: 5000}, "s2": {ID: "s2", Name: "Barba", PriceCents: 3000}}
	store.services["t2"] = map[string]domain.Service{"x1": {ID: "x1", Name: "De outra barbearia"}}
	return store
}

func TestSetProfessionalCommissions_Validation(t *testing.T) {
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}
	entry := func(service, kind string, value int64) map[string]any {
		return map[string]any{"service_id": service, "type": kind, "value": value}
	}
	cases := []struct {
		name     string
		user     domain.User
		prof     string
		entries  []map[string]any
		wantCode int
	}{
		{"percentual e fixo", manager, "p1", []map[string]any{entry("s1", "percentage", 4000), entry("s2", "fixed", 1500)}, http.StatusOK},
		{"lista vazia remove tudo", manager, "p1", []map[string]any{}, http.StatusOK},
		{"tipo inválido", manager, "p1", []map[string]any{entry("s1", "bonus", 10)}, http.StatusBadRequest},
		{"percentual acima de 100", manager, "p1", []map[string]any{entry("s1", "percentage", 10001)}, http.StatusBadRequest},
		{"valor negativo", manager, "p1", []map[string]any{entry("s1", "fixed", -1)}, http.StatusBadRequest},
		{"serviço repetido", manager, "p1", []map[string]any{entry("s1", "fixed", 1), entry("s1", "fixed", 2)}, http.StatusBadRequest},
		{"serviço vazio", manager, "p1", []map[string]any{entry("", "fixed", 1)}, http.StatusBadRequest},
		{"serviço de outra barbearia", manager, "p1", []map[string]any{entry("x1", "fixed", 1)}, http.StatusNotFound},
		{"profissional inexistente", manager, "nao-existe", []map[string]any{}, http.StatusNotFound},
		{"profissional não pode", domain.User{ID: "u1", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "p1"}, "p1", []map[string]any{}, http.StatusForbidden},
		{"cliente não pode", domain.User{ID: "u2", TenantID: "t1", Role: domain.RoleClient}, "p1", []map[string]any{}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, tok, _ := newTestServer(commissionStore())
			rec := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/"+tc.prof+"/commissions", authHeader(t, tok, tc.user), map[string]any{"entries": tc.entries})
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestProfessionalCommissions_RoundTrip(t *testing.T) {
	handler, tok, _ := newTestServer(commissionStore())
	auth := authHeader(t, tok, domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager})
	put := doRequest(t, handler, http.MethodPut, "/api/v1/professionals/p1/commissions", auth, map[string]any{
		"entries": []map[string]any{{"service_id": "s1", "type": "percentage", "value": 4000}},
	})
	if put.Code != http.StatusOK {
		t.Fatalf("put: %d %s", put.Code, put.Body.String())
	}
	get := doRequest(t, handler, http.MethodGet, "/api/v1/professionals/p1/commissions", auth, nil)
	var items []domain.ProfessionalCommission
	decodeBody(t, get, &items)
	if len(items) != 1 || items[0].ServiceID != "s1" || items[0].Value != 4000 {
		t.Fatalf("unexpected commissions: %+v", items)
	}
}

func TestCommissionReport(t *testing.T) {
	setup := func() *fakeStore {
		store := commissionStore()
		store.commissions["t1/p1"] = []domain.ProfessionalCommission{{ServiceID: "s1", Type: domain.CommissionPercentage, Value: 4000}}
		store.commissions["t1/p2"] = []domain.ProfessionalCommission{{ServiceID: "s1", Type: domain.CommissionFixed, Value: 1000}}
		day := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
		store.appointments["t1"] = map[string]domain.Appointment{
			"a1": {ID: "a1", ProfessionalID: "p1", ServiceID: "s1", Status: domain.StatusCompleted, StartsAt: day, PriceCents: 5000},
			"a2": {ID: "a2", ProfessionalID: "p1", ServiceID: "s2", Status: domain.StatusCompleted, StartsAt: day.Add(time.Hour), PriceCents: 3000},
			"a3": {ID: "a3", ProfessionalID: "p1", ServiceID: "s1", Status: domain.StatusScheduled, StartsAt: day, PriceCents: 5000},
			"a4": {ID: "a4", ProfessionalID: "p2", ServiceID: "s1", Status: domain.StatusCompleted, StartsAt: day, PriceCents: 5000},
			"a5": {ID: "a5", ProfessionalID: "p2", ServiceID: "s1", Status: domain.StatusCompleted, StartsAt: day.AddDate(0, 1, 0), PriceCents: 5000},
		}
		return store
	}
	const period = "/api/v1/reports/commissions?from=2026-11-01&to=2026-12-01"

	t.Run("gestor vê todos, só concluídos no período", func(t *testing.T) {
		handler, tok, _ := newTestServer(setup())
		rec := doRequest(t, handler, http.MethodGet, period, authHeader(t, tok, domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var report domain.CommissionReport
		decodeBody(t, rec, &report)
		if report.TotalCommissionCents != 3000 || len(report.Items) != 2 {
			t.Fatalf("unexpected report: %+v", report)
		}
		ana := report.Items[0]
		if ana.ProfessionalName != "Ana" || ana.Appointments != 2 || ana.CommissionCents != 2000 || ana.WithoutRule != 1 || ana.RevenueCents != 8000 {
			t.Fatalf("unexpected Ana: %+v", ana)
		}
	})

	t.Run("profissional vê só a própria linha", func(t *testing.T) {
		handler, tok, _ := newTestServer(setup())
		rec := doRequest(t, handler, http.MethodGet, period, authHeader(t, tok, domain.User{ID: "u2", TenantID: "t1", Role: domain.RoleProfessional, ProfessionalID: "p2"}), nil)
		var report domain.CommissionReport
		decodeBody(t, rec, &report)
		if len(report.Items) != 1 || report.Items[0].ProfessionalID != "p2" || report.TotalCommissionCents != 1000 {
			t.Fatalf("unexpected report for p2: %+v", report)
		}
	})

	t.Run("cliente não vê", func(t *testing.T) {
		handler, tok, _ := newTestServer(setup())
		rec := doRequest(t, handler, http.MethodGet, period, authHeader(t, tok, domain.User{ID: "c1", TenantID: "t1", Role: domain.RoleClient}), nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("período inválido", func(t *testing.T) {
		handler, tok, _ := newTestServer(setup())
		rec := doRequest(t, handler, http.MethodGet, "/api/v1/reports/commissions?from=2026-12-01&to=2026-11-01", authHeader(t, tok, domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}), nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}
