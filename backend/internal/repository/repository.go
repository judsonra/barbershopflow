package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListServices(ctx context.Context) ([]domain.Service, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, duration_minutes, price_cents, active, created_at FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Service{}
	for rows.Next() {
		var item domain.Service
		if err := rows.Scan(&item.ID, &item.Name, &item.DurationMinutes, &item.PriceCents, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateService(ctx context.Context, item domain.Service) (domain.Service, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO services(name, duration_minutes, price_cents) VALUES($1,$2,$3) RETURNING id, active, created_at`,
		item.Name, item.DurationMinutes, item.PriceCents).Scan(&item.ID, &item.Active, &item.CreatedAt)
	return item, err
}

func (r *Repository) ListProfessionals(ctx context.Context) ([]domain.Professional, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, phone, active, created_at FROM professionals ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Professional{}
	for rows.Next() {
		var item domain.Professional
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateProfessional(ctx context.Context, item domain.Professional) (domain.Professional, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO professionals(name, phone) VALUES($1,$2) RETURNING id, active, created_at`,
		item.Name, item.Phone).Scan(&item.ID, &item.Active, &item.CreatedAt)
	return item, err
}

func (r *Repository) ListCustomers(ctx context.Context) ([]domain.Customer, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, phone, email, created_at FROM customers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Customer{}
	for rows.Next() {
		var item domain.Customer
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Email, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateCustomer(ctx context.Context, item domain.Customer) (domain.Customer, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO customers(name, phone, email) VALUES($1,$2,$3) RETURNING id, created_at`,
		item.Name, item.Phone, item.Email).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func (r *Repository) ListAppointments(ctx context.Context, from, to time.Time) ([]domain.Appointment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.customer_id, c.name, a.professional_id, p.name, a.service_id, s.name,
		       a.starts_at, a.ends_at, a.status, a.notes, a.price_cents, a.created_at
		FROM appointments a
		JOIN customers c ON c.id=a.customer_id
		JOIN professionals p ON p.id=a.professional_id
		JOIN services s ON s.id=a.service_id
		WHERE a.starts_at >= $1 AND a.starts_at < $2 ORDER BY a.starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Appointment{}
	for rows.Next() {
		var item domain.Appointment
		if err := rows.Scan(&item.ID, &item.CustomerID, &item.CustomerName, &item.ProfessionalID, &item.ProfessionalName,
			&item.ServiceID, &item.ServiceName, &item.StartsAt, &item.EndsAt, &item.Status, &item.Notes,
			&item.PriceCents, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateAppointment(ctx context.Context, item domain.Appointment) (domain.Appointment, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var duration int
	var serviceActive, professionalActive bool
	err = tx.QueryRow(ctx, `SELECT duration_minutes, price_cents, active FROM services WHERE id=$1 FOR SHARE`,
		item.ServiceID).Scan(&duration, &item.PriceCents, &serviceActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	err = tx.QueryRow(ctx, `SELECT active FROM professionals WHERE id=$1 FOR SHARE`, item.ProfessionalID).Scan(&professionalActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	var customerExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1)`, item.CustomerID).Scan(&customerExists); err != nil {
		return item, err
	}
	if !customerExists {
		return item, domain.ErrNotFound
	}
	if !serviceActive || !professionalActive {
		return item, fmt.Errorf("service and professional must be active")
	}

	item.EndsAt = item.StartsAt.Add(time.Duration(duration) * time.Minute)
	err = tx.QueryRow(ctx, `INSERT INTO appointments(customer_id,professional_id,service_id,starts_at,ends_at,notes,price_cents)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,status,created_at`,
		item.CustomerID, item.ProfessionalID, item.ServiceID, item.StartsAt, item.EndsAt, item.Notes, item.PriceCents).
		Scan(&item.ID, &item.Status, &item.CreatedAt)
	if isExclusionViolation(err) {
		return item, domain.ErrScheduleConflict
	}
	if err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r *Repository) UpdateAppointmentStatus(ctx context.Context, id, status string) (domain.Appointment, error) {
	var current string
	if err := r.db.QueryRow(ctx, `SELECT status FROM appointments WHERE id=$1`, id).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return domain.Appointment{}, domain.ErrNotFound
	} else if err != nil {
		return domain.Appointment{}, err
	}
	if !domain.CanTransition(current, status) {
		return domain.Appointment{}, domain.ErrInvalidTransition
	}
	var item domain.Appointment
	err := r.db.QueryRow(ctx, `
		UPDATE appointments SET status=$2 WHERE id=$1
		RETURNING id,customer_id,professional_id,service_id,starts_at,ends_at,status,notes,price_cents,created_at`,
		id, status).Scan(&item.ID, &item.CustomerID, &item.ProfessionalID, &item.ServiceID, &item.StartsAt,
		&item.EndsAt, &item.Status, &item.Notes, &item.PriceCents, &item.CreatedAt)
	return item, err
}

func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}
