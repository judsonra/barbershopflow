package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func accessStore() *fakeStore {
	store := newFakeStore()
	store.tenants["t1"] = domain.Tenant{ID: "t1", Name: "Junin", Slug: "junin", Active: true}
	store.professionals["t1"] = map[string]domain.Professional{
		"p1": {ID: "p1", Name: "Barbeiro", Phone: "+5511988887777", Email: "barbeiro@example.com"},
		"p2": {ID: "p2", Name: "Sem contato"},
	}
	return store
}

var accessManager = domain.User{ID: "m1", TenantID: "t1", Role: domain.RoleManager}

func TestGrantProfessionalAccess_Channels(t *testing.T) {
	cases := []struct {
		name       string
		profID     string
		body       map[string]string
		wantCode   int
		wantSMS    int
		wantEmails int
	}{
		{"whatsapp é o padrão", "p1", map[string]string{}, http.StatusOK, 1, 0},
		{"sms", "p1", map[string]string{"channel": "sms"}, http.StatusOK, 1, 0},
		{"e-mail do cadastro", "p1", map[string]string{"channel": "email"}, http.StatusOK, 0, 1},
		{"manual não envia nada", "p1", map[string]string{"channel": "manual"}, http.StatusOK, 0, 0},
		{"e-mail sem e-mail cadastrado", "p2", map[string]string{"channel": "email"}, http.StatusBadRequest, 0, 0},
		{"e-mail inválido", "p2", map[string]string{"channel": "email", "email": "nao-e-email"}, http.StatusBadRequest, 0, 0},
		{"manual sem celular nem e-mail", "p2", map[string]string{"channel": "manual"}, http.StatusBadRequest, 0, 0},
		{"whatsapp sem celular", "p2", map[string]string{}, http.StatusBadRequest, 0, 0},
		{"canal desconhecido", "p1", map[string]string{"channel": "pombo"}, http.StatusBadRequest, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, tok, notifier := newTestServer(accessStore())
			rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/"+tc.profID+"/credentials", authHeader(t, tok, accessManager), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
			if len(notifier.sent) != tc.wantSMS || len(notifier.emails) != tc.wantEmails {
				t.Fatalf("sent sms/whatsapp=%d emails=%d, want %d/%d", len(notifier.sent), len(notifier.emails), tc.wantSMS, tc.wantEmails)
			}
		})
	}
}

func TestGrantProfessionalAccess_EmailCarriesTenantPage(t *testing.T) {
	handler, tok, notifier := newTestServer(accessStore())
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/credentials", authHeader(t, tok, accessManager), map[string]string{"channel": "email"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	sent := notifier.emails[0]
	if sent.To != "barbeiro@example.com" || sent.LoginURL != "http://localhost:5173/junin" || len(sent.Password) != 6 {
		t.Fatalf("unexpected e-mail: %+v", sent)
	}
}

// The manual password must actually work: log in with it, by phone and by
// e-mail, since the new identity gets both logins.
func TestGrantProfessionalAccess_ManualPasswordLogsIn(t *testing.T) {
	store := accessStore()
	handler, tok, _ := newTestServer(store)
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/credentials", authHeader(t, tok, accessManager), map[string]string{"channel": "manual"})
	var body map[string]string
	decodeBody(t, rec, &body)
	password := body["password"]
	if len(password) != 6 || body["login"] != "+5511988887777" || body["login_url"] != "http://localhost:5173/junin" {
		t.Fatalf("unexpected manual response: %v", body)
	}

	for _, credentials := range []map[string]string{
		{"phone": "+5511988887777", "password": password},
		{"email": "barbeiro@example.com", "password": password},
	} {
		login := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", credentials)
		if login.Code != http.StatusOK {
			t.Fatalf("login with %v: expected 200, got %d: %s", credentials, login.Code, login.Body.String())
		}
	}
}

// Asking again (forgot the password) regenerates it, and granting by e-mail
// to someone who only had a phone login adds the e-mail login.
func TestGrantProfessionalAccess_ExistingMembershipGainsEmail(t *testing.T) {
	store := accessStore()
	store.professionals["t1"]["p2"] = domain.Professional{ID: "p2", Name: "Só celular", Phone: "+5511977776666"}
	handler, tok, notifier := newTestServer(store)
	authorization := authHeader(t, tok, accessManager)
	if rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p2/credentials", authorization, map[string]string{"channel": "sms"}); rec.Code != http.StatusOK {
		t.Fatalf("first grant: %d %s", rec.Code, rec.Body.String())
	}
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p2/credentials", authorization, map[string]string{"channel": "email", "email": "novo@example.com"})
	if rec.Code != http.StatusOK {
		t.Fatalf("second grant: %d %s", rec.Code, rec.Body.String())
	}
	if len(notifier.emails) != 1 || notifier.emails[0].To != "novo@example.com" {
		t.Fatalf("expected the new password by e-mail, got %+v", notifier.emails)
	}
	login := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "novo@example.com", "password": notifier.emails[0].Password})
	if login.Code != http.StatusOK {
		t.Fatalf("e-mail login after grant: %d %s", login.Code, login.Body.String())
	}
	if store.professionals["t1"]["p2"].Email != "novo@example.com" {
		t.Fatal("expected the professional register to get the e-mail")
	}
}

func TestGrantProfessionalAccess_OnlyManagers(t *testing.T) {
	for _, role := range []string{domain.RoleProfessional, domain.RoleClient} {
		handler, tok, _ := newTestServer(accessStore())
		user := domain.User{ID: "u1", TenantID: "t1", Role: role, ProfessionalID: "p1"}
		rec := doRequest(t, handler, http.MethodPost, "/api/v1/professionals/p1/credentials", authHeader(t, tok, user), map[string]string{"channel": "manual"})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: expected 403, got %d", role, rec.Code)
		}
	}
}

func TestChangePassword(t *testing.T) {
	setup := func(t *testing.T) (http.Handler, string, *fakeStore) {
		store := newFakeStore()
		user := seedManagerWithPassword(store, "t1", "i1", "m1", "gestor@example.com", "123456")
		handler, tok, _ := newTestServer(store)
		return handler, authHeader(t, tok, user), store
	}

	t.Run("troca e passa a entrar com a nova", func(t *testing.T) {
		handler, authorization, _ := setup(t)
		rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/password", authorization, map[string]string{"current_password": "123456", "new_password": "minha-senha-nova"})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "gestor@example.com", "password": "123456"}).Code != http.StatusUnauthorized {
			t.Fatal("old password must stop working")
		}
		if doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "gestor@example.com", "password": "minha-senha-nova"}).Code != http.StatusOK {
			t.Fatal("new password must work")
		}
	})

	cases := []struct {
		name, current, next string
		wantCode            int
	}{
		{"senha atual errada", "errada", "minha-senha-nova", http.StatusBadRequest},
		{"nova senha curta", "123456", "curta", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, authorization, _ := setup(t)
			rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/password", authorization, map[string]string{"current_password": tc.current, "new_password": tc.next})
			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("sem login", func(t *testing.T) {
		handler, _, _ := setup(t)
		rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/password", "", map[string]string{"current_password": "123456", "new_password": "minha-senha-nova"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
