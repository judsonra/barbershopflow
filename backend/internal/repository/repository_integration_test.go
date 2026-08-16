package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

// setupAppointmentFixtures creates a service, professional and customer
// under tenantID, all active by default, ready for CreateAppointment.
func setupAppointmentFixtures(t *testing.T, repo *Repository, tenantID string) (serviceID, professionalID, customerID string) {
	t.Helper()
	ctx := context.Background()
	service, err := repo.CreateService(ctx, tenantID, domain.Service{Name: "Corte", DurationMinutes: 30, PriceCents: 5000})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	professional, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "Barbeiro"})
	if err != nil {
		t.Fatalf("create professional: %v", err)
	}
	customer, err := repo.CreateCustomer(ctx, tenantID, domain.Customer{Name: "Cliente"})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	return service.ID, professional.ID, customer.ID
}

// nextMonday returns midnight UTC of the next Monday on/after a fixed,
// far-future base date, so working-hours/holiday fixtures below have a
// deterministic weekday to key off without depending on when the test runs.
func nextMonday() time.Time {
	day := time.Date(2031, time.March, 1, 0, 0, 0, 0, time.UTC)
	for day.Weekday() != time.Monday {
		day = day.AddDate(0, 0, 1)
	}
	return day
}

func TestCreateAppointment_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	t.Run("happy path, no overrides", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)

		created, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID,
			StartsAt: nextMonday().Add(10 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateAppointment: %v", err)
		}
		if created.PriceCents != 5000 {
			t.Errorf("expected the service's own price 5000, got %d", created.PriceCents)
		}
		if !created.EndsAt.Equal(created.StartsAt.Add(30 * time.Minute)) {
			t.Errorf("expected EndsAt = StartsAt + 30min (service duration), got %v / %v", created.StartsAt, created.EndsAt)
		}
	})

	t.Run("happy path, with a price/duration override", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		overridePrice := int64(9000)
		overrideDuration := 45
		if _, err := repo.SetProfessionalServices(ctx, tenantID, professionalID, []domain.ProfessionalService{
			{ServiceID: serviceID, PriceCentsOverride: &overridePrice, DurationMinutesOverride: &overrideDuration},
		}); err != nil {
			t.Fatalf("SetProfessionalServices: %v", err)
		}

		created, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID,
			StartsAt: nextMonday().Add(10 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateAppointment: %v", err)
		}
		if created.PriceCents != overridePrice {
			t.Errorf("expected overridden price %d, got %d", overridePrice, created.PriceCents)
		}
		if !created.EndsAt.Equal(created.StartsAt.Add(45 * time.Minute)) {
			t.Errorf("expected EndsAt = StartsAt + 45min (overridden duration), got %v / %v", created.StartsAt, created.EndsAt)
		}
	})

	t.Run("service not in the professional's specialties", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		offeredServiceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		otherService, err := repo.CreateService(ctx, tenantID, domain.Service{Name: "Barba", DurationMinutes: 20, PriceCents: 3000})
		if err != nil {
			t.Fatalf("create other service: %v", err)
		}
		if _, err := repo.SetProfessionalServices(ctx, tenantID, professionalID, []domain.ProfessionalService{{ServiceID: offeredServiceID}}); err != nil {
			t.Fatalf("SetProfessionalServices: %v", err)
		}

		_, err = repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: otherService.ID,
			StartsAt: nextMonday().Add(10 * time.Hour),
		})
		if !errors.Is(err, domain.ErrServiceNotOffered) {
			t.Fatalf("expected ErrServiceNotOffered, got %v", err)
		}
	})

	t.Run("outside the professional's working hours", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		monday := nextMonday()
		if _, err := repo.SetProfessionalSchedule(ctx, tenantID, professionalID, []domain.ScheduleEntry{
			{Weekday: int(monday.Weekday()), StartMinute: 540, EndMinute: 720}, // 09:00-12:00
		}); err != nil {
			t.Fatalf("SetProfessionalSchedule: %v", err)
		}

		_, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID,
			StartsAt: monday.Add(20 * time.Hour), // 20:00, well outside 09:00-12:00
		})
		if !errors.Is(err, domain.ErrOutsideWorkingHours) {
			t.Fatalf("expected ErrOutsideWorkingHours, got %v", err)
		}
	})

	t.Run("within working hours succeeds", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		monday := nextMonday()
		if _, err := repo.SetProfessionalSchedule(ctx, tenantID, professionalID, []domain.ScheduleEntry{
			{Weekday: int(monday.Weekday()), StartMinute: 540, EndMinute: 720},
		}); err != nil {
			t.Fatalf("SetProfessionalSchedule: %v", err)
		}

		_, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID,
			StartsAt: monday.Add(10 * time.Hour), // 10:00, inside 09:00-12:00
		})
		if err != nil {
			t.Fatalf("expected success inside working hours, got %v", err)
		}
	})

	t.Run("overlaps a time-off block", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		start := nextMonday().Add(10 * time.Hour)
		if _, err := repo.CreateTimeOff(ctx, tenantID, professionalID, domain.TimeOff{
			StartsAt: start.Add(-time.Hour), EndsAt: start.Add(time.Hour), Reason: "Viagem",
		}); err != nil {
			t.Fatalf("CreateTimeOff: %v", err)
		}

		_, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: start,
		})
		if !errors.Is(err, domain.ErrTimeBlocked) {
			t.Fatalf("expected ErrTimeBlocked, got %v", err)
		}
	})

	t.Run("falls on a tenant holiday", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		monday := nextMonday()
		if _, err := repo.CreateHoliday(ctx, tenantID, domain.Holiday{Date: monday.Format("2006-01-02"), Name: "Feriado"}); err != nil {
			t.Fatalf("CreateHoliday: %v", err)
		}

		_, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: monday.Add(10 * time.Hour),
		})
		if !errors.Is(err, domain.ErrHolidayBlocked) {
			t.Fatalf("expected ErrHolidayBlocked, got %v", err)
		}
	})

	t.Run("conflicts with an existing appointment for the same professional", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		otherCustomer, err := repo.CreateCustomer(ctx, tenantID, domain.Customer{Name: "Outro Cliente"})
		if err != nil {
			t.Fatalf("create other customer: %v", err)
		}
		start := nextMonday().Add(10 * time.Hour)

		if _, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: start,
		}); err != nil {
			t.Fatalf("first CreateAppointment: %v", err)
		}

		_, err = repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
			CustomerID: otherCustomer.ID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: start,
		})
		if !errors.Is(err, domain.ErrScheduleConflict) {
			t.Fatalf("expected ErrScheduleConflict for the overlapping booking, got %v", err)
		}
	})

	t.Run("missing service, professional or customer", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)
		start := nextMonday().Add(10 * time.Hour)

		const missingID = "00000000-0000-0000-0000-000000000000" // valid UUID, no matching row
		cases := []struct {
			name                                  string
			serviceID, professionalID, customerID string
		}{
			{"missing service", missingID, professionalID, customerID},
			{"missing professional", serviceID, missingID, customerID},
			{"missing customer", serviceID, professionalID, missingID},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
					CustomerID: tc.customerID, ProfessionalID: tc.professionalID, ServiceID: tc.serviceID, StartsAt: start,
				})
				if !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("expected ErrNotFound, got %v", err)
				}
			})
		}
	})
}

