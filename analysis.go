package larder

import (
	"math"
	"sort"
	"strconv"
	"time"
)

// CadenceStatus says whether a regular purchase is due again.
type CadenceStatus string

// Cadence states, judged against a product's usual interval.
const (
	OnTrack CadenceStatus = "on-track"
	Due     CadenceStatus = "due"
	Overdue CadenceStatus = "overdue"
	Lapsed  CadenceStatus = "lapsed"
)

// AnalysisOptions controls what counts as a regular purchase.
type AnalysisOptions struct {
	// Today anchors how long ago each product was last bought.
	Today time.Time
	// MinTrips is the fewest separate days a product must be bought on.
	MinTrips int
	// MinSpan is the shortest time between first and last purchase.
	MinSpan time.Duration
}

// DefaultAnalysisOptions treats anything bought on four or more days over
// at least two months as regular.
func DefaultAnalysisOptions(today time.Time) AnalysisOptions {
	return AnalysisOptions{Today: today, MinTrips: 4, MinSpan: 60 * 24 * time.Hour}
}

// PricePoint is the unit price of one purchase.
type PricePoint struct {
	Date    time.Time
	Store   StoreID
	Shelf   Money
	Paid    Money
	Outlier bool
	// Sale is set when the purchase was discounted or marked on sale.
	Sale bool
	// Promotion is set when the store marked a sale without printing the
	// regular price, so Shelf is really the sale price and is left out of
	// shelf price trends.
	Promotion bool
	Quantity  float64
}

// YearPrice summarises a product's unit prices in one calendar year as
// quantity-weighted averages: the regular shelf price, and the price
// actually paid once sales and instant savings are taken off. Both use the
// same purchases and the same averaging, so sales are the only difference.
type YearPrice struct {
	Year      int   `json:"y"`
	Shelf     Money `json:"m"`
	Count     int   `json:"n"`
	Paid      Money `json:"p"`
	PaidCount int   `json:"pn"`
}

// Visit is one day a product was bought.
type Visit struct {
	Date     time.Time
	Quantity float64
	Paid     Money
	Sale     bool
}

// ProductStats summarises how often a product is bought and what it costs.
type ProductStats struct {
	ID       string
	Name     string
	Category string
	Keys     []ProductKey
	Stores   []StoreID
	Unit     Unit
	Visits   []Visit
	Quantity float64
	Spent    Money
	First    time.Time
	Last     time.Time
	// CadenceDays is the median number of days between purchases.
	CadenceDays   float64
	DaysSinceLast int
	Status        CadenceStatus
	Prices        []PricePoint
	Yearly        []YearPrice
	// PriceChange is the fractional change in median shelf price from the
	// first year with prices to the latest.
	PriceChange    float64
	PriceFrom      YearPrice
	PriceTo        YearPrice
	HasPriceChange bool
	// PaidChange is the same comparison using the average price actually
	// paid, so it reflects how often the product was bought on sale.
	PaidChange    float64
	PaidFrom      YearPrice
	PaidTo        YearPrice
	HasPaidChange bool
	Regular       bool
	Catalogued    bool
	// Purchases counts every line bought; SalePurchases those on sale.
	Purchases     int
	SalePurchases int
	// Saved is the total taken off by sales and instant savings.
	Saved Money
	// SaleDiscount is the median fraction taken off the shelf price when
	// the product was bought on sale with a known regular price.
	SaleDiscount    float64
	HasSaleDiscount bool
}

// SaleShare is the fraction of purchases made on sale.
func (s ProductStats) SaleShare() float64 {
	if s.Purchases == 0 {
		return 0
	}
	return float64(s.SalePurchases) / float64(s.Purchases)
}

// Trips is the number of separate days the product was bought.
func (s ProductStats) Trips() int { return len(s.Visits) }

