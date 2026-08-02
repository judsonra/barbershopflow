package domain

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"scheduled", "confirmed", true},
		{"scheduled", "cancelled", true},
		{"confirmed", "completed", true},
		{"completed", "scheduled", false},
		{"cancelled", "confirmed", false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
