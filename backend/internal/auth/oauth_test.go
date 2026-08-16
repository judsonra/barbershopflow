package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestSocialProvider_Configured(t *testing.T) {
	configured := NewGoogleProvider("client-id", "client-secret", "http://localhost/callback")
	if !configured.Configured() {
		t.Fatal("expected a provider with client id/secret to be configured")
	}
	unconfigured := NewGoogleProvider("", "", "")
	if unconfigured.Configured() {
		t.Fatal("expected a provider without credentials to be unconfigured")
	}
	var nilProvider *SocialProvider
	if nilProvider.Configured() {
		t.Fatal("expected a nil provider to be unconfigured")
	}
}

func TestSocialProvider_AuthCodeURLIncludesState(t *testing.T) {
	provider := NewFacebookProvider("client-id", "client-secret", "http://localhost/callback")
	url := provider.AuthCodeURL("state-123")
	if !strings.Contains(url, "state=state-123") {
		t.Fatalf("expected the auth URL to carry the state, got %q", url)
	}
}

// TestSocialProvider_ExchangeFetchesUserInfoWithToken exercises the full
// Exchange method honestly: a fake token endpoint stands in for the
// provider's real one, and fetchUserInfo (swapped, per SocialProvider's
// injectable func field) uses getJSON against a second fake server to prove
// the *http.Client it receives really does carry the exchanged token.
func TestSocialProvider_ExchangeFetchesUserInfoWithToken(t *testing.T) {
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("expected the userinfo request to carry the exchanged token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": "google-123", "email": "user@x.com", "name": "User"})
	}))
	defer userInfoServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	defer tokenServer.Close()

	provider := &SocialProvider{
		config: &oauth2.Config{
			ClientID: "id", ClientSecret: "secret",
			Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL, AuthStyle: oauth2.AuthStyleInParams},
		},
		fetchUserInfo: func(ctx context.Context, client *http.Client) (OAuthUserInfo, error) {
			var body struct{ Sub, Email, Name string }
			if err := getJSON(ctx, client, userInfoServer.URL, &body); err != nil {
				return OAuthUserInfo{}, err
			}
			return OAuthUserInfo{ProviderID: body.Sub, Email: body.Email, Name: body.Name}, nil
		},
	}

	info, err := provider.Exchange(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if info.ProviderID != "google-123" || info.Email != "user@x.com" {
		t.Fatalf("unexpected user info: %+v", info)
	}
}

func TestSocialProvider_ExchangePropagatesTokenEndpointError(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	provider := &SocialProvider{
		config: &oauth2.Config{
			ClientID: "id", ClientSecret: "secret",
			Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL, AuthStyle: oauth2.AuthStyleInParams},
		},
		fetchUserInfo: func(context.Context, *http.Client) (OAuthUserInfo, error) {
			t.Fatal("fetchUserInfo should not be called when the token exchange itself fails")
			return OAuthUserInfo{}, nil
		},
	}

	if _, err := provider.Exchange(context.Background(), "auth-code"); err == nil {
		t.Fatal("expected an error when the token endpoint rejects the exchange")
	}
}

func TestGetJSON_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "ok"})
	}))
	defer server.Close()

	var body struct{ Name string }
	if err := getJSON(context.Background(), server.Client(), server.URL, &body); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if body.Name != "ok" {
		t.Fatalf("expected decoded body, got %+v", body)
	}
}

func TestGetJSON_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	defer server.Close()

	var body struct{}
	if err := getJSON(context.Background(), server.Client(), server.URL, &body); err == nil {
		t.Fatal("expected an error for a non-2xx status")
	}
}
