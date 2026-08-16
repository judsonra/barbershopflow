package http

import (
	"net/http"
	"testing"

	"github.com/example/barberflow/backend/internal/auth"
	"github.com/example/barberflow/backend/internal/domain"
)

func seedManagerWithPassword(store *fakeStore, tenantID, identityID, membershipID, email, password string) domain.User {
	hash, _ := auth.HashPassword(password)
	store.identities[identityID] = domain.Identity{ID: identityID, Name: "Gestor", Email: email, PasswordHash: hash}
	user := domain.User{ID: membershipID, IdentityID: identityID, TenantID: tenantID, Name: "Gestor", Email: email, PasswordHash: hash, Role: domain.RoleManager, Active: true}
	store.memberships[membershipID] = user
	store.membershipOptions[identityID] = append(store.membershipOptions[identityID], domain.MembershipOption{
		MembershipID: membershipID, IdentityID: identityID, TenantID: tenantID, Role: domain.RoleManager,
	})
	return user
}

func TestLogin_EmailHappyPath(t *testing.T) {
	store := newFakeStore()
	seedManagerWithPassword(store, "t1", "id-1", "m-1", "gestor@x.com", "s3cret123")
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "gestor@x.com", "password": "s3cret123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	decodeBody(t, rec, &body)
	if body.AccessToken == "" {
		t.Fatal("expected an access_token in the response")
	}
}

func TestLogin_EmailWrongPassword(t *testing.T) {
	store := newFakeStore()
	seedManagerWithPassword(store, "t1", "id-1", "m-1", "gestor@x.com", "s3cret123")
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "gestor@x.com", "password": "wrong",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials, got %q", code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "nobody@x.com", "password": "whatever",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown e-mail, got %d", rec.Code)
	}
}

func TestLogin_MultipleMembershipsOffersChoice(t *testing.T) {
	store := newFakeStore()
	hash, _ := auth.HashPassword("s3cret123")
	store.identities["id-1"] = domain.Identity{ID: "id-1", Name: "Cliente", Email: "cliente@x.com", PasswordHash: hash}
	store.memberships["m-1"] = domain.User{ID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient}
	store.memberships["m-2"] = domain.User{ID: "m-2", IdentityID: "id-1", TenantID: "t2", Role: domain.RoleClient}
	store.membershipOptions["id-1"] = []domain.MembershipOption{
		{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient},
		{MembershipID: "m-2", IdentityID: "id-1", TenantID: "t2", Role: domain.RoleClient},
	}
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "cliente@x.com", "password": "s3cret123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		PreauthToken string                       `json:"preauth_token"`
		Memberships  []domain.MembershipOption `json:"memberships"`
	}
	decodeBody(t, rec, &body)
	if body.PreauthToken == "" || len(body.Memberships) != 2 {
		t.Fatalf("expected a preauth token and 2 membership choices, got %+v", body)
	}
}

func TestLogin_PhoneHappyPath(t *testing.T) {
	store := newFakeStore()
	hash, _ := auth.HashPassword("123456")
	store.identities["id-1"] = domain.Identity{ID: "id-1", Name: "Cliente", Phone: "+5511999990000", PasswordHash: hash}
	store.memberships["m-1"] = domain.User{ID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient, Active: true}
	store.membershipOptions["id-1"] = []domain.MembershipOption{{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient}}
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"phone": "+5511999990000", "password": "123456",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_PhoneLocksAfterMaxAttempts(t *testing.T) {
	store := newFakeStore()
	hash, _ := auth.HashPassword("123456")
	store.identities["id-1"] = domain.Identity{ID: "id-1", Name: "Cliente", Phone: "+5511999990000", PasswordHash: hash}
	store.memberships["m-1"] = domain.User{ID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient, Active: true}
	store.membershipOptions["id-1"] = []domain.MembershipOption{{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient}}
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"phone": "+5511999990000", "password": "wrong"})
	for i := 1; i < domain.MaxLoginAttempts; i++ {
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 while attempts remain, got %d", i, rec.Code)
		}
		rec = doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"phone": "+5511999990000", "password": "wrong"})
	}
	if rec.Code != http.StatusLocked {
		t.Fatalf("expected 423 locked after %d failed attempts, got %d: %s", domain.MaxLoginAttempts, rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "account_locked" {
		t.Fatalf("expected account_locked code, got %q", code)
	}

	// A subsequent attempt, even with the right password, stays locked.
	rec = doRequest(t, handler, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"phone": "+5511999990000", "password": "123456"})
	if rec.Code != http.StatusLocked {
		t.Fatalf("expected still-locked account to reject even the correct password, got %d", rec.Code)
	}
}

