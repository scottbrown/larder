package larder

import "testing"

func TestParseMoney(t *testing.T) {
	tests := []struct {
		in   string
		want Money
	}{
		{"12.99", 1299},
		{"$4", 400},
		{".99", 99},
		{"3.00-", -300},
		{"-1.50", -150},
		{"1,234.5", 123450},
		{"$0.05", 5},
	}
	for _, tt := range tests {
		got, err := ParseMoney(tt.in)
		if err != nil {
			t.Errorf("ParseMoney(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseMoney(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseMoneyRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "abc", "$", "1.2.3"} {
		if _, err := ParseMoney(in); err == nil {
			t.Errorf("ParseMoney(%q) succeeded, want error", in)
		}
	}
}

func TestMoneyFormatting(t *testing.T) {
	if got := Money(1299).String(); got != "$12.99" {
		t.Errorf("String() = %q", got)
	}
	if got := Money(-305).String(); got != "-$3.05" {
		t.Errorf("String() = %q", got)
	}
	if got := Money(-305).Abs(); got != 305 {
		t.Errorf("Abs() = %d", got)
	}
	if got := Money(250).Dollars(); got != 2.5 {
		t.Errorf("Dollars() = %v", got)
	}
}
