package repository

import (
	"context"
	"database/sql"
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

// Data-scoped resources (services, professionals, customers, appointments)
// always take tenantID as their first non-context argument and filter every
// query by it, so one tenant's data never leaks into another's response.
// Login identifiers (e-mail, phone, google/facebook id) stay globally
// unique across tenants instead — see docs/regras-de-negocio.md — so user
// lookups by those identifiers are not tenant-scoped; domain.User itself
// carries TenantID for the rows that are looked up by id/identifier.

func (r *Repository) ListServices(ctx context.Context, tenantID string) ([]domain.Service, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, duration_minutes, price_cents, active, created_at FROM services WHERE tenant_id=$1 ORDER BY name`, tenantID)
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

func (r *Repository) CreateService(ctx context.Context, tenantID string, item domain.Service) (domain.Service, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO services(tenant_id, name, duration_minutes, price_cents) VALUES($1,$2,$3,$4) RETURNING id, active, created_at`,
		tenantID, item.Name, item.DurationMinutes, item.PriceCents).Scan(&item.ID, &item.Active, &item.CreatedAt)
	return item, err
}

func (r *Repository) ListProfessionals(ctx context.Context, tenantID string) ([]domain.Professional, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, phone, email, cpf, active, created_at FROM professionals WHERE tenant_id=$1 ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Professional{}
	for rows.Next() {
		var item domain.Professional
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Email, &item.CPF, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateProfessional(ctx context.Context, tenantID string, item domain.Professional) (domain.Professional, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO professionals(tenant_id, name, phone, email, cpf) VALUES($1,$2,$3,$4,$5) RETURNING id, active, created_at`,
		tenantID, item.Name, item.Phone, item.Email, item.CPF).Scan(&item.ID, &item.Active, &item.CreatedAt)
	if _, ok := isUniqueViolation(err); ok {
		return item, domain.ErrConflict
	}
	return item, err
}

func (r *Repository) professionalExists(ctx context.Context, tenantID, professionalID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM professionals WHERE id=$1 AND tenant_id=$2)`, professionalID, tenantID).Scan(&exists)
	return exists, err
}

func (r *Repository) GetProfessionalSchedule(ctx context.Context, tenantID, professionalID string) ([]domain.ScheduleEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT weekday, start_minute, end_minute FROM professional_schedules
		WHERE tenant_id=$1 AND professional_id=$2 ORDER BY weekday`, tenantID, professionalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.ScheduleEntry{}
	for rows.Next() {
		var item domain.ScheduleEntry
		if err := rows.Scan(&item.Weekday, &item.StartMinute, &item.EndMinute); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SetProfessionalSchedule replaces the professional's whole week in one go
// - the caller (server.setProfessionalSchedule) always sends the full set,
// so partial/incremental updates aren't needed.
func (r *Repository) SetProfessionalSchedule(ctx context.Context, tenantID, professionalID string, entries []domain.ScheduleEntry) ([]domain.ScheduleEntry, error) {
	exists, err := r.professionalExists(ctx, tenantID, professionalID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM professional_schedules WHERE tenant_id=$1 AND professional_id=$2`, tenantID, professionalID); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if _, err := tx.Exec(ctx, `INSERT INTO professional_schedules(tenant_id, professional_id, weekday, start_minute, end_minute) VALUES($1,$2,$3,$4,$5)`,
			tenantID, professionalID, entry.Weekday, entry.StartMinute, entry.EndMinute); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *Repository) ListTimeOff(ctx context.Context, tenantID, professionalID string, from, to time.Time) ([]domain.TimeOff, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, professional_id, starts_at, ends_at, reason, created_at FROM professional_time_off
		WHERE tenant_id=$1 AND professional_id=$2 AND ends_at > $3 AND starts_at < $4
		ORDER BY starts_at`, tenantID, professionalID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.TimeOff{}
	for rows.Next() {
		var item domain.TimeOff
		if err := rows.Scan(&item.ID, &item.ProfessionalID, &item.StartsAt, &item.EndsAt, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateTimeOff(ctx context.Context, tenantID, professionalID string, item domain.TimeOff) (domain.TimeOff, error) {
	exists, err := r.professionalExists(ctx, tenantID, professionalID)
	if err != nil {
		return item, err
	}
	if !exists {
		return item, domain.ErrNotFound
	}
	item.ProfessionalID = professionalID
	err = r.db.QueryRow(ctx, `
		INSERT INTO professional_time_off(tenant_id, professional_id, starts_at, ends_at, reason)
		VALUES($1,$2,$3,$4,$5) RETURNING id, created_at`,
		tenantID, professionalID, item.StartsAt, item.EndsAt, item.Reason).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func (r *Repository) DeleteTimeOff(ctx context.Context, tenantID, professionalID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM professional_time_off WHERE id=$1 AND tenant_id=$2 AND professional_id=$3`, id, tenantID, professionalID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ListCustomers(ctx context.Context, tenantID string) ([]domain.Customer, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, phone, email, created_at FROM customers WHERE tenant_id=$1 ORDER BY name`, tenantID)
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

func (r *Repository) CreateCustomer(ctx context.Context, tenantID string, item domain.Customer) (domain.Customer, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO customers(tenant_id, name, phone, email) VALUES($1,$2,$3,$4) RETURNING id, created_at`,
		tenantID, item.Name, item.Phone, item.Email).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func (r *Repository) GetCustomerByID(ctx context.Context, tenantID, id string) (domain.Customer, error) {
	var item domain.Customer
	err := r.db.QueryRow(ctx, `SELECT id, name, phone, email, created_at FROM customers WHERE id=$1 AND tenant_id=$2`, id, tenantID).
		Scan(&item.ID, &item.Name, &item.Phone, &item.Email, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	return item, err
}

// UpdateCustomerPhone lets the barber fill in/correct the phone number at
// the moment they grant the client access.
func (r *Repository) UpdateCustomerPhone(ctx context.Context, tenantID, customerID, phone string) error {
	tag, err := r.db.Exec(ctx, `UPDATE customers SET phone=$3 WHERE id=$1 AND tenant_id=$2`, customerID, tenantID, phone)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListAppointments returns the tenant's agenda in [from, to). When
// customerID is non-empty (a client listing their own appointments), it
// only returns that customer's appointments; staff pass "" to see everyone.
func (r *Repository) ListAppointments(ctx context.Context, tenantID, customerID string, from, to time.Time) ([]domain.Appointment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.customer_id, c.name, a.professional_id, p.name, a.service_id, s.name,
		       a.starts_at, a.ends_at, a.status, a.notes, a.price_cents, a.created_at
		FROM appointments a
		JOIN customers c ON c.id=a.customer_id
		JOIN professionals p ON p.id=a.professional_id
		JOIN services s ON s.id=a.service_id
		WHERE a.tenant_id=$1 AND a.starts_at >= $2 AND a.starts_at < $3
		  AND ($4 = '' OR a.customer_id::text = $4)
		ORDER BY a.starts_at`, tenantID, from, to, customerID)
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

// CreateAppointment inserts with the given initial status: staff-created
// appointments always pass domain.StatusScheduled; client self-scheduling
// passes StatusConfirmed instead when the tenant's auto-confirm setting is
// on (see server.createAppointment).
func (r *Repository) CreateAppointment(ctx context.Context, tenantID, status string, item domain.Appointment) (domain.Appointment, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var duration int
	var serviceActive, professionalActive bool
	err = tx.QueryRow(ctx, `SELECT duration_minutes, price_cents, active FROM services WHERE id=$1 AND tenant_id=$2 FOR SHARE`,
		item.ServiceID, tenantID).Scan(&duration, &item.PriceCents, &serviceActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	err = tx.QueryRow(ctx, `SELECT active FROM professionals WHERE id=$1 AND tenant_id=$2 FOR SHARE`, item.ProfessionalID, tenantID).Scan(&professionalActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	var customerExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND tenant_id=$2)`, item.CustomerID, tenantID).Scan(&customerExists); err != nil {
		return item, err
	}
	if !customerExists {
		return item, domain.ErrNotFound
	}
	if !serviceActive || !professionalActive {
		return item, fmt.Errorf("service and professional must be active")
	}

	item.EndsAt = item.StartsAt.Add(time.Duration(duration) * time.Minute)

	var hasSchedule bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM professional_schedules WHERE professional_id=$1)`, item.ProfessionalID).Scan(&hasSchedule); err != nil {
		return item, err
	}
	if hasSchedule {
		weekday := int(item.StartsAt.Weekday())
		startMinute := item.StartsAt.Hour()*60 + item.StartsAt.Minute()
		endMinute := startMinute + duration
		var windowStart, windowEnd int
		err := tx.QueryRow(ctx, `SELECT start_minute, end_minute FROM professional_schedules WHERE professional_id=$1 AND weekday=$2`,
			item.ProfessionalID, weekday).Scan(&windowStart, &windowEnd)
		if errors.Is(err, pgx.ErrNoRows) {
			return item, domain.ErrOutsideWorkingHours
		}
		if err != nil {
			return item, err
		}
		if startMinute < windowStart || endMinute > windowEnd {
			return item, domain.ErrOutsideWorkingHours
		}
	}

	var blocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM professional_time_off
			WHERE professional_id=$1 AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)'))`,
		item.ProfessionalID, item.StartsAt, item.EndsAt).Scan(&blocked); err != nil {
		return item, err
	}
	if blocked {
		return item, domain.ErrTimeBlocked
	}

	err = tx.QueryRow(ctx, `INSERT INTO appointments(tenant_id,customer_id,professional_id,service_id,starts_at,ends_at,notes,price_cents,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,status,created_at`,
		tenantID, item.CustomerID, item.ProfessionalID, item.ServiceID, item.StartsAt, item.EndsAt, item.Notes, item.PriceCents, status).
		Scan(&item.ID, &item.Status, &item.CreatedAt)
	if isExclusionViolation(err) {
		return item, domain.ErrScheduleConflict
	}
	if err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r *Repository) UpdateAppointmentStatus(ctx context.Context, tenantID, id, status string) (domain.Appointment, error) {
	var current string
	if err := r.db.QueryRow(ctx, `SELECT status FROM appointments WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return domain.Appointment{}, domain.ErrNotFound
	} else if err != nil {
		return domain.Appointment{}, err
	}
	if !domain.CanTransition(current, status) {
		return domain.Appointment{}, domain.ErrInvalidTransition
	}
	var item domain.Appointment
	err := r.db.QueryRow(ctx, `
		UPDATE appointments SET status=$3 WHERE id=$1 AND tenant_id=$2
		RETURNING id,customer_id,professional_id,service_id,starts_at,ends_at,status,notes,price_cents,created_at`,
		id, tenantID, status).Scan(&item.ID, &item.CustomerID, &item.ProfessionalID, &item.ServiceID, &item.StartsAt,
		&item.EndsAt, &item.Status, &item.Notes, &item.PriceCents, &item.CreatedAt)
	return item, err
}

func (r *Repository) GetAppointmentProfessionalID(ctx context.Context, tenantID, id string) (string, error) {
	var professionalID string
	err := r.db.QueryRow(ctx, `SELECT professional_id FROM appointments WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&professionalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return professionalID, err
}

const userColumns = `id, tenant_id, name, email, phone, password_hash, google_id, facebook_id, role,
	professional_id, customer_id, failed_login_attempts, locked_at, active, created_at`

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email)=lower($1)`, email))
}

func (r *Repository) GetUserByPhone(ctx context.Context, phone string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE phone=$1`, phone))
}