func TestRefresh_HappyPath(t *testing.T) {
	store := newFakeStore()
	user := domain.User{ID: "m-1", TenantID: "t1", Role: domain.RoleManager, Active: true}
	store.memberships["m-1"] = user
	handler, tok, _ := newTestServer(store)
	refresh, err := tok.GenerateRefreshToken(user)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRefresh_RejectsAccessTokenAsRefresh(t *testing.T) {
	store := newFakeStore()
	user := domain.User{ID: "m-1", TenantID: "t1", Role: domain.RoleManager, Active: true}
	store.memberships["m-1"] = user
	handler, tok, _ := newTestServer(store)
	access, _ := tok.GenerateAccessToken(user)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": access})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when an access token is used as a refresh token, got %d", rec.Code)
	}
}

func TestRefresh_InactiveUser(t *testing.T) {
	store := newFakeStore()
	user := domain.User{ID: "m-1", TenantID: "t1", Role: domain.RoleManager, Active: false}
	store.memberships["m-1"] = user
	handler, tok, _ := newTestServer(store)
	refresh, _ := tok.GenerateRefreshToken(user)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{"refresh_token": refresh})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an inactive membership, got %d", rec.Code)
	}
}

func TestRecoverPhone_KnownNumberSendsPassword(t *testing.T) {
	store := newFakeStore()
	store.identities["id-1"] = domain.Identity{ID: "id-1", Phone: "+5511999990000"}
	handler, _, notifier := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/recover", "", map[string]string{"phone": "+5511999990000"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(notifier.sent) != 1 || notifier.sent[0].Phone != "+5511999990000" {
		t.Fatalf("expected the notifier to receive one SendPassword call, got %+v", notifier.sent)
	}
}

func TestRecoverPhone_UnknownNumberStillReturns200(t *testing.T) {
	store := newFakeStore()
	handler, _, notifier := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/recover", "", map[string]string{"phone": "+5511999990000"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even for an unregistered phone (no enumeration), got %d", rec.Code)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("expected no notification for an unregistered phone, got %+v", notifier.sent)
	}
}

func TestRecoverPhone_MissingPhone(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/recover", "", map[string]string{"phone": ""})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing phone, got %d", rec.Code)
	}
}

func TestSelectMembership_HappyPath(t *testing.T) {
	store := newFakeStore()
	store.memberships["m-1"] = domain.User{ID: "m-1", IdentityID: "id-1", TenantID: "t1", Role: domain.RoleClient}
	store.membershipOptions["id-1"] = []domain.MembershipOption{{MembershipID: "m-1", IdentityID: "id-1", TenantID: "t1"}}
	handler, tok, _ := newTestServer(store)
	preauth, err := auth.SignState(tok.Secret(), "id-1")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/select-membership", "", map[string]string{
		"preauth_token": preauth, "membership_id": "m-1",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSelectMembership_TamperedPreauthToken(t *testing.T) {
	store := newFakeStore()
	handler, tok, _ := newTestServer(store)
	preauth, err := auth.SignState(tok.Secret(), "id-1")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}

	// selectMembership hard-codes a 5 minute maxAge, which can't be
	// shortened from a test; a tampered signature exercises the same
	// invalid_token rejection path without a real 5 minute sleep.
	rec := doRequest(t, handler, http.MethodPost, "/api/v1/auth/select-membership", "", map[string]string{
		"preauth_token": preauth + "tampered", "membership_id": "m-1",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an invalid preauth token, got %d", rec.Code)
	}
}

func TestMe_HappyPath(t *testing.T) {
	store := newFakeStore()
	user := domain.User{ID: "m-1", TenantID: "t1", Role: domain.RoleManager}
	store.memberships["m-1"] = user
	handler, tok, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/auth/me", authHeader(t, tok, user), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/auth/me", "", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestOAuthStart_NotConfiguredReturns501(t *testing.T) {
	store := newFakeStore()
	handler, _, _ := newTestServer(store)

	rec := doRequest(t, handler, http.MethodGet, "/api/v1/auth/google/start", "", nil)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 when Google OAuth isn't configured, got %d", rec.Code)
	}
}
