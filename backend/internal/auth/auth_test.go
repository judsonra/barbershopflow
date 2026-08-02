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
