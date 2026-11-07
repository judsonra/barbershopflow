package repository

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestTenantBranding_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	tenantID := newTestTenant(t, repo)
	tenant, err := repo.GetTenantByID(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetTenantByID: %v", err)
	}

	t.Run("barbearia nova começa na paleta clássica e sem logo", func(t *testing.T) {
		if tenant.ColorScheme != domain.DefaultColorScheme || tenant.LogoUpdatedAt != nil {
			t.Fatalf("unexpected defaults: scheme=%q logo=%v", tenant.ColorScheme, tenant.LogoUpdatedAt)
		}
	})

	t.Run("troca a paleta", func(t *testing.T) {
		got, err := repo.UpdateTenantBranding(ctx, tenantID, "vinho")
		if err != nil || got.ColorScheme != "vinho" {
			t.Fatalf("UpdateTenantBranding: %v %+v", err, got)
		}
	})

	t.Run("envia, substitui e lê a logo pelo slug", func(t *testing.T) {
		first, err := repo.SetTenantLogo(ctx, tenantID, "image/png", []byte("first"))
		if err != nil || first.LogoUpdatedAt == nil {
			t.Fatalf("SetTenantLogo: %v %+v", err, first)
		}
		if _, err := repo.SetTenantLogo(ctx, tenantID, "image/webp", []byte("second")); err != nil {
			t.Fatalf("SetTenantLogo (replace): %v", err)
		}
		contentType, data, err := repo.GetTenantLogo(ctx, tenant.Slug)
		if err != nil || contentType != "image/webp" || !bytes.Equal(data, []byte("second")) {
			t.Fatalf("GetTenantLogo: %v %q %q", err, contentType, data)
		}
	})

	t.Run("remove a logo", func(t *testing.T) {
		got, err := repo.DeleteTenantLogo(ctx, tenantID)
		if err != nil || got.LogoUpdatedAt != nil {
			t.Fatalf("DeleteTenantLogo: %v %+v", err, got)
		}
		if _, _, err := repo.GetTenantLogo(ctx, tenant.Slug); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
		if _, err := repo.DeleteTenantLogo(ctx, tenantID); err != nil {
			t.Fatalf("second delete should be a no-op, got %v", err)
		}
	})

	t.Run("barbearia inativa não serve logo", func(t *testing.T) {
		if _, err := repo.SetTenantLogo(ctx, tenantID, "image/png", []byte("x")); err != nil {
			t.Fatalf("SetTenantLogo: %v", err)
		}
		if _, err := testPool.Exec(ctx, `UPDATE tenants SET active=false WHERE id=$1`, tenantID); err != nil {
			t.Fatalf("deactivate: %v", err)
		}
		if _, _, err := repo.GetTenantLogo(ctx, tenant.Slug); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for inactive tenant, got %v", err)
		}
	})

	t.Run("barbearia desconhecida", func(t *testing.T) {
		if _, err := repo.UpdateTenantBranding(ctx, "00000000-0000-0000-0000-000000000000", "vinho"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
