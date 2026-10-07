package larder

import (
	"math"
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func purchase(key ProductKey, date time.Time, price Money) Purchase {
	return Purchase{
		Date: date, Store: key.Store(), Key: key,
		Item: LineItem{Description: string(key), Quantity: Units(1), Price: price},
	}
}

func TestAnalyseCadenceAndPrices(t *testing.T) {
	var ps []Purchase
	start := day(2024, time.January, 1)
	for i := range 10 {
		price := Money(500)
		if i >= 6 {
			price = 600
		}
		ps = append(ps, purchase("costco:30669", start.AddDate(0, 0, 73*i), price))
	}
	ps = append(ps, purchase("costco:30669", start.AddDate(0, 0, 73*9), 6000)) // OCR outlier, same day
	ps = append(ps, purchase("coop:CAKE", start, 1000))

	cat, _ := NewCatalogue([]CatalogueEntry{{Name: "Bananas", Category: "Produce", Match: []string{"costco:30669"}}}, nil)
	today := start.AddDate(0, 0, 73*9+80)
	stats := Analyse(ps, cat, DefaultAnalysisOptions(today))
	if len(stats) != 2 {
		t.Fatalf("got %d products", len(stats))
	}
	b := stats[0]
	if b.Name != "Bananas" || b.Category != "Produce" || !b.Catalogued || !b.Regular {
		t.Errorf("bananas = %+v", b)
	}
	if b.Trips() != 10 || b.CadenceDays != 73 {
		t.Errorf("trips %d cadence %v", b.Trips(), b.CadenceDays)
	}
	if b.Status != Due {
		t.Errorf("status = %s", b.Status)
	}
	outliers := 0
	for _, p := range b.Prices {
		if p.Outlier {
			outliers++
		}
	}
	if outliers != 1 {
		t.Errorf("outliers = %d", outliers)
	}
	if !b.HasPriceChange || math.Abs(b.PriceChange-0.2) > 1e-9 {
		t.Errorf("price change = %v (%v)", b.PriceChange, b.Yearly)
	}
	if stats[1].Regular || stats[1].Status != Lapsed {
		t.Errorf("one-off cake = %+v", stats[1])
	}

	ignoring, _ := NewCatalogue(nil, []string{"coop:CAKE"})
	if got := Analyse(ps, ignoring, DefaultAnalysisOptions(today)); len(got) != 1 {
		t.Errorf("ignored product analysed")
	}
}

func TestAnalyseSales(t *testing.T) {
	start := day(2025, time.January, 1)
	ps := []Purchase{
		purchase("costco:339054", start, 1399),
		purchase("costco:339054", start.AddDate(0, 0, 30), 1399),
		purchase("costco:339054", start.AddDate(0, 0, 60), 1399),
		purchase("costco:339054", start.AddDate(0, 0, 90), 1399),
	}
	ps[1].Item.Discount = 300
	ps[2].Item.Discount = 400
	ps[3].Item.Price, ps[3].Item.Promotion = 999, true

	cat, _ := NewCatalogue(nil, nil)
	s := Analyse(ps, cat, DefaultAnalysisOptions(start.AddDate(0, 4, 0)))[0]
	if s.Purchases != 4 || s.SalePurchases != 3 || s.SaleShare() != 0.75 {
		t.Errorf("sale counts %d/%d", s.SalePurchases, s.Purchases)
	}
	if s.Saved != 700 {
		t.Errorf("saved = %v", s.Saved)
	}
	if !s.HasSaleDiscount || math.Abs(s.SaleDiscount-0.25017869907076483) > 1e-9 {
		t.Errorf("sale discount = %v", s.SaleDiscount)
	}
	if !s.Visits[1].Sale || s.Visits[0].Sale {
		t.Errorf("visits = %+v", s.Visits)
	}
	if !s.Prices[3].Promotion || s.Yearly[0].Shelf != 1399 || s.Yearly[0].Count != 3 {
		t.Errorf("promotion leaked into trend: %+v %+v", s.Prices[3], s.Yearly)
	}
	if (ProductStats{}).SaleShare() != 0 {
		t.Error("empty sale share")
	}
}

func TestCadenceStatus(t *testing.T) {
	tests := []struct {
		cadence float64
		since   int
		want    CadenceStatus
	}{
		{14, 3, OnTrack}, {14, 13, Due}, {14, 25, Overdue}, {14, 100, Lapsed}, {0, 1, Lapsed},
	}
	for _, tt := range tests {
		if got := cadenceStatus(tt.cadence, tt.since); got != tt.want {
			t.Errorf("cadenceStatus(%v, %d) = %s, want %s", tt.cadence, tt.since, got, tt.want)
		}
	}
}

func yp(year int, shelf, paid Money) YearPrice {
	return YearPrice{Year: year, Shelf: shelf, Count: 3, Paid: paid, PaidCount: 3}
}

func TestBasketIndexChainsYearlyChanges(t *testing.T) {
	stats := []ProductStats{
		{Regular: true, Spent: 1000, Yearly: []YearPrice{yp(2019, 100, 100), yp(2020, 110, 99), yp(2021, 121, 121)}},
		{Regular: true, Spent: 1000, Yearly: []YearPrice{yp(2019, 200, 200), yp(2020, 220, 198)}},
		{Regular: false, Spent: 9999, Yearly: []YearPrice{yp(2019, 100, 100), yp(2020, 900, 900)}},
	}
	idx := BasketIndex(stats)
	if len(idx) != 3 || idx[0].Index != 100 || idx[0].Paid != 100 {
		t.Fatalf("index = %+v", idx)
	}
	if math.Abs(idx[1].Index-110) > 1e-9 || idx[1].Products != 2 {
		t.Errorf("2020 shelf = %+v", idx[1])
	}
	if math.Abs(idx[1].Paid-99) > 1e-9 || idx[1].PaidProducts != 2 {
		t.Errorf("2020 paid = %+v", idx[1])
	}
	if math.Abs(idx[2].Index-121) > 1e-9 || idx[2].Products != 1 {
		t.Errorf("2021 shelf = %+v", idx[2])
	}
	if math.Abs(idx[2].Paid-121) > 1e-9 {
		t.Errorf("2021 paid = %+v", idx[2])
	}
	if BasketIndex(nil) != nil {
		t.Error("empty stats produced an index")
	}
}

func TestPaidChangeReflectsSales(t *testing.T) {
	var ps []Purchase
	for i := range 4 {
		ps = append(ps, purchase("costco:339054", day(2024, time.March, 1+7*i), 1000))
		later := purchase("costco:339054", day(2025, time.March, 1+7*i), 1200)
		if i < 3 {
			later.Item.Discount = 300 // usually bought on sale the next year
		}
		ps = append(ps, later)
	}
	cat, _ := NewCatalogue(nil, nil)
	s := Analyse(ps, cat, DefaultAnalysisOptions(day(2025, time.June, 1)))[0]
	if !s.HasPriceChange || math.Abs(s.PriceChange-0.2) > 1e-9 {
		t.Errorf("shelf change = %v", s.PriceChange)
	}
	// 2025 paid: (900*3 + 1200) / 4 = 975, against 1000 the year before.
	if !s.HasPaidChange || math.Abs(s.PaidChange-(-0.025)) > 1e-9 || s.PaidTo.Paid != 975 {
		t.Errorf("paid change = %v (%+v)", s.PaidChange, s.PaidTo)
	}
}

func TestMedian(t *testing.T) {
	if median(nil) != 0 || median([]float64{3, 1, 2}) != 2 || median([]float64{4, 1, 3, 2}) != 2.5 {
		t.Error("median wrong")
	}
}

func TestReceiptStatus(t *testing.T) {
	item := LineItem{Quantity: Units(1), Price: 1000}
	r := Receipt{Items: []LineItem{item}, Subtotal: 1000, found: true}
	if r.Status() != Reconciled {
		t.Errorf("exact = %s", r.Status())
	}
	r.Subtotal = 1100
	if r.Status() != Mismatch {
		t.Errorf("short = %s", r.Status())
	}
	r.TaxIncluded = true
	if r.Status() != Reconciled {
		t.Errorf("tax-inclusive = %s", r.Status())
	}
	if (Receipt{}).Status() != Unverified {
		t.Error("empty receipt verified")
	}
	if (LineItem{Price: 300}).UnitPrice() != 300 {
		t.Error("zero quantity unit price")
	}
}

func TestAnalyseKeepsPackSizesApart(t *testing.T) {
	start := day(2024, time.January, 1)
	var ps []Purchase
	for i := range 4 {
		small := purchase("costco:725236", start.AddDate(0, 0, 30*i), 699)
		small.Item.Description = "BROCCOLI"
		renumbered := purchase("costco:900001", start.AddDate(0, 0, 30*i+5), 729)
		renumbered.Item.Description = "BROCCOLI"
		large := purchase("costco:1453677", start.AddDate(0, 0, 30*i+10), 1399)
		large.Item.Description = "BROCCOLI"
		ps = append(ps, small, renumbered, large)
	}
	cat, _ := NewCatalogue(nil, nil)
	stats := Analyse(ps, cat, DefaultAnalysisOptions(start.AddDate(1, 0, 0)))
	if len(stats) != 2 {
		t.Fatalf("got %d products, want 2", len(stats))
	}
	if len(stats[0].Keys) != 2 || len(stats[1].Keys) != 1 {
		t.Errorf("keys = %v / %v", stats[0].Keys, stats[1].Keys)
	}
}
