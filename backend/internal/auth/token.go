package auth

import (
	"errors"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type Claims struct {
	UserID         string `json:"uid"`
	Role           string `json:"role"`
	ProfessionalID string `json:"professional_id,omitempty"`
	CustomerID     string `json:"customer_id,omitempty"`
	TokenType      string `json:"type"`
	jwt.RegisteredClaims
}

type Tokenizer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenizer(secret string, accessTTL, refreshTTL time.Duration) *Tokenizer {
	return &Tokenizer{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// Secret exposes the signing key so it can also be used to sign the OAuth
// state parameter — no separate secret to configure.
func (t *Tokenizer) Secret() []byte { return t.secret }

func (t *Tokenizer) GenerateAccessToken(user domain.User) (string, error) {
	return t.generate(user, TokenTypeAccess, t.accessTTL)
}

func (t *Tokenizer) GenerateRefreshToken(user domain.User) (string, error) {
	return t.generate(user, TokenTypeRefresh, t.refreshTTL)
}

func (t *Tokenizer) generate(user domain.User, tokenType string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:         user.ID,
		Role:           user.Role,
		ProfessionalID: user.ProfessionalID,
		CustomerID:     user.CustomerID,
		TokenType:      tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

func (t *Tokenizer) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
