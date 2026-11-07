package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestProfessionalAccessEmails_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	tenantID := newTestTenant(t, repo)
	suffix := uniqueSuffix()

	professional, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "Barbeiro " + suffix})
	if err != nil {
		t.Fatalf("CreateProfessional: %v", err)
	}
	other, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "Outro " + suffix, Email: "outro-" + suffix + "@example.com"})
	if err != nil {
		t.Fatalf("CreateProfessional (other): %v", err)
	}

	t.Run("grava o e-mail do profissional", func(t *testing.T) {
		email := "barbeiro-" + suffix + "@example.com"
		if err := repo.UpdateProfessionalEmail(ctx, tenantID, professional.ID, email); err != nil {
			t.Fatalf("UpdateProfessionalEmail: %v", err)
		}
		got, err := repo.GetProfessionalByID(ctx, tenantID, professional.ID)
		if err != nil || got.Email != email {
			t.Fatalf("expected %q, got %q (%v)", email, got.Email, err)
		}
	})

	t.Run("e-mail repetido na barbearia é conflito", func(t *testing.T) {
		if err := repo.UpdateProfessionalEmail(ctx, tenantID, professional.ID, other.Email); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})

	t.Run("identidade só com celular ganha login por e-mail", func(t *testing.T) {
		user, err := repo.CreateIdentityWithMembership(ctx, "Barbeiro", "", fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1_000_000_000), "hash", "", "", tenantID, domain.RoleProfessional, professional.ID, "")
		if err != nil {
			t.Fatalf("CreateIdentityWithMembership: %v", err)
		}
		email := "login-" + suffix + "@example.com"
		if err := repo.SetIdentityEmail(ctx, user.IdentityID, email); err != nil {
			t.Fatalf("SetIdentityEmail: %v", err)
		}
		identity, err := repo.FindIdentityByEmail(ctx, email)
		if err != nil || identity.ID != user.IdentityID {
			t.Fatalf("FindIdentityByEmail: %v %+v", err, identity)
		}
	})
}
