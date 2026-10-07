package larder

import (
	_ "embed"
	"html/template"
	"io"
	"sort"
	"time"
)

//go:embed dashboard.tmpl.html
var dashboardTemplate string

// DashboardData is everything the dashboard page renders, serialised into
// the page as JSON.
type DashboardData struct {
	Generated string             `json:"generated"`
	From      string             `json:"from"`
	To        string             `json:"to"`
	Today     string             `json:"today"`
	Receipts  int                `json:"receipts"`
	Saved     Money              `json:"saved"`
	SaleItems int                `json:"saleItems"`
	Items     int                `json:"items"`
	Public    bool               `json:"public"`
	Stores    []DashboardStore   `json:"stores"`
	Quality   []StoreQuality     `json:"quality"`
	Index     []IndexPoint       `json:"index"`
	Products  []DashboardProduct `json:"products"`
	Others    int                `json:"others"`
}

// DashboardStore names a store for display.
type DashboardStore struct {
	ID   StoreID `json:"id"`
	Name string  `json:"name"`
}

// StoreQuality counts how many of a store's receipts reconciled.
type StoreQuality struct {
	Store      StoreID `json:"store"`
	Receipts   int     `json:"receipts"`
	Reconciled int     `json:"reconciled"`
	Mismatch   int     `json:"mismatch"`
	Unverified int     `json:"unverified"`
	Items      int     `json:"items"`
}

// DashboardProduct is a regular product in the compact form the page uses.
// Money is in cents; dates are YYYY-MM-DD.
type DashboardProduct struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Category   string        `json:"category"`
	Stores     []StoreID     `json:"stores"`
	Keys       []ProductKey  `json:"keys"`
	Unit       Unit          `json:"unit"`
	Trips      int           `json:"trips"`
	Cadence    float64       `json:"cadence"`
	Since      int           `json:"since"`
	Status     CadenceStatus `json:"status"`
	First      string        `json:"first"`
	Last       string        `json:"last"`
	Spent      Money         `json:"spent"`
	Quantity   float64       `json:"quantity"`
	Change     *float64      `json:"change"`
	ChangeFrom *YearPrice    `json:"changeFrom"`
	ChangeTo   *YearPrice    `json:"changeTo"`
	PaidChange *float64      `json:"paidChange"`
	PaidFrom   *YearPrice    `json:"paidFrom"`
	PaidTo     *YearPrice    `json:"paidTo"`
	Catalogued bool          `json:"catalogued"`
	Purchases  int           `json:"purchases"`
	OnSale     int           `json:"onSale"`
	Saved      Money         `json:"saved"`
	SaleCut    *float64      `json:"saleCut"`
	Yearly     []YearPrice   `json:"yearly"`
	Visits     []dashVisit   `json:"visits"`
	Prices     []dashPricePt `json:"prices"`
}

type dashVisit struct {
	Date     string  `json:"d"`
	Quantity float64 `json:"q"`
	Paid     Money   `json:"p"`
	Sale     bool    `json:"s,omitempty"`
}

type dashPricePt struct {
	Date    string  `json:"d"`
	Store   StoreID `json:"s"`
	Shelf   Money   `json:"v"`
	Paid    Money   `json:"p"`
	Outlier bool    `json:"o,omitempty"`
	Sale    bool    `json:"sl,omitempty"`
	Marked  bool    `json:"mk,omitempty"`
}

// NewDashboardData assembles the page's data from parsed receipts and the
// product analysis, keeping full detail only for regular products. A public
// dashboard should be given PublicStats; it also rounds every date to the
// start of its week.
func NewDashboardData(receipts []Receipt, stats []ProductStats, r DateRange, today, generated time.Time, public bool) DashboardData {
	day := func(t time.Time) string {
		if public {
			t = weekOf(t)
		}
		return t.Format(time.DateOnly)
	}
	d := DashboardData{
		Public:    public,
		Generated: generated.Format("2006-01-02 15:04"),
		From:      r.From.Format(time.DateOnly),
		To:        earlier(r.To, today).Format(time.DateOnly),
		Today:     today.Format(time.DateOnly),
		Receipts:  len(receipts),
		Index:     BasketIndex(stats),
	}
	for _, s := range Stores {
		d.Stores = append(d.Stores, DashboardStore{ID: s.ID, Name: s.Name})
	}
	d.Quality = receiptQuality(receipts)
	for _, s := range stats {
		d.Items += s.Purchases
		d.Saved += s.Saved
		d.SaleItems += s.SalePurchases
	}

	for _, s := range stats {
		if !s.Regular {
			d.Others++
			continue
		}
		p := DashboardProduct{
			ID: s.ID, Name: s.Name, Category: s.Category, Stores: s.Stores, Keys: s.Keys,
			Unit: s.Unit, Trips: s.Trips(), Cadence: s.CadenceDays, Since: s.DaysSinceLast,
			Status: s.Status, First: day(s.First), Last: day(s.Last),
			Spent: s.Spent, Quantity: s.Quantity, Catalogued: s.Catalogued, Yearly: s.Yearly,
			Purchases: s.Purchases, OnSale: s.SalePurchases, Saved: s.Saved,
		}
		if s.HasPaidChange {
			change, from, to := s.PaidChange, s.PaidFrom, s.PaidTo
			p.PaidChange, p.PaidFrom, p.PaidTo = &change, &from, &to
		}
		if s.HasSaleDiscount {
			cut := s.SaleDiscount
			p.SaleCut = &cut
		}
		if s.HasPriceChange {
			change, from, to := s.PriceChange, s.PriceFrom, s.PriceTo
			p.Change, p.ChangeFrom, p.ChangeTo = &change, &from, &to
		}
		for _, v := range s.Visits {
			p.Visits = append(p.Visits, dashVisit{Date: day(v.Date), Quantity: v.Quantity, Paid: v.Paid, Sale: v.Sale})
		}
		for _, pp := range s.Prices {
			p.Prices = append(p.Prices, dashPricePt{
				Date: day(pp.Date), Store: pp.Store, Shelf: pp.Shelf, Paid: pp.Paid, Outlier: pp.Outlier,
				Sale: pp.Sale, Marked: pp.Promotion,
			})
		}
		if public {
			p.Since = int(today.Sub(weekOf(s.Last)).Hours() / 24)
		}
		d.Products = append(d.Products, p)
	}
	return d
}

func receiptQuality(receipts []Receipt) []StoreQuality {
	byStore := map[StoreID]*StoreQuality{}
	for _, r := range receipts {
		q := byStore[r.File.Store.ID]
		if q == nil {
			q = &StoreQuality{Store: r.File.Store.ID}
			byStore[r.File.Store.ID] = q
		}
		q.Receipts++
		q.Items += len(r.Items)
		switch r.Status() {
		case Reconciled:
			q.Reconciled++
		case Mismatch:
			q.Mismatch++
		default:
			q.Unverified++
		}
	}
	out := make([]StoreQuality, 0, len(byStore))
	for _, q := range byStore {
		out = append(out, *q)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Receipts > out[j].Receipts })
	return out
}

// RenderDashboard writes the self-contained HTML dashboard.
func RenderDashboard(w io.Writer, data DashboardData) error {
	tmpl, err := template.New("dashboard").Parse(dashboardTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

func earlier(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
