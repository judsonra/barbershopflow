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

// User is a joined view of a membership (which tenant, which role, which
// customer/professional record) plus the identity backing it (name,
// contact, credentials). It's what every handler/JSON response works with;
// see Identity for the standalone identity-only view used during login
// before a specific membership has been resolved.
type User struct {
	ID                  string    `json:"id"`
	IdentityID          string    `json:"-"`
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

// Identity is a person's login credentials — one row, one password, shared
// across every barbershop (Membership) that person is enrolled in. Looked
// up first by email/phone/social id during login, before any tenant is
// known.
type Identity struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Email               string    `json:"email,omitempty"`
	Phone               string    `json:"phone,omitempty"`
	PasswordHash        string    `json:"-"`
	GoogleID            string    `json:"-"`
	FacebookID          string    `json:"-"`
	FailedLoginAttempts int       `json:"-"`
	LockedAt            time.Time `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

func (i Identity) Locked() bool { return !i.LockedAt.IsZero() }

// MembershipOption is one barbershop an identity is enrolled in, offered
// as a choice when login resolves to more than one — see
// server.resolveLogin.
type MembershipOption struct {
	MembershipID string `json:"membership_id"`
	IdentityID   string `json:"-"`
	TenantID     string `json:"tenant_id"`
	TenantName   string `json:"tenant_name"`
	TenantSlug   string `json:"tenant_slug"`
	Role         string `json:"role"`
}

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
	Email     string    `json:"email,omitempty"`
	CPF       string    `json:"cpf,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidCPF checks the standard Brazilian CPF check-digit algorithm. value
// may contain formatting (dots/dash) — only digits are considered.
// Sequences of 11 identical digits (e.g. "000.000.000-00") pass the
// checksum math but are never real CPFs, so they're rejected explicitly.
func ValidCPF(value string) bool {
	digits := make([]int, 0, 11)
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) != 11 {
		return false
	}
	allSame := true
	for _, d := range digits {
		if d != digits[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	checkDigit := func(length int) int {
		sum := 0
		for i := 0; i < length; i++ {
			sum += digits[i] * (length + 1 - i)
		}
		remainder := (sum * 10) % 11
		if remainder == 10 {
			remainder = 0
		}
		return remainder
	}
	return checkDigit(9) == digits[9] && checkDigit(10) == digits[10]
}

// DigitsOnly strips everything but 0-9, used to normalize CPF (and other
// masked numeric fields) before storage/comparison.
func DigitsOnly(value string) string {
	digits := make([]byte, 0, len(value))
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits = append(digits, byte(r))
		}
	}
	return string(digits)
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
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminCustomerMatch is one row of the superadmin's global customer search
// (GET /admin/customers?q=): a customer plus which barbershop they belong
// to, so a match can be found without impersonating tenant by tenant.
type AdminCustomerMatch struct {
	Customer
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	TenantSlug string `json:"tenant_slug"`
}

// ImpersonationAudit is one row of the durable impersonation trail (GET
// /admin/audit): which superadmin accessed which barbershop, and when —
// the log.Printf in server.impersonateTenant stays for local debugging,
// this is the queryable record.
type ImpersonationAudit struct {
	ID         string    `json:"id"`
	ActorName  string    `json:"actor_name"`
	ActorEmail string    `json:"actor_email,omitempty"`
	TenantID   string    `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	TenantSlug string    `json:"tenant_slug"`
	CreatedAt  time.Time `json:"created_at"`
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

// ProfessionalOccupancy is one professional's booked-vs-available minutes
// within a report period. Professionals with no configured
// professional_schedules are left out entirely (see OccupancyReport) since
// their available capacity is undefined, not zero.
type ProfessionalOccupancy struct {
	ProfessionalID   string  `json:"professional_id"`
	ProfessionalName string  `json:"professional_name"`
	AvailableMinutes int     `json:"available_minutes"`
	BookedMinutes    int     `json:"booked_minutes"`
	Rate             float64 `json:"rate"`
}

// OccupancyReport aggregates ProfessionalOccupancy. OverallRate is booked
// over available across every included professional combined - it is not
// an average of each professional's individual rate, so periods differ in
// weight by how much capacity each professional actually has.
type OccupancyReport struct {
	OverallRate    float64                  `json:"overall_rate"`
	ByProfessional []ProfessionalOccupancy `json:"by_professional"`
}

type ProfessionalRevenue struct {
	ProfessionalID   string `json:"professional_id"`
	ProfessionalName string `json:"professional_name"`
	TotalCents       int64  `json:"total_cents"`
}

// RevenueReport counts only StatusCompleted appointments - revenue that
// was actually realized, not merely scheduled/confirmed.
type RevenueReport struct {
	TotalCents     int64                  `json:"total_cents"`
	ByProfessional []ProfessionalRevenue `json:"by_professional"`
}

// RetentionReport measures repeat-customer rate: among customers with a
// completed appointment in the report period, how many already had a
// completed appointment before the period started.
type RetentionReport struct {
	TotalCustomers     int     `json:"total_customers"`
	ReturningCustomers int     `json:"returning_customers"`
	Rate               float64 `json:"rate"`
}

type Report struct {
	From      time.Time       `json:"from"`
	To        time.Time       `json:"to"`
	Occupancy OccupancyReport `json:"occupancy"`
	Revenue   RevenueReport   `json:"revenue"`
	Retention RetentionReport `json:"retention"`
}