func (r *Repository) GetUserByGoogleID(ctx context.Context, googleID string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE google_id=$1`, googleID))
}

func (r *Repository) GetUserByFacebookID(ctx context.Context, facebookID string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE facebook_id=$1`, facebookID))
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id))
}

// CreateClientUser provisions a brand-new customer + login pair from a
// social login self-registration (google/facebook id set, no password).
// item.TenantID must already be set by the caller.
func (r *Repository) CreateClientUser(ctx context.Context, item domain.User) (domain.User, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO users(tenant_id, name, email, phone, password_hash, google_id, facebook_id, role, customer_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,'client',$8)
		RETURNING `+userColumns,
		item.TenantID, item.Name, nullable(item.Email), nullable(item.Phone), nullable(item.PasswordHash),
		nullable(item.GoogleID), nullable(item.FacebookID), item.CustomerID).
		Scan(userScanTargets(&item)...)
	return item, err
}

// LinkGoogleID attaches a Google account to an existing user (staff account
// recovery, or a returning client).
func (r *Repository) LinkGoogleID(ctx context.Context, userID, googleID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET google_id=$2 WHERE id=$1`, userID, googleID)
	return err
}

func (r *Repository) LinkFacebookID(ctx context.Context, userID, facebookID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET facebook_id=$2 WHERE id=$1`, userID, facebookID)
	return err
}

// SetPassword is used both when the barber grants a client phone access and
// when a locked-out client recovers via a new SMS/WhatsApp password. Both
// cases should also clear the lockout.
func (r *Repository) SetPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash=$2, failed_login_attempts=0, locked_at=NULL WHERE id=$1`, userID, passwordHash)
	return err
}

