package auth

import (
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !CheckPassword(hash, "s3cret") {
		t.Fatal("expected password to match its hash")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to not match")
	}
}

func TestTokenizerAccessAndRefresh(t *testing.T) {
	tokenizer := NewTokenizer("test-secret", time.Minute, time.Hour)
	user := domain.User{ID: "user-1", Role: domain.RoleProfessional, ProfessionalID: "prof-1"}

	access, err := tokenizer.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	claims, err := tokenizer.Parse(access)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.TokenType != TokenTypeAccess || claims.UserID != user.ID || claims.ProfessionalID != user.ProfessionalID {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	refresh, err := tokenizer.GenerateRefreshToken(user)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	refreshClaims, err := tokenizer.Parse(refresh)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.TokenType != TokenTypeRefresh {
		t.Fatalf("expected refresh token type, got %q", refreshClaims.TokenType)
	}
}

func TestTokenizerRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenizer("secret-a", time.Minute, time.Hour)
	verifier := NewTokenizer("secret-b", time.Minute, time.Hour)

	token, err := issuer.GenerateAccessToken(domain.User{ID: "user-1", Role: domain.RoleManager})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}

func TestTokenizerRejectsExpiredToken(t *testing.T) {
	tokenizer := NewTokenizer("test-secret", -time.Minute, time.Hour)
	token, err := tokenizer.GenerateAccessToken(domain.User{ID: "user-1", Role: domain.RoleManager})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := tokenizer.Parse(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestSignAndVerifyState(t *testing.T) {
	secret := []byte("state-secret")
	state, err := SignState(secret, "acme-barbearia")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	payload, err := VerifyState(secret, state, time.Minute)
	if err != nil {
		t.Fatalf("expected valid state to verify, got %v", err)
	}
	if payload != "acme-barbearia" {
		t.Fatalf("expected payload to round-trip, got %q", payload)
	}
}

func TestVerifyStateRejectsTamperedOrWrongSecret(t *testing.T) {
	secret := []byte("state-secret")
	state, err := SignState(secret, "")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	if _, err := VerifyState([]byte("other-secret"), state, time.Minute); err == nil {
		t.Fatal("expected state signed with a different secret to be rejected")
	}
	if _, err := VerifyState(secret, state+"tampered", time.Minute); err == nil {
		t.Fatal("expected tampered state to be rejected")
	}
	if _, err := VerifyState(secret, "not.a.validstate", time.Minute); err == nil {
		t.Fatal("expected malformed state to be rejected")
	}
}

func TestVerifyStateRejectsExpired(t *testing.T) {
	secret := []byte("state-secret")
	state, err := SignState(secret, "")
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	if _, err := VerifyState(secret, state, -time.Second); err == nil {
		t.Fatal("expected expired state to be rejected")
	}
}

func TestGenerateTempPasswordIsSixDigits(t *testing.T) {
	password, err := GenerateTempPassword()
	if err != nil {
		t.Fatalf("generate temp password: %v", err)
	}
	if len(password) != 6 {
		t.Fatalf("expected a 6-digit password, got %q", password)
	}
	for _, r := range password {
		if r < '0' || r > '9' {
			t.Fatalf("expected only digits, got %q", password)
		}
	}
}
