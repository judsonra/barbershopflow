package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

// Smallest valid PNG (1x1), enough for http.DetectContentType.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func doBytesRequest(t *testing.T, handler http.Handler, method, path, authorization, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func brandingStore() *fakeStore {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Junin Barber", Slug: "junin", ColorScheme: domain.DefaultColorScheme, Active: true}
	store.tenants["t2"] = domain.Tenant{ID: "t2", Name: "Fechada", Slug: "fechada", ColorScheme: "vinho", Active: false}
	return store
}

func TestPublicTenant(t *testing.T) {
	cases := []struct {
		name     string
		slug     string
		wantCode int
	}{
		{"barbearia ativa", "junin", http.StatusOK},
		{"barbearia inativa fica escondida", "fechada", http.StatusNotFound},
		{"slug inexistente", "nao-existe", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, _, _ := newTestServer(brandingStore())
			rec := doRequest(t, handler, http.MethodGet, "/api/v1/public/tenants/"+tc.slug, "", nil)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("expõe só os dados da marca", func(t *testing.T) {
		store := brandingStore()
		updated := time.Unix(1700000000, 0)
		tenant := store.tenants["t1"]
		tenant.LogoUpdatedAt = &updated
		store.tenants["t1"] = tenant
		handler, _, _ := newTestServer(store)

		rec := doRequest(t, handler, http.MethodGet, "/api/v1/public/tenants/junin", "", nil)
		var body map[string]any
		decodeBody(t, rec, &body)
		if body["name"] != "Junin Barber" || body["color_scheme"] != "classico" {
			t.Fatalf("unexpected body: %v", body)
		}
		if body["logo_url"] != "/api/v1/public/tenants/junin/logo?v=1700000000" {
			t.Fatalf("unexpected logo_url: %v", body["logo_url"])
		}
		for _, private := range []string{"id", "self_scheduling_enabled", "timezone", "active"} {
			if _, ok := body[private]; ok {
				t.Fatalf("public payload leaked %q: %v", private, body)
			}
		}
	})
}

func TestUpdateTenantBranding(t *testing.T) {
	cases := []struct {
		name     string
		role     string
		scheme   string
		wantCode int
	}{
		{"gestor escolhe paleta válida", domain.RoleManager, "vinho", http.StatusOK},
		{"paleta fora da lista", domain.RoleManager, "neon", http.StatusBadRequest},
		{"paleta vazia", domain.RoleManager, "", http.StatusBadRequest},
		{"profissional não pode", domain.RoleProfessional, "vinho", http.StatusForbidden},
		{"cliente não pode", domain.RoleClient, "vinho", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := brandingStore()
			handler, tok, _ := newTestServer(store)
			user := domain.User{ID: "u1", TenantID: "t1", Role: tc.role}
			rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant/branding", authHeader(t, tok, user), map[string]string{"color_scheme": tc.scheme})
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
			if tc.wantCode == http.StatusOK && store.tenants["t1"].ColorScheme != tc.scheme {
				t.Fatalf("scheme not saved: %q", store.tenants["t1"].ColorScheme)
			}
		})
	}

	t.Run("sem login", func(t *testing.T) {
		handler, _, _ := newTestServer(brandingStore())
		rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant/branding", "", map[string]string{"color_scheme": "vinho"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestUploadTenantLogo(t *testing.T) {
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}
	cases := []struct {
		name        string
		user        domain.User
		contentType string
		body        []byte
		wantCode    int
	}{
		{"PNG válido", manager, "image/png", pngBytes, http.StatusOK},
		{"texto disfarçado de PNG", manager, "image/png", []byte("<svg onload=alert(1)></svg>"), http.StatusBadRequest},
		{"GIF não é aceito", manager, "image/gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), http.StatusBadRequest},
		{"corpo vazio", manager, "image/png", nil, http.StatusBadRequest},
		{"maior que 1 MB", manager, "image/png", append(append([]byte{}, pngBytes...), make([]byte, domain.MaxLogoBytes)...), http.StatusRequestEntityTooLarge},
		{"profissional não pode", domain.User{ID: "p1", TenantID: "t1", Role: domain.RoleProfessional}, "image/png", pngBytes, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := brandingStore()
			handler, tok, _ := newTestServer(store)
			rec := doBytesRequest(t, handler, http.MethodPut, "/api/v1/tenant/logo", authHeader(t, tok, tc.user), tc.contentType, tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
			_, saved := store.logos["t1"]
			if saved != (tc.wantCode == http.StatusOK) {
				t.Fatalf("logo saved=%v for status %d", saved, rec.Code)
			}
			if tc.wantCode == http.StatusOK {
				var body map[string]any
				decodeBody(t, rec, &body)
				if url, _ := body["logo_url"].(string); !strings.HasPrefix(url, "/api/v1/public/tenants/junin/logo?v=") {
					t.Fatalf("expected logo_url in tenant payload, got %v", body["logo_url"])
				}
			}
		})
	}
}

func TestPublicTenantLogo(t *testing.T) {
	store := brandingStore()
	handler, tok, _ := newTestServer(store)
	manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

	if rec := doRequest(t, handler, http.MethodGet, "/api/v1/public/tenants/junin/logo", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before upload, got %d", rec.Code)
	}
	if rec := doBytesRequest(t, handler, http.MethodPut, "/api/v1/tenant/logo", authHeader(t, tok, manager), "image/png", pngBytes); rec.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/public/tenants/junin/logo?v=1", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	wantHeaders := map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'",
		"Cache-Control":           "public, max-age=31536000, immutable",
	}
	for header, want := range wantHeaders {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatal("logo bytes differ from the upload")
	}

	if rec := doRequest(t, handler, http.MethodDelete, "/api/v1/tenant/logo", authHeader(t, tok, manager), nil); rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d", rec.Code)
	}
	if rec := doRequest(t, handler, http.MethodGet, "/api/v1/public/tenants/junin/logo", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
	if store.tenants["t1"].LogoUpdatedAt != nil {
		t.Fatal("expected logo_updated_at cleared")
	}
}

func TestReservedSlugsAreRejected(t *testing.T) {
	t.Run("no cadastro", func(t *testing.T) {
		handler, _, _ := newTestServer(newFakeStore())
		rec := doRequest(t, handler, http.MethodPost, "/api/v1/tenants", "", map[string]string{
			"tenant_name": "Api", "slug": "api", "manager_name": "Fulano", "email": "f@x.com", "password": "s3cret123",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
	t.Run("na alteração da barbearia", func(t *testing.T) {
		store := brandingStore()
		handler, tok, _ := newTestServer(store)
		manager := domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}
		rec := doRequest(t, handler, http.MethodPatch, "/api/v1/tenant", authHeader(t, tok, manager), map[string]any{
			"name": "Junin Barber", "slug": "admin", "self_scheduling_enabled": false, "auto_confirm_appointments": false, "cancellation_window_hours": 0,
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if store.tenants["t1"].Slug != "junin" {
			t.Fatal("slug must not change")
		}
	})
}
