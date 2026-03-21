package places

import "testing"

func TestNormalizeSettlementType(t *testing.T) {
	tests := []struct {
		code string
		pop  int64
		want string
	}{
		// Capitals / admin seats → city
		{"PPLC", 0, "місто"},
		{"PPLC", 3_000_000, "місто"},
		{"PPLA", 0, "місто"},
		{"PPLA2", 500, "місто"},
		{"PPLA3", 0, "місто"},
		{"PPLA4", 0, "місто"},

		// Generic PPL — Volnogorsk-class (~22k) and other cities without PPLA*
		{"PPL", 22_000, "місто"},
		{"PPL", 10_000, "місто"},
		{"PPL", 5_000, "місто"},
		{"PPL", 4_999, "селище"},
		{"PPL", 2_000, "селище"},
		{"PPL", 500, "селище"},
		{"PPL", 499, "село"},
		{"PPL", 100, "село"},
		{"PPL", 0, "село"},

		{"ppl", 50_000, "місто"},
		{" PPL ", 15_000, "місто"},

		{"PPLQ", 0, ""},
		{"", 0, ""},
	}
	for _, tt := range tests {
		got := NormalizeSettlementType(tt.code, tt.pop)
		if got != tt.want {
			t.Errorf("NormalizeSettlementType(%q, %d) = %q; want %q", tt.code, tt.pop, got, tt.want)
		}
	}
}
