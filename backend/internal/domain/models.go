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
)

const (
	RoleManager      = "manager"
	RoleProfessional = "professional"
)

type User struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"`
	Role           string    `json:"role"`
	ProfessionalID string    `json:"professional_id,omitempty"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
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

func CanTransition(from, to string) bool {
	allowed := map[string]map[string]bool{
		"scheduled": {"confirmed": true, "cancelled": true},
		"confirmed": {"completed": true, "cancelled": true},
	}
	return allowed[from][to]
}