func TestUpdateAppointmentStatus_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	tenantID := newTestTenant(t, repo)
	serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)

	created, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
		CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: nextMonday().Add(10 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	confirmed, err := repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusConfirmed)
	if err != nil {
		t.Fatalf("scheduled -> confirmed: %v", err)
	}
	if confirmed.Status != domain.StatusConfirmed {
		t.Fatalf("expected status confirmed, got %q", confirmed.Status)
	}

	completed, err := repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusCompleted)
	if err != nil {
		t.Fatalf("confirmed -> completed: %v", err)
	}
	if completed.Status != domain.StatusCompleted {
		t.Fatalf("expected status completed, got %q", completed.Status)
	}

	_, err = repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusConfirmed)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition going from completed back to confirmed, got %v", err)
	}
}

func TestGetReport_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	tenantID := newTestTenant(t, repo)
	serviceID, professionalID, customerID := setupAppointmentFixtures(t, repo, tenantID)

	monday := nextMonday()
	from := monday
	to := monday.AddDate(0, 0, 1)

	// Full-day availability on exactly the one weekday covered by [from,to),
	// so available minutes for the period is deterministic: 1440.
	if _, err := repo.SetProfessionalSchedule(ctx, tenantID, professionalID, []domain.ScheduleEntry{
		{Weekday: int(monday.Weekday()), StartMinute: 0, EndMinute: 1440},
	}); err != nil {
		t.Fatalf("SetProfessionalSchedule: %v", err)
	}

	created, err := repo.CreateAppointment(ctx, tenantID, domain.StatusScheduled, domain.Appointment{
		CustomerID: customerID, ProfessionalID: professionalID, ServiceID: serviceID, StartsAt: monday.Add(10 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if _, err := repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusConfirmed); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := repo.UpdateAppointmentStatus(ctx, tenantID, created.ID, domain.StatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}

	report, err := repo.GetReport(ctx, tenantID, from, to)
	if err != nil {
		t.Fatalf("GetReport: %v", err)
	}

	if len(report.Occupancy.ByProfessional) != 1 {
		t.Fatalf("expected 1 professional in occupancy, got %d", len(report.Occupancy.ByProfessional))
	}
	occ := report.Occupancy.ByProfessional[0]
	if occ.AvailableMinutes != 1440 {
		t.Errorf("expected 1440 available minutes, got %d", occ.AvailableMinutes)
	}
	if occ.BookedMinutes != 30 {
		t.Errorf("expected 30 booked minutes (the service's duration), got %d", occ.BookedMinutes)
	}

	if report.Revenue.TotalCents != 5000 {
		t.Errorf("expected revenue 5000 (the one completed appointment), got %d", report.Revenue.TotalCents)
	}

	if report.Retention.TotalCustomers != 1 {
		t.Errorf("expected 1 customer with a completed appointment in the period, got %d", report.Retention.TotalCustomers)
	}
	if report.Retention.ReturningCustomers != 0 {
		t.Errorf("expected 0 returning customers (no prior completed appointment), got %d", report.Retention.ReturningCustomers)
	}
}

func TestUniqueViolationMapping_Integration(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	t.Run("duplicate professional e-mail within the same tenant", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		if _, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "A", Email: "dup@x.com"}); err != nil {
			t.Fatalf("create first professional: %v", err)
		}
		_, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "B", Email: "dup@x.com"})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected ErrConflict for a duplicate e-mail, got %v", err)
		}
	})

	t.Run("duplicate professional CPF within the same tenant", func(t *testing.T) {
		tenantID := newTestTenant(t, repo)
		if _, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "A", CPF: "11144477735"}); err != nil {
			t.Fatalf("create first professional: %v", err)
		}
		_, err := repo.CreateProfessional(ctx, tenantID, domain.Professional{Name: "B", CPF: "11144477735"})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected ErrConflict for a duplicate CPF, got %v", err)
		}
	})

	t.Run("the same e-mail across two different tenants is not a conflict", func(t *testing.T) {
		tenantA := newTestTenant(t, repo)
		tenantB := newTestTenant(t, repo)
		if _, err := repo.CreateProfessional(ctx, tenantA, domain.Professional{Name: "A", Email: "shared@x.com"}); err != nil {
			t.Fatalf("create in tenant A: %v", err)
		}
		if _, err := repo.CreateProfessional(ctx, tenantB, domain.Professional{Name: "A", Email: "shared@x.com"}); err != nil {
			t.Fatalf("expected the same e-mail to be fine in a different tenant, got %v", err)
		}
	})

	t.Run("duplicate tenant name", func(t *testing.T) {
		suffix := uniqueSuffix()
		name := "Barbearia Duplicada " + suffix
		if _, _, err := repo.CreateTenantWithManager(ctx, name, "slug-a-"+suffix, "Gestor", "a-"+suffix+"@x.com", "hash"); err != nil {
			t.Fatalf("create first tenant: %v", err)
		}
		_, _, err := repo.CreateTenantWithManager(ctx, name, "slug-b-"+suffix, "Gestor", "b-"+suffix+"@x.com", "hash")
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected ErrConflict for a duplicate tenant name, got %v", err)
		}
	})
}