// Analyse groups purchases into products, using the catalogue to name and
// merge them, and summarises each one's cadence and price history. Results
// are ordered by number of trips, most frequent first.
func Analyse(purchases []Purchase, cat *Catalogue, opts AnalysisOptions) []ProductStats {
	type group struct {
		id, name, category string
		catalogued         bool
		purchases          []Purchase
	}
	productOf := sameProductKeys(purchases)

	groups := map[string]*group{}
	var order []string
	for _, p := range purchases {
		if cat.Ignored(p.Key) {
			continue
		}
		id, name, category, catalogued := productOf[p.Key], "", "", false
		if e, ok := cat.Lookup(p.Key); ok {
			id, name, category, catalogued = "cat:"+e.Name, e.Name, e.Category, true
		}
		g := groups[id]
		if g == nil {
			g = &group{id: id, name: name, category: category, catalogued: catalogued}
			groups[id] = g
			order = append(order, id)
		}
		g.purchases = append(g.purchases, p)
	}

	out := make([]ProductStats, 0, len(groups))
	for _, id := range order {
		g := groups[id]
		s := summarise(g.purchases, opts)
		s.ID, s.Catalogued = g.id, g.catalogued
		if g.name != "" {
			s.Name = g.name
		}
		if g.category != "" {
			s.Category = g.category
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Trips() != out[j].Trips() {
			return out[i].Trips() > out[j].Trips()
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// sameProductKeys decides which store items are one product when the
// catalogue does not say. Stores renumber products over time, so items with
// the same description at the same store are merged, unless their typical
// prices differ enough to suggest different pack sizes sharing a name.
func sameProductKeys(purchases []Purchase) map[ProductKey]string {
	type keyInfo struct {
		key    ProductKey
		count  int
		prices []float64
		descs  map[string]int
	}
	info := map[ProductKey]*keyInfo{}
	for _, p := range purchases {
		k := info[p.Key]
		if k == nil {
			k = &keyInfo{key: p.Key, descs: map[string]int{}}
			info[p.Key] = k
		}
		k.count++
		k.prices = append(k.prices, float64(p.Item.UnitPrice()))
		k.descs[normaliseDescription(p.Item.Description)]++
	}

	byName := map[string][]*keyInfo{}
	for _, k := range info {
		name := string(k.key.Store()) + ":" + topKey(k.descs)
		byName[name] = append(byName[name], k)
	}

	out := map[ProductKey]string{}
	for name, keys := range byName {
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].count != keys[j].count {
				return keys[i].count > keys[j].count
			}
			return keys[i].key < keys[j].key
		})
		var anchors []float64
		for _, k := range keys {
			m := median(k.prices)
			cluster := -1
			for i, a := range anchors {
				if a > 0 && m/a < 1.5 && m/a > 1/1.5 {
					cluster = i
					break
				}
			}
			if cluster < 0 {
				cluster = len(anchors)
				anchors = append(anchors, m)
			}
			id := name
			if cluster > 0 {
				id += "#" + strconv.Itoa(cluster+1)
			}
			out[k.key] = id
		}
	}
	return out
}

func summarise(purchases []Purchase, opts AnalysisOptions) ProductStats {
	sort.SliceStable(purchases, func(i, j int) bool { return purchases[i].Date.Before(purchases[j].Date) })
	var s ProductStats

	names, departments, units := map[string]int{}, map[string]int{}, map[Unit]int{}
	keys, stores := map[ProductKey]bool{}, map[StoreID]bool{}
	for _, p := range purchases {
		names[p.Item.Description]++
		if p.Item.Department != "" {
			departments[p.Item.Department]++
		}
		units[p.Item.Quantity.Unit]++
		keys[p.Key], stores[p.Store] = true, true
	}
	s.Name, s.Category = topKey(names), topKey(departments)
	s.Unit = Each
	if units[Kilogram] > units[Each] {
		s.Unit = Kilogram
	}
	for k := range keys {
		s.Keys = append(s.Keys, k)
	}
	sort.Slice(s.Keys, func(i, j int) bool { return s.Keys[i] < s.Keys[j] })
	for st := range stores {
		s.Stores = append(s.Stores, st)
	}
	sort.Slice(s.Stores, func(i, j int) bool { return s.Stores[i] < s.Stores[j] })

	var discounts []float64
	for _, p := range purchases {
		item := p.Item
		s.Spent += item.Paid()
		s.Saved += item.Discount
		s.Purchases++
		if item.OnSale() {
			s.SalePurchases++
		}
		if item.Discount > 0 && item.Price > 0 {
			discounts = append(discounts, float64(item.Discount)/float64(item.Price))
		}
		if item.Quantity.Unit == s.Unit {
			s.Quantity += item.Quantity.Amount
		}
		if n := len(s.Visits); n > 0 && s.Visits[n-1].Date.Equal(p.Date) {
			v := &s.Visits[n-1]
			v.Quantity += item.Quantity.Amount
			v.Paid += item.Paid()
			v.Sale = v.Sale || item.OnSale()
		} else {
			s.Visits = append(s.Visits, Visit{Date: p.Date, Quantity: item.Quantity.Amount, Paid: item.Paid(), Sale: item.OnSale()})
		}
		if item.Quantity.Unit == s.Unit && !item.Estimated {
			s.Prices = append(s.Prices, PricePoint{
				Date: p.Date, Store: p.Store, Shelf: item.UnitPrice(), Paid: item.UnitPaid(),
				Sale: item.OnSale(), Promotion: item.Promotion && item.Discount == 0,
				Quantity: item.Quantity.Amount,
			})
		}
	}
	if len(discounts) > 0 {
		s.SaleDiscount, s.HasSaleDiscount = median(discounts), true
	}
	s.First, s.Last = s.Visits[0].Date, s.Visits[len(s.Visits)-1].Date

	markOutliers(s.Prices)
	s.Yearly = yearlyPrices(s.Prices)
	s.PriceChange, s.PriceFrom, s.PriceTo, s.HasPriceChange = priceChange(s.Yearly, shelfPrice)
	s.PaidChange, s.PaidFrom, s.PaidTo, s.HasPaidChange = priceChange(s.Yearly, paidPrice)

	s.CadenceDays = medianGap(s.Visits)
	s.DaysSinceLast = int(opts.Today.Sub(s.Last).Hours() / 24)
	s.Status = cadenceStatus(s.CadenceDays, s.DaysSinceLast)
	s.Regular = s.Trips() >= opts.MinTrips && s.Last.Sub(s.First) >= opts.MinSpan
	return s
}

// markOutliers flags unit prices far from the product's median, which are
// almost always OCR errors or a different pack size sharing a code.
func markOutliers(prices []PricePoint) {
	var values []float64
	for _, p := range prices {
		if !p.Promotion {
			values = append(values, float64(p.Shelf))
		}
	}
	if len(values) < 3 {
		return
	}
	med := median(values)
	for i := range prices {
		v := float64(prices[i].Shelf)
		prices[i].Outlier = !prices[i].Promotion && (v > 2*med || v < 0.5*med)
	}
}

// yearlyPrices averages each year's prices, weighting each purchase by the
// quantity bought.
func yearlyPrices(prices []PricePoint) []YearPrice {
	type acc struct {
		shelf, shelfQty, paid, paidQty float64
		shelfCount, paidCount          int
	}
	byYear := map[int]*acc{}
	for _, p := range prices {
		if p.Outlier {
			continue
		}
		a := byYear[p.Date.Year()]
		if a == nil {
			a = &acc{}
			byYear[p.Date.Year()] = a
		}
		qty := p.Quantity
		if qty <= 0 {
			qty = 1
		}
		if !p.Promotion {
			a.shelf += float64(p.Shelf) * qty
			a.shelfQty += qty
			a.shelfCount++
		}
		a.paid += float64(p.Paid) * qty
		a.paidQty += qty
		a.paidCount++
	}
	var out []YearPrice
	for y, a := range byYear {
		yp := YearPrice{Year: y, Count: a.shelfCount, PaidCount: a.paidCount,
			Paid: Money(math.Round(a.paid / a.paidQty))}
		if a.shelfQty > 0 {
			yp.Shelf = Money(math.Round(a.shelf / a.shelfQty))
		}
		out = append(out, yp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year < out[j].Year })
	return out
}

// priceMeasure picks one kind of yearly price and how many purchases it
// rests on.
type priceMeasure func(YearPrice) (Money, int)

func shelfPrice(y YearPrice) (Money, int) { return y.Shelf, y.Count }
func paidPrice(y YearPrice) (Money, int)  { return y.Paid, y.PaidCount }

// priceChange compares the earliest and latest years with at least two
// prices each, so a single sale or misread price cannot set the trend.
func priceChange(years []YearPrice, measure priceMeasure) (float64, YearPrice, YearPrice, bool) {
	var solid []YearPrice
	for _, y := range years {
		if v, n := measure(y); n >= 2 && v > 0 {
			solid = append(solid, y)
		}
	}
	if len(solid) < 2 {
		return 0, YearPrice{}, YearPrice{}, false
	}
	from, to := solid[0], solid[len(solid)-1]
	a, _ := measure(from)
	b, _ := measure(to)
	return float64(b)/float64(a) - 1, from, to, true
}

func medianGap(visits []Visit) float64 {
	if len(visits) < 2 {
		return 0
	}
	gaps := make([]float64, 0, len(visits)-1)
	for i := 1; i < len(visits); i++ {
		gaps = append(gaps, visits[i].Date.Sub(visits[i-1].Date).Hours()/24)
	}
	return median(gaps)
}

func cadenceStatus(cadence float64, since int) CadenceStatus {
	switch {
	case cadence <= 0:
		return Lapsed
	case float64(since) > 3*cadence && since > 60:
		return Lapsed
	case float64(since) > 1.5*cadence:
		return Overdue
	case float64(since) >= 0.85*cadence:
		return Due
	}
	return OnTrack
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

// IndexPoint is the price level of the household's regular basket in a
// year, relative to 100 in the first year: Index by regular shelf prices,
// Paid by what was actually paid after sales.
type IndexPoint struct {
	Year         int     `json:"year"`
	Index        float64 `json:"index"`
	Products     int     `json:"products"`
	Paid         float64 `json:"paid"`
	PaidProducts int     `json:"paidProducts"`
}

// BasketIndex chains year-over-year price changes of regular products,
// weighting each product by what was spent on it, to show how the cost of
// the household's own staples has moved. It is computed twice: once from
// regular shelf prices, showing what stores charge, and once from prices
// actually paid, which also reflects how much was bought on sale.
func BasketIndex(stats []ProductStats) []IndexPoint {
	years := map[int]bool{}
	for _, s := range stats {
		for _, y := range s.Yearly {
			years[y.Year] = true
		}
	}
	var ordered []int
	for y := range years {
		ordered = append(ordered, y)
	}
	sort.Ints(ordered)
	if len(ordered) == 0 {
		return nil
	}

	shelf := chainIndex(stats, ordered, shelfPrice)
	paid := chainIndex(stats, ordered, paidPrice)
	out := make([]IndexPoint, len(ordered))
	for i, y := range ordered {
		out[i] = IndexPoint{Year: y, Index: shelf[i].level, Products: shelf[i].n, Paid: paid[i].level, PaidProducts: paid[i].n}
	}
	return out
}

type chainStep struct {
	level float64
	n     int
}

func chainIndex(stats []ProductStats, years []int, measure priceMeasure) []chainStep {
	out := []chainStep{{level: 100}}
	for i := 1; i < len(years); i++ {
		var sumW, sumLog float64
		n := 0
		for _, s := range stats {
			if !s.Regular {
				continue
			}
			a, okA := yearPrice(s.Yearly, years[i-1], measure)
			b, okB := yearPrice(s.Yearly, years[i], measure)
			if !okA || !okB {
				continue
			}
			rel := math.Min(2, math.Max(0.5, float64(b)/float64(a)))
			w := s.Spent.Dollars()
			sumW += w
			sumLog += w * math.Log(rel)
			n++
		}
		level := out[len(out)-1].level
		if sumW > 0 {
			level *= math.Exp(sumLog / sumW)
		}
		out = append(out, chainStep{level: level, n: n})
	}
	return out
}

func yearPrice(years []YearPrice, year int, measure priceMeasure) (Money, bool) {
	for _, y := range years {
		if y.Year == year {
			v, n := measure(y)
			return v, n > 0 && v > 0
		}
	}
	return 0, false
}
