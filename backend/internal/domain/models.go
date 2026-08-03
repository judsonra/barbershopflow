package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrScheduleConflict    = errors.New("professional already has an appointment in this period")
	ErrInvalidTransition   = errors.New("invalid appointment status transition")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrForbidden           = errors.New("not allowed to perform this action")
	ErrAccountLocked       = errors.New("account locked after too many failed login attempts")
	ErrConflict            = errors.New("already in use")
	ErrOutsideWorkingHours = errors.New("requested time is outside the professional's working hours")
	ErrTimeBlocked         = errors.New("requested time overlaps a blocked period")
)

type Tenant struct {
	ID                      string    `json:"id"`
	Name                    string    `json:"name"`
	Slug                    string    `json:"slug"`
	SelfSchedulingEnabled   bool      `json:"self_scheduling_enabled"`
	AutoConfirmAppointments bool      `json:"auto_confirm_appointments"`
	Active                  bool      `json:"active"`
	CreatedAt               time.Time `json:"created_at"`
}

const (
	RoleManager      = "manager"
	RoleProfessional = "professional"
	RoleClient       = "client"
	// RoleSuperAdmin is the platform operator: not scoped to any tenant's
	// data directly. It only lists tenants and impersonates a tenant's
	// manager (see server.impersonateTenant) — every other endpoint stays
	// scoped to a single tenant, so this role never needs to bypass that.
	RoleSuperAdmin = "superadmin"
)

// MaxLoginAttempts is the number of failed password attempts allowed before
// an account is locked. Only enforced for accounts without an e-mail (phone
// + password login) — social login and staff accounts are not rate limited
// this way.
const MaxLoginAttempts = 3

type User struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	Name                string    `json:"name"`
	Email               string    `json:"email,omitempty"`
	Phone               string    `json:"phone,omitempty"`
	PasswordHash        string    `json:"-"`
	GoogleID            string    `json:"-"`
	FacebookID          string    `json:"-"`
	Role                string    `json:"role"`
	ProfessionalID      string    `json:"professional_id,omitempty"`
	CustomerID          string    `json:"customer_id,omitempty"`
	FailedLoginAttempts int       `json:"-"`
	LockedAt            time.Time `json:"-"`
	Active              bool      `json:"active"`
	CreatedAt           time.Time `json:"created_at"`
}

func (u User) Locked() bool { return !u.LockedAt.IsZero() }

type Service struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	DurationMinutes int       `json:"duration_minutes"`
	PriceCents      int64     `json:"price_cents"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}

type Professional struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// ScheduleEntry is one weekday's working window for a professional, in
// minutes since midnight (local to whatever offset appointments for that
// professional are booked in — see docs/regras-de-negocio.md). Weekday
// follows Go's time.Weekday: 0=Sunday..6=Saturday. A professional with no
// entries at all has no working-hours restriction (backward compatible with
// professionals created before this feature); once at least one entry
// exists, any weekday without one becomes a fixed day off.
type ScheduleEntry struct {
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

// TimeOff is an ad-hoc blocked period (absence, day off, travel) that
// overrides the recurring schedule and blocks new appointments regardless
// of it.
type TimeOff struct {
	ID             string    `json:"id"`
	ProfessionalID string    `json:"professional_id"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Customer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone,omitempty"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Appointment struct {
	ID               string    `json:"id"`
	CustomerID       string    `json:"customer_id"`
	CustomerName     string    `json:"customer_name"`
	ProfessionalID   string    `json:"professional_id"`
	ProfessionalName string    `json:"professional_name"`
	ServiceID        string    `json:"service_id"`
	ServiceName      string    `json:"service_name"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	Status           string    `json:"status"`
	Notes            string    `json:"notes,omitempty"`
	PriceCents       int64     `json:"price_cents"`
	CreatedAt        time.Time `json:"created_at"`
}

const (
	StatusScheduled = "scheduled"
	StatusConfirmed = "confirmed"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

func CanTransition(from, to string) bool {
	allowed := map[string]map[string]bool{
		StatusScheduled: {StatusConfirmed: true, StatusCancelled: true},
		StatusConfirmed: {StatusCompleted: true, StatusCancelled: true},
	}
	return allowed[from][to]
}