func (r *Repository) ResetLoginAttempts(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET failed_login_attempts=0, locked_at=NULL WHERE id=$1`, userID)
	return err
}

// IncrementFailedLogin records a failed password attempt and locks the
// account once domain.MaxLoginAttempts is reached, returning whether it is
// now locked.
func (r *Repository) IncrementFailedLogin(ctx context.Context, userID string) (bool, error) {
	var locked bool
	err := r.db.QueryRow(ctx, `
		UPDATE users SET
			failed_login_attempts = failed_login_attempts + 1,
			locked_at = CASE WHEN failed_login_attempts + 1 >= $2 THEN now() ELSE locked_at END
		WHERE id=$1
		RETURNING locked_at IS NOT NULL`, userID, domain.MaxLoginAttempts).Scan(&locked)
	return locked, err
}

// UpsertClientCredentials grants (or regenerates) a customer's phone-login
// password: creates the linked user on first use, or resets its password
// and lockout state on subsequent calls. Callers must have already
// confirmed customerID belongs to tenantID (e.g. via GetCustomerByID).
func (r *Repository) UpsertClientCredentials(ctx context.Context, tenantID, customerID, name, phone, passwordHash string) (domain.User, error) {
	item := domain.User{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO users(tenant_id, name, phone, password_hash, role, customer_id)
		VALUES($1,$2,$3,$4,'client',$5)
		ON CONFLICT (customer_id) WHERE customer_id IS NOT NULL DO UPDATE SET
			name = EXCLUDED.name,
			phone = EXCLUDED.phone,
			password_hash = EXCLUDED.password_hash,
			failed_login_attempts = 0,
			locked_at = NULL
		RETURNING `+userColumns,
		tenantID, name, phone, passwordHash, customerID).
		Scan(userScanTargets(&item)...)
	return item, err
}

