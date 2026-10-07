package larder

import (
	"math"
	"strings"
)

// Unit is the measure an item is sold by.
type Unit string

// Units an item can be sold by.
const (
	Each     Unit = "each"
	Kilogram Unit = "kg"
)

// Quantity is how much of an item was bought.
type Quantity struct {
	Amount float64
	Unit   Unit
}

// Units returns a quantity of n items.
func Units(n float64) Quantity { return Quantity{Amount: n, Unit: Each} }

// Kilograms returns a weighed quantity.
func Kilograms(kg float64) Quantity { return Quantity{Amount: kg, Unit: Kilogram} }

// ProductCode is the item number, UPC or PLU a store prints for an item.
type ProductCode string

// LineItem is one product purchased on a receipt.
type LineItem struct {
	Code        ProductCode
	Description string
	Department  string
	Quantity    Quantity
	// Price is the shelf price of the whole line before discounts.
	Price Money
	// Discount is the amount taken off Price, as a positive number.
	Discount Money
	// Estimated is set when OCR could not read the price and it was
	// inferred from the receipt's subtotal.
	Estimated bool
	// Promotion is set when the store marked the item as on sale without
	// printing its regular price, so Price is the sale price.
	Promotion bool
}

// OnSale reports whether the item was bought at a reduced price.
func (i LineItem) OnSale() bool { return i.Discount > 0 || i.Promotion }

// Paid is what the line actually cost.
func (i LineItem) Paid() Money { return i.Price - i.Discount }

// UnitPrice is the shelf price per unit or per kilogram.
func (i LineItem) UnitPrice() Money { return i.per(i.Price) }

// UnitPaid is the price actually paid per unit or per kilogram.
func (i LineItem) UnitPaid() Money { return i.per(i.Paid()) }

func (i LineItem) per(m Money) Money {
	if i.Quantity.Amount <= 0 {
		return m
	}
	return Money(math.Round(float64(m) / i.Quantity.Amount))
}

// Status describes whether a receipt's items add up to its printed subtotal.
type Status string

// Reconciliation outcomes.
const (
	Reconciled Status = "reconciled"
	Mismatch   Status = "mismatch"
	Unverified Status = "unverified"
)

// Receipt is the parsed content of one scanned receipt.
type Receipt struct {
	File  ReceiptFile
	Items []LineItem
	// Fees covers deposits, recycling and bag charges, and donations.
	Fees Money
	// Adjustments are receipt-level discounts not tied to one item.
	Adjustments Money
	Subtotal    Money
	// TaxIncluded is set when the only printed figure includes tax.
	TaxIncluded bool
	found       bool
}

// ItemTotal sums what was paid for every item.
func (r Receipt) ItemTotal() Money {
	var sum Money
	for _, i := range r.Items {
		sum += i.Paid()
	}
	return sum
}

// Status reports whether the parsed items account for the printed subtotal.
func (r Receipt) Status() Status {
	if !r.found || len(r.Items) == 0 {
		return Unverified
	}
	diff := r.Subtotal - (r.ItemTotal() + r.Fees + r.Adjustments)
	if diff == 0 {
		return Reconciled
	}
	// A tax-inclusive total can exceed the items by at most GST and PST.
	if r.TaxIncluded && diff > 0 && float64(diff) <= 0.13*float64(r.Subtotal)+1 {
		return Reconciled
	}
	return Mismatch
}

// parsed is what a store-specific parser extracts from receipt lines.
type parsed struct {
	items       []LineItem
	fees        Money
	adjustments Money
	subtotal    Money
	taxIncluded bool
	found       bool
	// unpriced indexes items whose price could not be read.
	unpriced []int
}

// remove deletes the most recent item with the given code, as when a line
// is voided.
func (p *parsed) remove(code ProductCode) {
	for i := len(p.items) - 1; i >= 0; i-- {
		if p.items[i].Code == code {
			p.items = append(p.items[:i], p.items[i+1:]...)
			for j, u := range p.unpriced {
				if u > i {
					p.unpriced[j]--
				}
			}
			return
		}
	}
}

