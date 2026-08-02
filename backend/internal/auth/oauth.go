package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

type OAuthUserInfo struct {
	ProviderID string
	Email      string
	Name       string
}

type SocialProvider struct {
	config        *oauth2.Config
	fetchUserInfo func(ctx context.Context, client *http.Client) (OAuthUserInfo, error)
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *SocialProvider {
	return &SocialProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		fetchUserInfo: fetchGoogleUserInfo,
	}
}

func NewFacebookProvider(clientID, clientSecret, redirectURL string) *SocialProvider {
	return &SocialProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"email", "public_profile"},
			Endpoint:     facebook.Endpoint,
		},
		fetchUserInfo: fetchFacebookUserInfo,
	}
}

func (p *SocialProvider) Configured() bool {
	return p != nil && p.config.ClientID != "" && p.config.ClientSecret != ""
}

func (p *SocialProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *SocialProvider) Exchange(ctx context.Context, code string) (OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return OAuthUserInfo{}, err
	}
	return p.fetchUserInfo(ctx, p.config.Client(ctx, token))
}

func fetchGoogleUserInfo(ctx context.Context, client *http.Client) (OAuthUserInfo, error) {
	var body struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := getJSON(ctx, client, "https://www.googleapis.com/oauth2/v3/userinfo", &body); err != nil {
		return OAuthUserInfo{}, err
	}
	if body.Email == "" {
		return OAuthUserInfo{}, errors.New("google account has no e-mail")
	}
	return OAuthUserInfo{ProviderID: body.Sub, Email: body.Email, Name: body.Name}, nil
}

func fetchFacebookUserInfo(ctx context.Context, client *http.Client) (OAuthUserInfo, error) {
	var body struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := getJSON(ctx, client, "https://graph.facebook.com/me?fields=id,name,email", &body); err != nil {
		return OAuthUserInfo{}, err
	}
	if body.Email == "" {
		return OAuthUserInfo{}, errors.New("facebook account has no e-mail")
	}
	return OAuthUserInfo{ProviderID: body.ID, Email: body.Email, Name: body.Name}, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// State signing: a self-verifying CSRF token for the OAuth redirect flow,
// so no server-side session storage is needed. It also carries an optional
// payload (the tenant slug a brand-new client is signing up into) through
// the round trip to the provider and back.
// Format: <nonce>.<unixTime>.<base64url(payload)>.<hmac>.
var ErrInvalidState = errors.New("invalid or expired oauth state")

func SignState(secret []byte, payload string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	encodedNonce := base64.RawURLEncoding.EncodeToString(nonce)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signStatePayload(secret, encodedNonce, ts, encodedPayload)
	return encodedNonce + "." + ts + "." + encodedPayload + "." + sig, nil
}

func VerifyState(secret []byte, state string, maxAge time.Duration) (string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 4 {
		return "", ErrInvalidState
	}
	encodedNonce, ts, encodedPayload, sig := parts[0], parts[1], parts[2], parts[3]
	expected := signStatePayload(secret, encodedNonce, ts, encodedPayload)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return "", ErrInvalidState
	}
	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", ErrInvalidState
	}
	if time.Since(time.Unix(seconds, 0)) > maxAge {
		return "", ErrInvalidState
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", ErrInvalidState
	}
	return string(payload), nil
}

func signStatePayload(secret []byte, encodedNonce, ts, encodedPayload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encodedNonce + "." + ts + "." + encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