const tenantColumns = `id, name, slug, self_scheduling_enabled, auto_confirm_appointments, active, created_at`

func scanTenant(item *domain.Tenant) []any {
	return []any{&item.ID, &item.Name, &item.Slug, &item.SelfSchedulingEnabled, &item.AutoConfirmAppointments, &item.Active, &item.CreatedAt}
}

// ListTenants is superadmin-only: every other query in this file is
// deliberately scoped to a single tenant.
func (r *Repository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := r.db.Query(ctx, `SELECT `+tenantColumns+` FROM tenants ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Tenant{}
	for rows.Next() {
		var item domain.Tenant
		if err := rows.Scan(scanTenant(&item)...); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetFirstManagerByTenant backs superadmin impersonation: hand back the
// tenant's own oldest manager account and issue tokens for it, rather than
// minting a token for a user row that doesn't match its claims.
func (r *Repository) GetFirstManagerByTenant(ctx context.Context, tenantID string) (domain.User, error) {
	return r.scanUser(r.db.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE tenant_id=$1 AND role='manager' ORDER BY created_at LIMIT 1`, tenantID))
}

// TenantNameAvailable backs the real-time name check on the signup form.
// Best-effort only — the unique index on tenants(lower(name)) is what
// actually prevents a race between the check and the real signup.
func (r *Repository) TenantNameAvailable(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE lower(name)=lower($1))`, name).Scan(&exists)
	return !exists, err
}

// GetTenantBySlug is used both by tenant onboarding (uniqueness check) and
// by OAuth self-registration to resolve the ?tenant=<slug> query param.
func (r *Repository) GetTenantBySlug(ctx context.Context, slug string) (domain.Tenant, error) {
	var item domain.Tenant
	err := r.db.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE slug=$1`, slug).Scan(scanTenant(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	return item, err
}

func (r *Repository) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	var item domain.Tenant
	err := r.db.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE id=$1`, id).Scan(scanTenant(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	return item, err
}

// UpdateTenantSettings lets a manager turn self-scheduling and
// auto-confirmation on or off for their own barbershop.
func (r *Repository) UpdateTenantSettings(ctx context.Context, tenantID string, selfSchedulingEnabled, autoConfirmAppointments bool) (domain.Tenant, error) {
	var item domain.Tenant
	err := r.db.QueryRow(ctx, `
		UPDATE tenants SET self_scheduling_enabled=$2, auto_confirm_appointments=$3
		WHERE id=$1 RETURNING `+tenantColumns,
		tenantID, selfSchedulingEnabled, autoConfirmAppointments).Scan(scanTenant(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	return item, err
}

// CreateTenantWithManager is the self-service onboarding flow: a brand-new
// barbershop and its first manager account are created atomically, so a
// failure never leaves an orphaned tenant with no one able to log into it.
func (r *Repository) CreateTenantWithManager(ctx context.Context, tenantName, slug, managerName, email, passwordHash string) (domain.Tenant, domain.User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Tenant{}, domain.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenant domain.Tenant
	err = tx.QueryRow(ctx, `INSERT INTO tenants(name, slug) VALUES($1,$2) RETURNING `+tenantColumns,
		tenantName, slug).Scan(scanTenant(&tenant)...)
	if constraint, ok := isUniqueViolation(err); ok {
		return domain.Tenant{}, domain.User{}, fmt.Errorf("%w: %s", domain.ErrConflict, constraint)
	}
	if err != nil {
		return domain.Tenant{}, domain.User{}, err
	}

	manager := domain.User{TenantID: tenant.ID}
	err = tx.QueryRow(ctx, `
		INSERT INTO users(tenant_id, name, email, password_hash, role)
		VALUES($1,$2,$3,$4,'manager')
		RETURNING `+userColumns,
		tenant.ID, managerName, email, passwordHash).
		Scan(userScanTargets(&manager)...)
	if constraint, ok := isUniqueViolation(err); ok {
		return domain.Tenant{}, domain.User{}, fmt.Errorf("%w: %s", domain.ErrConflict, constraint)
	}
	if err != nil {
		return domain.Tenant{}, domain.User{}, err
	}

	return tenant, manager, tx.Commit(ctx)
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func userScanTargets(item *domain.User) []any {
	return []any{
		&item.ID, &item.TenantID, &item.Name, &nullString{&item.Email}, &nullString{&item.Phone}, &nullString{&item.PasswordHash},
		&nullString{&item.GoogleID}, &nullString{&item.FacebookID}, &item.Role,
		&nullString{&item.ProfessionalID}, &nullString{&item.CustomerID},
		&item.FailedLoginAttempts, &nullTime{&item.LockedAt}, &item.Active, &item.CreatedAt,
	}
}

// nullString/nullTime adapt nullable Postgres columns onto the plain string
// and time.Time fields domain.User exposes, so callers never juggle
// sql.NullString themselves.
type nullString struct{ dst *string }

func (n *nullString) Scan(value any) error {
	var wrapped sql.NullString
	if err := wrapped.Scan(value); err != nil {
		return err
	}
	*n.dst = wrapped.String
	return nil
}

type nullTime struct{ dst *time.Time }

func (n *nullTime) Scan(value any) error {
	var wrapped sql.NullTime
	if err := wrapped.Scan(value); err != nil {
		return err
	}
	*n.dst = wrapped.Time
	return nil
}

func (r *Repository) scanUser(row pgx.Row) (domain.User, error) {
	var item domain.User
	err := row.Scan(userScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	return item, err
}

func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

func isUniqueViolation(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}