func (p *parsed) setSubtotal(m Money, taxIncluded bool) {
	p.subtotal, p.taxIncluded, p.found = m, taxIncluded, true
}

func (p *parsed) last() *LineItem {
	if len(p.items) == 0 {
		return nil
	}
	return &p.items[len(p.items)-1]
}

type lineParser func(lines []string) parsed

var parsers = map[StoreID]lineParser{
	"costco":        parseCostco,
	"superstore":    parseLoblaw,
	"wholesaleclub": parseLoblaw,
	"walmart":       parseWalmart,
	"coop":          parseGeneric(genericDialect{}),
	"sobeys":        parseGeneric(genericDialect{}),
	"safeway":       parseGeneric(genericDialect{}),
	"iga":           parseGeneric(genericDialect{}),
	"freshco":       parseGeneric(genericDialect{}),
	"saveonfoods":   parseGeneric(genericDialect{}),
	"tnt":           parseGeneric(genericDialect{}),
	"bulkbarn":      parseGeneric(genericDialect{startAfter: "TRANSACTION:"}),
}

// ParseReceipt extracts the items bought from a receipt's recognised text.
// Multi-page scans sometimes repeat part of a receipt, so when the whole
// document does not reconcile each page is also tried on its own.
func ParseReceipt(file ReceiptFile, doc Document) Receipt {
	best := parseDocument(file, doc)
	if best.Status() == Reconciled || len(doc) < 2 {
		return best
	}
	for _, page := range doc {
		if r := parseDocument(file, Document{page}); r.Status() == Reconciled {
			return r
		}
	}
	return best
}

func parseDocument(file ReceiptFile, doc Document) Receipt {
	r := Receipt{File: file}
	parse, ok := parsers[file.Store.ID]
	if !ok {
		return r
	}
	var lines []string
	for _, l := range doc.Lines() {
		if s := normaliseLine(l.Text()); s != "" {
			lines = append(lines, s)
		}
	}
	p := parse(lines)
	p.repair()
	for i := range p.items {
		p.items[i].Description = strings.TrimSpace(p.items[i].Description)
	}
	r.Items, r.Fees, r.Adjustments = p.items, p.fees, p.adjustments
	r.Subtotal, r.TaxIncluded, r.found = p.subtotal, p.taxIncluded, p.found
	return r
}

// repair fixes the two most common OCR failures using the printed subtotal:
// a single item whose price was unreadable has it inferred, and an item
// priced differently from an identical item on the same receipt (".39"
// beside "8.39") takes its sibling's price when that makes the receipt
// balance.
func (p *parsed) repair() {
	if len(p.unpriced) > 0 {
		defer func() {
			// Drop any items still without a price.
			kept := p.items[:0]
			for _, item := range p.items {
				if item.Price > 0 || item.Estimated {
					kept = append(kept, item)
				}
			}
			p.items = kept
			p.unpriced = nil
		}()
	}
	if !p.found || p.taxIncluded {
		return
	}
	gap := func() Money {
		var sum Money
		for _, item := range p.items {
			sum += item.Paid()
		}
		return p.subtotal - sum - p.fees - p.adjustments
	}
	if len(p.unpriced) == 1 {
		if g := gap(); g > 0 {
			item := &p.items[p.unpriced[0]]
			item.Price, item.Estimated = g+item.Discount, true
		}
		return
	}
	if len(p.unpriced) > 0 || gap() == 0 {
		return
	}
	for i := range p.items {
		for j := range p.items {
			if i == j || p.items[i].Code == "" || p.items[i].Code != p.items[j].Code || p.items[i].Price == p.items[j].Price {
				continue
			}
			was := p.items[i].Price
			p.items[i].Price = p.items[j].Price
			if gap() == 0 {
				p.items[i].Estimated = true
				return
			}
			p.items[i].Price = was
		}
	}
}
