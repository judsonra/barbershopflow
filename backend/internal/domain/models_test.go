package domain

import (
	"testing"
	"time"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"scheduled", "confirmed", true},
		{"scheduled", "cancelled", true},
		{"scheduled", "no_show", true},
		{"confirmed", "completed", true},
		{"confirmed", "no_show", true},
		{"completed", "scheduled", false},
		{"cancelled", "confirmed", false},
		{"no_show", "scheduled", false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestValidCPF(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"valid, formatted", "111.444.777-35", true},
		{"valid, digits only", "11144477735", true},
		{"wrong check digits", "111.444.777-36", false},
		{"all identical digits", "111.111.111-11", false},
		{"too short", "123456789", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidCPF(tt.value); got != tt.want {
				t.Errorf("ValidCPF(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestDigitsOnly(t *testing.T) {
	if got := DigitsOnly("111.444.777-35"); got != "11144477735" {
		t.Errorf("DigitsOnly() = %q, want %q", got, "11144477735")
	}
}

func TestUserLocked(t *testing.T) {
	tests := []struct {
		name     string
		lockedAt time.Time
		want     bool
	}{
		{"zero value is not locked", time.Time{}, false},
		{"non-zero value is locked", time.Now(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (User{LockedAt: tt.lockedAt}).Locked(); got != tt.want {
				t.Errorf("User.Locked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdentityLocked(t *testing.T) {
	tests := []struct {
		name     string
		lockedAt time.Time
		want     bool
	}{
		{"zero value is not locked", time.Time{}, false},
		{"non-zero value is locked", time.Now(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Identity{LockedAt: tt.lockedAt}).Locked(); got != tt.want {
				t.Errorf("Identity.Locked() = %v, want %v", got, tt.want)
			}
		})
	}
}
