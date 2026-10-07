package larder

import (
	"regexp"
	"strconv"
	"strings"
)

// genericDialect tunes the generic parser for one retailer's layout.
type genericDialect struct {
	// startAfter skips lines up to and including the first one containing
	// this upper-case text, for receipts that print the payment slip first.
	startAfter string
}

var (
	genericEnd       = regexp.MustCompile(`(?i)^(?:\d+\s+)?(SUB[\s-]?TOTAL|BALANCE DUE|TOTAL)\b`)
	genericFee       = regexp.MustCompile(`(?i)^\+|DEPOSIT|\bDEP\b|DEP/EA|\bEHC\b|ENVIRO|RECYCL|COMPOST|^BAGS?\b|BAG CHARGE|BOTTLE DEP|CHARITY|DONATION`)
	genericSlip      = regexp.MustCompile(`^(TYPE: Purchase|ACCT:|TRANSACTION RECORD|CARD NUMBER)`)
	genericSaved     = regexp.MustCompile(`(?i)YOU SAVED`)
	genericCardSave  = regexp.MustCompile(`(?i)\bSAVE\b|COUPON|DISCOUNT|PROMO`)
	genericWeight    = regexp.MustCompile(`^(\d*\.\d+)\s*(kg|lb)\s*[@a-z9(]*\s*\$?\s*(\d+\.\d{2})\s*/\s*(kg|lb)`)
	genericMulti     = regexp.MustCompile(`^(\d+)\s*[@g9]\s*(.*)$`)
	genericUnitPrice = regexp.MustCompile(`\$?(\d+\.\d{2})\s*(?:ea)?\.?`)
	genericCode      = regexp.MustCompile(`\s+(\d{8,13})$`)
	genericMarker    = regexp.MustCompile(`\s+(?:W|#|\*|[A-Z]{1,2})$`)
	genericSkip      = regexp.MustCompile(`(?i)POINTS|PTS\b|SERVED BY|MEMBER|GST|PST|TAX|^CARD\s+\d|SCENE|AIR MILES|PRESCRIPTION|PHARM|^SAVINGS\b`)
	genericSection   = map[string]bool{
		"GROCERY": true, "DELI": true, "PRODUCE": true, "DAIRY": true, "MEAT": true,
		"BAKERY": true, "FROZEN": true, "SEAFOOD": true, "FOOD": true, "FROZEN FOOD": true,
		"GENERAL MERCHANDISE": true, "HEALTH & BEAUTY": true, "HBC": true, "LIQUOR": true,
	}
)

// parseGeneric reads the description-and-price layouts used by Co-op,
// Sobeys, Safeway, IGA, FreshCo, Save-On-Foods, T&T and Bulk Barn.
func parseGeneric(d genericDialect) lineParser {
	return func(lines []string) parsed {
		var p parsed
		department := ""
		pending := ""
		started := d.startAfter == ""
		inSlip := false
		for _, line := range lines {
			upper := strings.ToUpper(line)
			if !started {
				started = strings.Contains(upper, d.startAfter)
				continue
			}
			// Some receipts print the card slip before the balance due.
			if genericSlip.MatchString(line) {
				inSlip = true
			}
			m := genericEnd.FindStringSubmatch(line)
			if inSlip && m == nil {
				continue
			}
			if m != nil {
				if _, amount, ok := splitPrice(line); ok {
					p.setSubtotal(amount, !strings.HasPrefix(strings.ToUpper(m[1]), "SUB"))
					break
				}
				continue
			}
			if genericSection[upper] {
				department = titleCase(upper)
				continue
			}
			if genericSkip.MatchString(line) {
				continue
			}
			if m := genericWeight.FindStringSubmatch(line); m != nil {
				applyGenericWeight(&p, &pending, department, line, m)
				continue
			}
			text, amount, hasPrice := splitPrice(line)
			if genericSaved.MatchString(text) {
				if item := p.last(); item != nil && amount > 0 {
					item.Price += amount
					item.Discount += amount
				}
				continue
			}
			if hasPrice && amount < 0 && genericCardSave.MatchString(text) {
				if item := p.last(); item != nil {
					item.Discount += amount.Abs()
				}
				continue
			}
			if m := genericMulti.FindStringSubmatch(line); m != nil {
				applyGenericMulti(&p, &pending, department, m)
				continue
			}
			if !hasPrice {
				pending = line
				continue
			}
			if genericFee.MatchString(text) {
				p.fees += amount
				pending = ""
				continue
			}
			if amount < 0 {
				p.adjustments += amount
				continue
			}
			p.items = append(p.items, genericItem(text, department, Units(1), amount))
			pending = ""
		}
		return p
	}
}

func genericItem(text, department string, qty Quantity, price Money) LineItem {
	text = strings.TrimSpace(genericMarker.ReplaceAllString(text, ""))
	promotion := false
	if rest, ok := strings.CutPrefix(text, "(SALE)"); ok {
		text, promotion = strings.TrimSpace(rest), true
	}
	item := LineItem{Description: text, Department: department, Quantity: qty, Price: price, Promotion: promotion}
	if m := genericCode.FindStringSubmatch(text); m != nil {
		item.Code = ProductCode(strings.TrimLeft(m[1], "0"))
		item.Description = strings.TrimSpace(text[:len(text)-len(m[0])])
	}
	return item
}

// applyGenericWeight handles "0.070 kg @ $6.15/kg", which either completes a
// pending description (when it carries the total) or describes the
// previous item.
func applyGenericWeight(p *parsed, pending *string, department, line string, m []string) {
	weight, _ := strconv.ParseFloat(m[1], 64)
	if m[2] == "lb" {
		weight *= 0.45359237
	}
	_, total, ok := splitPrice(line[len(m[0]):])
	if ok && *pending != "" {
		p.items = append(p.items, genericItem(*pending, department, Kilograms(weight), total))
	} else if item := p.last(); item != nil {
		item.Quantity = Kilograms(weight)
	}
	*pending = ""
}

// applyGenericMulti handles quantity lines. "2 @ $5.37ea. $10.74" carries
// the line total and completes a pending description; "2 @ 6.99" or
// "1 @ 2/ $5.00" only says how many of the previous item were bought.
func applyGenericMulti(p *parsed, pending *string, department string, m []string) {
	qty, _ := strconv.Atoi(m[1])
	rest := m[2]
	_, total, hasTotal := splitPrice(rest)
	carriesTotal := hasTotal && len(genericUnitPrice.FindAllString(rest, -1)) >= 2
	if *pending != "" && hasTotal {
		if !carriesTotal {
			if unit, ok := firstAmount(rest); ok {
				total = unit * Money(qty)
			}
		}
		p.items = append(p.items, genericItem(*pending, department, Units(float64(qty)), total))
	} else if item := p.last(); item != nil && qty > 0 {
		item.Quantity = Units(float64(qty))
	}
	*pending = ""
}

func firstAmount(s string) (Money, bool) {
	m := genericUnitPrice.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	amount, err := ParseMoney(m[1])
	return amount, err == nil
}
