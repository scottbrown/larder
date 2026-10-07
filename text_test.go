package larder

import "testing"

func TestSplitPrice(t *testing.T) {
	tests := []struct {
		in       string
		wantText string
		want     Money
		wantOK   bool
	}{
		{"39036 ROMAINE 8.99", "39036 ROMAINE", 899, true},
		{"3333310 REGISTER 14.97 G", "3333310 REGISTER", 1497, true},
		{"2028074 TPD/39917 5.00-G", "2028074 TPD/39917", -500, true},
		{"MAC CHEESE 001356200050 $1.77 D", "MAC CHEESE 001356200050", 177, true},
		{"MAC CHEESE 001356200050 $1 77 D", "MAC CHEESE 001356200050", 177, true},
		{"PANKO BREAD CRUMBS $0.43GD", "PANKO BREAD CRUMBS", 43, true},
		{"Card $3.45 Save -1.54", "Card $3.45 Save", -154, true},
		{"0.815 kg @ $2.14/kg 1.74", "0.815 kg @ $2.14/kg", 174, true},
		{"0.485 kg @ $13.18/kg 6. 39", "0.485 kg @ $13.18/kg", 639, true},
		{"MEIJI VANILLA YAN YAN W $1. 99 G", "MEIJI VANILLA YAN YAN W", 199, true},
		{"ROOSTER RICE STK MRJ", "ROOSTER RICE STK MRJ", 0, false},
		{"BOTTLED WATER 24", "BOTTLED WATER 24", 0, false},
	}
	for _, tt := range tests {
		text, amount, ok := splitPrice(tt.in)
		if ok != tt.wantOK || amount != tt.want || text != tt.wantText {
			t.Errorf("splitPrice(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.in, text, amount, ok, tt.wantText, tt.want, tt.wantOK)
		}
	}
}

func TestNormaliseLine(t *testing.T) {
	got := normaliseLine("F&F CINNAMON GRND |  S2.44")
	if got != "F&F CINNAMON GRND $2.44" {
		t.Errorf("normaliseLine = %q", got)
	}
	if got := normaliseLine("0.590 kg ₴ | $5.45/kg"); got != "0.590 kg @ $5.45/kg" {
		t.Errorf("normaliseLine = %q", got)
	}
}

func TestNormaliseDescription(t *testing.T) {
	if got := normaliseDescription("  Kewpie  sesam. "); got != "KEWPIE SESAM" {
		t.Errorf("normaliseDescription = %q", got)
	}
}

func TestNormaliseLineRepairsAmounts(t *testing.T) {
	tests := map[string]string{
		"1333726 TPD/535333 2.0C-": "1333726 TPD/535333 2.00-",
		"SUBTOTÂL 4.69":            "SUBTOTAL 4.69",
		"BANANAS 1O.99 G":          "BANANAS 10.99 G",
		"KS BATTERY 119.99 br":     "KS BATTERY 119.99 br",
	}
	for in, want := range tests {
		if got := normaliseLine(in); got != want {
			t.Errorf("normaliseLine(%q) = %q, want %q", in, got, want)
		}
	}
	if _, amount, ok := splitPrice("924280 KS BATTERY 119.99 br"); !ok || amount != 11999 {
		t.Errorf("lower-case flags not tolerated: %d %v", amount, ok)
	}
}

func TestIsSubtotal(t *testing.T) {
	for _, s := range []string{"SUBTOTAL 9.86", "SUBTO AL 70.69", "Sub Total $27.20", "SUB TOTAL $21.69"} {
		if !isSubtotal(s) {
			t.Errorf("isSubtotal(%q) = false", s)
		}
	}
	for _, s := range []string{"TOTAL 9.94", "SUBWAY SANDWICH 5.99"} {
		if isSubtotal(s) {
			t.Errorf("isSubtotal(%q) = true", s)
		}
	}
}

func TestNormaliseLineRepairsLetterDigits(t *testing.T) {
	tests := map[string]string{
		"453777 CIN TST 1.3K B.49": "453777 CIN TST 1.3K 8.49",
		"15690 BAKTNG BWDR 8/69":   "15690 BAKTNG BWDR 8.69",
		"MAYO LITE":                "MAYO LITE",
	}
	for in, want := range tests {
		if got := normaliseLine(in); got != want {
			t.Errorf("normaliseLine(%q) = %q, want %q", in, got, want)
		}
	}
}
