package domain

import (
	"testing"
	"time"
)

func TestCommissionCents(t *testing.T) {
	cases := []struct {
		name  string
		price int64
		rule  *ProfessionalCommission
		want  int64
	}{
		{"sem regra", 5000, nil, 0},
		{"40% de R$ 50,00", 5000, &ProfessionalCommission{Type: CommissionPercentage, Value: 4000}, 2000},
		{"33,33% arredonda ao centavo", 1000, &ProfessionalCommission{Type: CommissionPercentage, Value: 3333}, 333},
		{"12,5% de R$ 0,99 arredonda para cima", 99, &ProfessionalCommission{Type: CommissionPercentage, Value: 1250}, 12},
		{"100%", 4590, &ProfessionalCommission{Type: CommissionPercentage, Value: 10000}, 4590},
		{"fixo ignora o preço", 5000, &ProfessionalCommission{Type: CommissionFixed, Value: 1500}, 1500},
		{"fixo zero", 5000, &ProfessionalCommission{Type: CommissionFixed, Value: 0}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CommissionCents(tc.price, tc.rule); got != tc.want {
				t.Fatalf("CommissionCents(%d) = %d, want %d", tc.price, got, tc.want)
			}
		})
	}
}

func TestSummarizeCommissions(t *testing.T) {
	pct := &ProfessionalCommission{Type: CommissionPercentage, Value: 4000}
	fixed := &ProfessionalCommission{Type: CommissionFixed, Value: 1000}
	lines := []CommissionLine{
		{ProfessionalID: "a", ProfessionalName: "Ana", PriceCents: 5000, Rule: pct},
		{ProfessionalID: "a", ProfessionalName: "Ana", PriceCents: 3000, Rule: nil},
		{ProfessionalID: "b", ProfessionalName: "Bruno", PriceCents: 7000, Rule: fixed},
	}
	from, to := time.Unix(0, 0), time.Unix(100, 0)
	got := SummarizeCommissions(from, to, lines)

	if got.TotalCommissionCents != 3000 || len(got.Items) != 2 {
		t.Fatalf("unexpected report: %+v", got)
	}
	ana, bruno := got.Items[0], got.Items[1]
	if ana.ProfessionalName != "Ana" || ana.Appointments != 2 || ana.RevenueCents != 8000 || ana.CommissionCents != 2000 || ana.WithoutRule != 1 {
		t.Fatalf("unexpected Ana: %+v", ana)
	}
	if bruno.Appointments != 1 || bruno.CommissionCents != 1000 || bruno.WithoutRule != 0 {
		t.Fatalf("unexpected Bruno: %+v", bruno)
	}
	if empty := SummarizeCommissions(from, to, nil); empty.Items == nil || empty.TotalCommissionCents != 0 {
		t.Fatalf("empty report must have an empty (non-nil) list: %+v", empty)
	}
}
