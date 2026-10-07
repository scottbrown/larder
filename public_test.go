package larder

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPublicStatsDropsSensitiveProducts(t *testing.T) {
	cat, err := ParseCatalogue([]byte("products:\n  - name: Wine\n    category: Alcohol\n    match: [costco:1]\n"))
	if err != nil {
		t.Fatal(err)
	}
	stats := []ProductStats{
		{Name: "Bananas", Category: "Produce", Keys: []ProductKey{"costco:30669"}},
		{Name: "Wine", Category: "Alcohol"},
		{Name: "Renu contact solution", Category: "Personal care"},
		{Name: "ADVIL JUNIOR"},
		{Name: "KS PROBIO 24"},
		{Name: "BEA HOMO MILK JG", Category: "Dairy"},
	}
	got := PublicStats(stats, cat)
	if len(got) != 2 || got[0].Name != "Bananas" || got[1].Name != "BEA HOMO MILK JG" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Keys != nil {
		t.Error("store codes kept")
	}

	custom, _ := ParseCatalogue([]byte("products: []\nprivate: [Produce]\n"))
	if !custom.PrivateCategory("produce") || custom.PrivateCategory("Alcohol") {
		t.Error("catalogue private categories not honoured")
	}
}

func TestWeekOf(t *testing.T) {
	for _, d := range []int{6, 7, 8, 12} { // Mon 6 Oct 2025 .. Sun 12 Oct
		if got := weekOf(day(2025, time.October, d)); !got.Equal(day(2025, time.October, 6)) {
			t.Errorf("weekOf(Oct %d) = %s", d, got.Format(time.DateOnly))
		}
	}
}

func TestPublicDashboardRoundsDatesAndHidesCodes(t *testing.T) {
	start := day(2025, time.January, 1) // a Wednesday
	var receipts []Receipt
	for i := range 5 {
		receipts = append(receipts, Receipt{
			File:  ReceiptFile{Date: start.AddDate(0, 0, 30*i), Store: Store{ID: "costco"}},
			Items: []LineItem{{Code: "30669", Description: "BANANAS", Quantity: Units(1), Price: 199}},
		})
	}
	cat, _ := NewCatalogue(nil, nil)
	today := start.AddDate(0, 0, 130)
	stats := PublicStats(Analyse(Purchases(receipts), cat, DefaultAnalysisOptions(today)), cat)
	data := NewDashboardData(receipts, stats, YearRange(2025, 2025), today, today, true)
	p := data.Products[0]
	if !data.Public || p.First != "2024-12-30" || p.Visits[0].Date != "2024-12-30" || p.Prices[0].Date != "2024-12-30" {
		t.Errorf("dates not rounded: %+v", p)
	}
	var buf bytes.Buffer
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"costco:30669", "2025-01-31", "2025-03-02"} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("public page leaks %s", leak)
		}
	}
}

func TestPurchasesSkipNonProducts(t *testing.T) {
	r := Receipt{File: ReceiptFile{Store: Store{ID: "walmart"}}, Items: []LineItem{
		{Description: "STARBUCKS GIFT/CRD", Quantity: Units(1), Price: 2500},
		{Description: "PLUS .07 CRF/EA", Quantity: Units(1), Price: 7},
		{Description: "2 FOR $6.00", Quantity: Units(1), Price: 300},
		{Description: "ADULT TICKETS", Quantity: Units(1), Price: 1299},
		{Description: "BANANAS", Quantity: Units(1), Price: 199},
	}}
	if ps := Purchases([]Receipt{r}); len(ps) != 1 || ps[0].Item.Description != "BANANAS" {
		t.Errorf("purchases = %+v", ps)
	}
}
