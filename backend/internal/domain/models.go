package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrScheduleConflict   = errors.New("professional already has an appointment in this period")
	ErrInvalidTransition  = errors.New("invalid appointment status transition")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrForbidden          = errors.New("not allowed to perform this action")
	ErrAccountLocked      = errors.New("account locked after too many failed login attempts")
	ErrConflict           = errors.New("already in use")
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
