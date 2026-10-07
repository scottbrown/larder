package larder

import "testing"

func TestParseGenericSobeysSavings(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"SOBEYS ANYTOWN",
		"Served by: SC022",
		"SunRype Apple Juice $2.50",
		"1 @ 2/ $5.00",
		"YOU SAVED $0.29",
		"+EHC $0.04",
		"+Deposit $0.10 R",
		"Summer Sausage 6940100169 $3.00 C",
		"SUBTOTAL $5.64",
		"TOTAL $5.64",
	})
	if !p.found || p.subtotal != 564 || p.taxIncluded {
		t.Fatalf("subtotal %v found %v", p.subtotal, p.found)
	}
	if len(p.items) != 2 {
		t.Fatalf("items %+v", p.items)
	}
	if p.items[0].Price != 279 || p.items[0].Discount != 29 || p.items[0].Paid() != 250 {
		t.Errorf("juice = %+v", p.items[0])
	}
	if p.items[1].Code != "6940100169" || p.items[1].Description != "Summer Sausage" {
		t.Errorf("sausage = %+v", p.items[1])
	}
	if p.fees != 14 {
		t.Errorf("fees = %v", p.fees)
	}
}

func TestParseGenericSaveOnCardSavings(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"save-on-foods #123",
		"Everything Bagel 4.99",
		"Card $3.45 Save -1.54",
		"R/HFlour 13.98",
		"2 @ 6.99",
		"Sub Total $17.43",
	})
	if len(p.items) != 2 || p.subtotal != 1743 {
		t.Fatalf("parsed %+v", p)
	}
	if p.items[0].Paid() != 345 || p.items[1].Quantity != Units(2) || p.items[1].UnitPrice() != 699 {
		t.Errorf("items %+v", p.items)
	}
}

func TestParseGenericTnTPendingDescription(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"GROCERY",
		"MEIJI VANILLA YAN YAN W $1.99 G",
		"LKK SA CHA SAUCE",
		"2 @ $5.37ea. W $10.74",
		"SUB TOTAL $12.73",
	})
	if len(p.items) != 2 {
		t.Fatalf("items %+v", p.items)
	}
	if p.items[0].Description != "MEIJI VANILLA YAN YAN" || p.items[0].Department != "Grocery" {
		t.Errorf("first = %+v", p.items[0])
	}
	if p.items[1].Description != "LKK SA CHA SAUCE" || p.items[1].Price != 1074 || p.items[1].Quantity != Units(2) {
		t.Errorf("second = %+v", p.items[1])
	}
}

func TestParseGenericBulkBarnWeights(t *testing.T) {
	p := parseGeneric(genericDialect{startAfter: "TRANSACTION:"})([]string{
		"Sale $4.26",
		"TOTAL $4.26",
		"Transaction: 72410289291",
		"PANKO BREAD CRUMBS $0.43D",
		"0.070 kg @ $6.15 /kg",
		"Sub-Total: $0.43",
	})
	if len(p.items) != 1 || p.subtotal != 43 || p.items[0].Quantity != Kilograms(0.07) {
		t.Fatalf("parsed %+v", p)
	}
	if p.items[0].UnitPrice() != 614 {
		t.Errorf("unit price %v", p.items[0].UnitPrice())
	}
}

func TestParseGenericCoopSlipBeforeBalance(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"CALGARY CO-OP",
		"JT MERLOT 4L * $36.99 G",
		"PLUS .25 DEP/EA $0.25",
		"PRESCRIPTION SALE $9.16",
		"TYPE: Purchase",
		"ACCT: MASTERCARD 37.24",
		"2 BALANCE DUE $37.24",
	})
	if len(p.items) != 1 || p.fees != 25 || !p.found || !p.taxIncluded {
		t.Fatalf("parsed %+v", p)
	}
	if p.items[0].Description != "JT MERLOT 4L" {
		t.Errorf("description %q", p.items[0].Description)
	}
}

func TestParseGenericWeighedPendingAndPounds(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"BANANAS",
		"2.20 lb @ $0.68/lb $1.50",
		"SUBTOTAL $1.50",
	})
	if len(p.items) != 1 || p.items[0].Quantity.Unit != Kilogram || p.items[0].Price != 150 {
		t.Fatalf("parsed %+v", p.items)
	}
}

func TestFirstAmount(t *testing.T) {
	if m, ok := firstAmount("$5.37ea. W $10.74"); !ok || m != 537 {
		t.Errorf("firstAmount = %v %v", m, ok)
	}
	if _, ok := firstAmount("none"); ok {
		t.Error("found an amount in text without one")
	}
}

func TestParseGenericSaleMarkersAndNegativeDollar(t *testing.T) {
	p := parseGeneric(genericDialect{})([]string{
		"(SALE) T&T TRADITIONAL TOFU W $1.88",
		"(SALE) GREEN ONION",
		"2 @ 2/$0.88 W $0.88",
		"PANKO BREAD CRUMBS $6.90D",
		normaliseLine("20% DISCOUNT $-1.38TD"),
		"SUB TOTAL $8.28",
	})
	if len(p.items) != 3 {
		t.Fatalf("items %+v", p.items)
	}
	if !p.items[0].Promotion || p.items[0].Description != "T&T TRADITIONAL TOFU" || !p.items[0].OnSale() {
		t.Errorf("tofu = %+v", p.items[0])
	}
	if !p.items[1].Promotion || p.items[1].Description != "GREEN ONION" {
		t.Errorf("onion = %+v", p.items[1])
	}
	if p.items[2].Discount != 138 || p.items[2].Promotion {
		t.Errorf("panko = %+v", p.items[2])
	}
}
