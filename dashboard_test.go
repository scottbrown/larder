package larder

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestDashboardDataAndRender(t *testing.T) {
	start := day(2024, time.January, 5)
	var receipts []Receipt
	for i := range 8 {
		price := Money(199)
		if i >= 4 {
			price = 229
		}
		receipts = append(receipts, Receipt{
			File:     ReceiptFile{Date: start.AddDate(0, 0, 100*i), Store: Store{ID: "costco"}},
			Items:    []LineItem{{Code: "30669", Description: "BANANAS", Quantity: Units(1), Price: price, Discount: Money(10 * (i % 2))}},
			Subtotal: price - Money(10*(i%2)),
			found:    true,
		})
	}
	receipts = append(receipts, Receipt{
		File:  ReceiptFile{Date: start, Store: Store{ID: "coop"}},
		Items: []LineItem{{Description: "Birthday cake", Quantity: Units(1), Price: 2499}},
	})
	today := start.AddDate(0, 0, 100*7+10)
	cat, _ := NewCatalogue(nil, nil)
	stats := Analyse(Purchases(receipts), cat, DefaultAnalysisOptions(today))
	data := NewDashboardData(receipts, stats, YearRange(2024, 2025), today, today, false)

	if data.Receipts != 9 || len(data.Products) != 1 || data.Others != 1 || data.Items != 9 {
		t.Fatalf("data = %+v", data)
	}
	p := data.Products[0]
	if p.Name != "BANANAS" || p.Change == nil || p.ChangeFrom.Year != 2024 || p.ChangeTo.Year != 2025 || len(p.Visits) != 8 {
		t.Errorf("product = %+v", p)
	}
	if data.Saved != 40 || data.SaleItems != 4 || p.OnSale != 4 || p.Purchases != 8 || p.Saved != 40 || p.SaleCut == nil {
		t.Errorf("sales: data %v/%d, product %d/%d saved %v", data.Saved, data.SaleItems, p.OnSale, p.Purchases, p.Saved)
	}
	if len(data.Quality) != 2 || data.Quality[0].Store != "costco" || data.Quality[0].Reconciled != 8 || data.Quality[1].Unverified != 1 {
		t.Errorf("quality = %+v", data.Quality)
	}
	if len(data.Index) != 2 || data.Index[1].Index <= 100 {
		t.Errorf("index = %+v", data.Index)
	}

	var buf bytes.Buffer
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatal(err)
	}
	page := buf.String()
	for _, want := range []string{"<title>Larder</title>", `"name":"BANANAS"`, `"store":"costco"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %s", want)
		}
	}
	if strings.Contains(page, "Birthday cake") {
		t.Error("one-off product leaked into the page")
	}
}

func TestDashboardEscapesScriptContent(t *testing.T) {
	data := DashboardData{Products: []DashboardProduct{{Name: "</script><script>alert(1)</script>"}}}
	var buf bytes.Buffer
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "<script>alert(1)") {
		t.Error("product name not escaped")
	}
}

func TestRenderDashboardInlinesScript(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderDashboard(&buf, DashboardData{}); err != nil {
		t.Fatal(err)
	}
	page := buf.String()
	if !strings.Contains(page, "<script>const DATA = ") || !strings.Contains(page, dashboardScript) {
		t.Error("self-contained page does not inline its data and script")
	}
	if strings.Contains(page, "<script src=") {
		t.Error("self-contained page loads an external script")
	}
}

func TestRenderDashboardFilesLoadsScriptsFromPrefix(t *testing.T) {
	data := DashboardData{Products: []DashboardProduct{{Name: "BANANAS"}}}
	var page, dataJS, script bytes.Buffer
	files := DashboardFiles{Page: &page, Data: &dataJS, Script: &script}
	if err := RenderDashboardFiles(files, data, AssetPrefix("/larder/")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<script src="/larder/data.js"></script>`, `<script src="/larder/app.js"></script>`} {
		if !strings.Contains(page.String(), want) {
			t.Errorf("page missing %s", want)
		}
	}
	if strings.Contains(page.String(), "<script>") {
		t.Error("page still has an inline script")
	}
	if !strings.HasPrefix(dataJS.String(), "const DATA = ") || !strings.Contains(dataJS.String(), `"name":"BANANAS"`) {
		t.Errorf("data file = %q", dataJS.String())
	}
	if script.String() != dashboardScript {
		t.Error("script file differs from the dashboard script")
	}
}

func TestRenderDashboardFilesEscapesScriptContent(t *testing.T) {
	data := DashboardData{Products: []DashboardProduct{{Name: "</script><script>alert(1)</script>"}}}
	var page, dataJS, script bytes.Buffer
	if err := RenderDashboardFiles(DashboardFiles{Page: &page, Data: &dataJS, Script: &script}, data, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dataJS.String(), "</script>") {
		t.Error("product name not escaped in the data file")
	}
}

func TestAssetPrefixURL(t *testing.T) {
	for _, tc := range []struct {
		prefix AssetPrefix
		want   string
	}{
		{"/larder/", "/larder/app.js"},
		{"/larder", "/larder/app.js"},
		{"https://example.com/larder/", "https://example.com/larder/app.js"},
		{"", "app.js"},
	} {
		if got := tc.prefix.URL(DashboardScriptFile); got != tc.want {
			t.Errorf("AssetPrefix(%q).URL = %q, want %q", tc.prefix, got, tc.want)
		}
	}
}
