package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

func TestProfessionalCommissions_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	tenantID := newTestTenant(t, repo)
	serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
	beard, err := repo.CreateService(ctx, tenantID, domain.Service{Name: "Barba", DurationMinutes: 30, PriceCents: 3000})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	t.Run("grava, substitui e lista", func(t *testing.T) {
		if _, err := repo.SetProfessionalCommissions(ctx, tenantID, professionalID, []domain.ProfessionalCommission{
			{ServiceID: serviceID, Type: domain.CommissionFixed, Value: 999},
		}); err != nil {
			t.Fatalf("first set: %v", err)
		}
		if _, err := repo.SetProfessionalCommissions(ctx, tenantID, professionalID, []domain.ProfessionalCommission{
			{ServiceID: serviceID, Type: domain.CommissionPercentage, Value: 4000},
		}); err != nil {
			t.Fatalf("replace: %v", err)
		}
		got, err := repo.ListProfessionalCommissions(ctx, tenantID, professionalID)
		if err != nil || len(got) != 1 || got[0].Type != domain.CommissionPercentage || got[0].Value != 4000 {
			t.Fatalf("ListProfessionalCommissions: %v %+v", err, got)
		}
	})

	t.Run("serviço de outra barbearia não é aceito", func(t *testing.T) {
		otherTenant := newTestTenant(t, repo)
		otherService, _, _ := setupAppointmentFixtures(t, repo, otherTenant)
		_, err := repo.SetProfessionalCommissions(ctx, tenantID, professionalID, []domain.ProfessionalCommission{
			{ServiceID: otherService, Type: domain.CommissionFixed, Value: 1},
		})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		got, _ := repo.ListProfessionalCommissions(ctx, tenantID, professionalID)
		if len(got) != 1 {
			t.Fatalf("a failed replace must keep the previous rules, got %+v", got)
		}
	})

	t.Run("relatório soma só concluídos do período, com e sem regra", func(t *testing.T) {
		monday := nextMonday()
		complete := func(service string, startsAt time.Time) {
			created, err := repo.CreateAppointment(ctx, tenantID, domain.StatusConfirmed, domain.Appointment{
				CustomerID: customerID, ProfessionalID: professionalID, ServiceID: service, StartsAt: startsAt,
			})
			if err != nil {
				t.Fatalf("CreateAppointment: %v", err)
			}
			if _, err := repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusCompleted); err != nil {
				t.Fatalf("complete: %v", err)
			}
		}
		complete(serviceID, monday.Add(10*time.Hour))
		complete(beard.ID, monday.Add(11*time.Hour))
		if _, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: monday.Add(12 * time.Hour),
		}); err != nil {
			t.Fatalf("CreateAppointment (scheduled): %v", err)
		}

		lines, err := repo.ListCommissionLines(ctx, tenantID, "", monday, monday.AddDate(0, 0, 1))
		if err != nil {
			t.Fatalf("ListCommissionLines: %v", err)
		}
		report := domain.SummarizeCommissions(monday, monday.AddDate(0, 0, 1), lines)
		if len(report.Items) != 1 {
			t.Fatalf("expected one professional, got %+v", report)
		}
		item := report.Items[0]
		if item.Appointments != 2 || item.RevenueCents != 8000 || item.CommissionCents != 2000 || item.WithoutRule != 1 {
			t.Fatalf("unexpected summary: %+v", item)
		}

		only, err := repo.ListCommissionLines(ctx, tenantID, professionalID, monday, monday.AddDate(0, 0, 1))
		if err != nil || len(only) != 2 {
			t.Fatalf("filter by professional: %v %d", err, len(only))
		}
		none, err := repo.ListCommissionLines(ctx, tenantID, "00000000-0000-0000-0000-000000000000", monday, monday.AddDate(0, 0, 1))
		if err != nil || len(none) != 0 {
			t.Fatalf("unknown professional: %v %d", err, len(none))
		}
	})
}
