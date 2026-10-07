package larder

import "testing"

func TestParseCostcoItemsDiscountsAndMultiples(t *testing.T) {
	p := parseCostco([]string{
		"COSTCO WHOLESALE",
		"Anytown #1234",
		"2Y Member 111111111111",
		"39036 ROMAINE 8.99",
		"339054 PF GOLDFISH 13.99",
		"2014432 TPD/339054 3.00-",
		"2 @ 1.50",
		"23983 REG HOT DOG 3.00 G",
		"ENVIRO FEE C 0.09",
		"SUBTOTAL 22.07",
		"TAX 0.15",
	})
	if !p.found || p.subtotal != 2207 {
		t.Fatalf("subtotal = %v (found %v)", p.subtotal, p.found)
	}
	if len(p.items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(p.items), p.items)
	}
	if p.items[1].Code != "339054" || p.items[1].Discount != 300 || p.items[1].Paid() != 1099 {
		t.Errorf("goldfish = %+v", p.items[1])
	}
	if p.items[2].Quantity != Units(2) || p.items[2].UnitPrice() != 150 {
		t.Errorf("hot dogs = %+v", p.items[2])
	}
	if p.fees != 9 {
		t.Errorf("fees = %v", p.fees)
	}
}

func TestParseCostcoVoidRemovesItem(t *testing.T) {
	p := parseCostco([]string{
		"394485 KS BABY WIPE 21.99 GP",
		"1316144 TPD/394485 4.20-GP",
		"394485 KS BABY WIPE 21.99 GP",
		"1316144 TPD/394485 4.20-GP",
		"VOID",
		"394485 KS BABY WIPE 21.99-GP",
		"VOID",
		"1316144 TPD/394485 4.20 GP",
		"SUBTOTAL 17.79",
	})
	if len(p.items) != 1 || p.items[0].Paid() != 1779 {
		t.Fatalf("items = %+v", p.items)
	}
}

func TestParseCostcoTotalWithoutSubtotal(t *testing.T) {
	p := parseCostco([]string{
		"Order Number:",
		"1265738 ICE CREAM S 2.29 G",
		"TAX 0.11",
		"**** TOTAL 2.40",
	})
	if !p.found || p.subtotal != 229 || len(p.items) != 1 {
		t.Fatalf("parsed = %+v", p)
	}
}

func TestParseCostcoIgnoresOnlineOrders(t *testing.T) {
	p := parseCostco([]string{"Your Costco.ca Order Number 1 Was Received.", "Item # 5140023", "$49.99"})
	if len(p.items) != 0 || p.found {
		t.Errorf("online order parsed: %+v", p)
	}
}

func TestParseReceiptInfersSingleUnreadablePrice(t *testing.T) {
	file := ReceiptFile{Store: Store{ID: "costco"}}
	doc := docFromRows(
		"30669 BANANAS 1.99",
		"458 MILK 49",
		"118219 BUTTER 3.95",
		"SUBTOTAL 10.43",
	)
	r := ParseReceipt(file, doc)
	if r.Status() != Reconciled || len(r.Items) != 3 {
		t.Fatalf("status %s items %+v", r.Status(), r.Items)
	}
	if !r.Items[1].Estimated || r.Items[1].Price != 449 {
		t.Errorf("milk = %+v", r.Items[1])
	}
}

func TestParseReceiptUsesSiblingPrice(t *testing.T) {
	file := ReceiptFile{Store: Store{ID: "costco"}}
	r := ParseReceipt(file, docFromRows(
		"1134671 MULTI CHEERI .39",
		"1134671 MULTI CHEERI 8.39",
		"SUBTOTAL 16.78",
	))
	if r.Status() != Reconciled || r.Items[0].Price != 839 || !r.Items[0].Estimated {
		t.Errorf("status %s items %+v", r.Status(), r.Items)
	}
}

// docFromRows lays text out one row per line, as Vision would report a
// straight scan.
func docFromRows(rows ...string) Document {
	page := Page{Width: 600, Height: float64(40 * len(rows))}
	for i, r := range rows {
		y := 1 - float64(i+1)/float64(len(rows)+1)
		page.Observations = append(page.Observations, Observation{Text: r, X: 0.05, Y: y, W: 0.9, H: 0.5 / float64(len(rows)+1)})
	}
	return Document{page}
}
