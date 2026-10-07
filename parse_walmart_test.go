package larder

import "testing"

func TestWalmartAmount(t *testing.T) {
	tests := []struct {
		in     string
		want   Money
		wantOK bool
	}{
		{"$1.77 D", 177, true},
		{"$1 77 D", 177, true},
		{"$1 4T", 147, true},
		{"$1.4T", 147, true},
		{"$2 2.97", 297, true},
		{"$1 00 H", 100, true},
		{"$1 .17", 117, true},
		{"$2.35-D", -235, true},
		{"$1", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := walmartAmount(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("walmartAmount(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestParseWalmart(t *testing.T) {
	p := parseWalmart([]string{
		"ST# 03652 OP# 009046 TE# 46 TR# 02929",
		"MAC CHEESE 001356200050 $1.77 D",
		"OTV TOMATO 000000004664K",
		"0.590 kg @ $5.45/kg $3.22 D",
		"SUBTOTAL $4.99",
		"PLASTIC BAG 000000001234K",
		"2 AT $0.05 $0.10 C",
		"SAUCE 006591200128",
		"3 AT $1.00 $3.00",
		"SUBTOTAL $8.09",
		"MULTI DISCOUNT",
		"Sauces 5 for $5 018L $0.50-D",
		"SUBTOTAL $7.59",
		"MCARD TEND $7.59",
		"SUBTOTAL $99.99",
	})
	if !p.found || p.subtotal != 759 {
		t.Fatalf("subtotal = %v", p.subtotal)
	}
	if len(p.items) != 3 {
		t.Fatalf("items = %+v", p.items)
	}
	if p.items[0].Code != "1356200050" || p.items[0].Price != 177 {
		t.Errorf("mac = %+v", p.items[0])
	}
	if p.items[1].Quantity != Kilograms(0.59) || p.items[1].Price != 322 {
		t.Errorf("tomato = %+v", p.items[1])
	}
	if p.items[2].Quantity != Units(3) || p.items[2].Price != 300 {
		t.Errorf("sauce = %+v", p.items[2])
	}
	if p.fees != 10 || p.adjustments != -50 {
		t.Errorf("fees %v adjustments %v", p.fees, p.adjustments)
	}
}

func TestParseWalmartMultiBuyDiscount(t *testing.T) {
	p := parseWalmart([]string{
		"SHEPHERD 006620001741L $1.47 D",
		"MULTI 18",
		"GF GRAVY TUR 006620001526L $1.47 D",
		"MJLTI 18",
		"JELL-0 006618801950 $1.97 D",
		"SUBTOTAL $4.91",
		"MULTI DISCOUNT",
		"Sauces 2 for $2 018L $0.94-D",
		"SUBTOTAL $3.97",
		"MCARD TEND $3.97",
	})
	if len(p.items) != 3 || p.adjustments != 0 {
		t.Fatalf("items %+v adjustments %v", p.items, p.adjustments)
	}
	if p.items[0].Discount != 47 || p.items[1].Discount != 47 || p.items[2].OnSale() {
		t.Errorf("discounts = %v %v %v", p.items[0].Discount, p.items[1].Discount, p.items[2].Discount)
	}
}
