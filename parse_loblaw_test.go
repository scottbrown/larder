package larder

import "testing"

func TestParseLoblaw(t *testing.T) {
	p := parseLoblaw([]string{
		"REAL CANADIAN SUPERSTORE",
		"21-GROCERY",
		"06148305180 ROOSTER RICE STK MRJ",
		"$1.49 lmt 4, $1.99 ea",
		"1 @ $1.49 ea 1.49",
		"(2)06321128421 CAMS CKY CRMY TH MRJ",
		"(2)06321128362 CAMS CKY BEEF MRJ",
		"$3.49 ea or 4/$10.00",
		"4 @ 4/$10.00 10.00",
		"22-DAIRY",
		"06570010026 BEA HOMO MILK JG RQ 6.18",
		"RECYCLING FEE 0.06",
		"DEPOSIT 1 0.25",
		"27-PRODUCE",
		"4562 CARROT",
		"0.880 kg @ $2.80/kg 2.46",
		"05550001380 CLX LIQ HE CONC",
		"GPMRJ 3.48",
		"41-HOME",
		"(4)9 PLASTIC BAGS",
		"4 @ $0.05 0.20",
		"*11302 PC CHARITY Q 2.00",
		"SUBTOTAL 26.12",
	})
	if !p.found || p.subtotal != 2612 {
		t.Fatalf("subtotal %v found %v", p.subtotal, p.found)
	}
	want := []struct {
		code  ProductCode
		dept  string
		price Money
		disc  Money
		qty   Quantity
	}{
		{"6148305180", "Grocery", 199, 50, Units(1)},
		{"6321128421", "Grocery", 698, 198, Units(2)},
		{"6321128362", "Grocery", 698, 198, Units(2)},
		{"6570010026", "Dairy", 618, 0, Units(1)},
		{"4562", "Produce", 246, 0, Kilograms(0.88)},
		{"5550001380", "Produce", 348, 0, Units(1)},
	}
	if len(p.items) != len(want) {
		t.Fatalf("got %d items: %+v", len(p.items), p.items)
	}
	for i, w := range want {
		got := p.items[i]
		if got.Code != w.code || got.Department != w.dept || got.Price != w.price || got.Discount != w.disc || got.Quantity != w.qty {
			t.Errorf("item %d = %+v, want %+v", i, got, w)
		}
	}
	if p.items[0].Description != "ROOSTER RICE STK" {
		t.Errorf("flags not stripped: %q", p.items[0].Description)
	}
	if p.fees != 6+25+20+200 {
		t.Errorf("fees = %v", p.fees)
	}
	var sum Money
	for _, i := range p.items {
		sum += i.Paid()
	}
	if sum+p.fees != p.subtotal {
		t.Errorf("items %v + fees %v != subtotal %v", sum, p.fees, p.subtotal)
	}
}

func TestParseLoblawTareWeights(t *testing.T) {
	p := parseLoblaw([]string{
		"25-NATURAL FOODS",
		"06477711692 R.W.JELLY BEANS 0.240 kg Gross GMRJ",
		"-0.005 kg Tare =",
		"0.235 kg Net @ $6.80/kg 1.60",
		"SUBTOTAL 1.60",
	})
	if len(p.items) != 1 {
		t.Fatalf("items %+v", p.items)
	}
	got := p.items[0]
	if got.Description != "R.W.JELLY BEANS" || got.Quantity != Kilograms(0.235) || got.Price != 160 {
		t.Errorf("item = %+v", got)
	}
}
