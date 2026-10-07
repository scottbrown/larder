package larder

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	walmartItem    = regexp.MustCompile(`^(.+?)\s*:?\s+(\d[\d:']{10,13}\d)[A-Za-z]?(?:\s+(.*))?$`)
	walmartWeight  = regexp.MustCompile(`^(\d*\.\d+)\s*kg\s*[@a-z9]?\s*\$?(\d+\.\d{2})\s*/\s*kg`)
	walmartMulti   = regexp.MustCompile(`^(\d+)\s*(?:AT|@)\s*\$?\d+\.\d{2}`)
	walmartEnd     = regexp.MustCompile(`(?i)\bTEND\b|CHANGE DUE`)
	walmartFee     = regexp.MustCompile(`(?i)\bBAG\b|DEPOSIT|ENVIRO|ECO FEE|RECYCL|DONATION|\bCRF\b|BEV DEP|^RX$`)
	walmartFull    = regexp.MustCompile(`^\d+\.\d{2}$`)
	walmartCents   = regexp.MustCompile(`\.\d{2}$`)
	walmartMinus   = regexp.MustCompile(`-[A-Z]{0,2}$`)
	walmartTag     = regexp.MustCompile(`^\S{0,3}LTI\s+(\d{1,3})$`)
	walmartGroup   = regexp.MustCompile(`\b0*(\d{1,3})L\b`)
	walmartOCRChar = strings.NewReplacer("T", "7", "\"", "7", "i", "1", "l", "1", "I", "1", "O", "0", "o", "0", "S", "5", "B", "8", "Z", "2")
)

// parseWalmart reads Walmart receipts, whose items look like
// "WELCH JAM 006591200128 $2.97 D" with weighed produce priced on the next
// line, e.g. "0.590 kg @ $5.45/kg $3.22 D", and multiples as
// "2 AT $0.05 $0.10".
func parseWalmart(lines []string) parsed {
	var p parsed
	var pending *LineItem
	pendingFee := false
	// Items in a multi-buy are tagged "MULTI 18"; the discount for the deal
	// later names the same group as "018L".
	groups := map[string][]int{}
	for _, line := range lines {
		upper := strings.ToUpper(line)
		if walmartEnd.MatchString(line) {
			break
		}
		if strings.HasPrefix(upper, "SUBTOTAL") {
			if _, amount, ok := splitPrice(line); ok {
				p.setSubtotal(amount, false)
			}
			continue
		}
		if m := walmartTag.FindStringSubmatch(upper); m != nil {
			if len(p.items) > 0 {
				groups[m[1]] = append(groups[m[1]], len(p.items)-1)
			}
			continue
		}
		if m := walmartWeight.FindStringSubmatch(line); m != nil {
			if _, total, ok := splitPrice(line[len(m[0]):]); ok && pending != nil {
				kg, _ := strconv.ParseFloat(m[1], 64)
				pending.Quantity = Kilograms(kg)
				pending.Price = total
				p.items = append(p.items, *pending)
			}
			pending, pendingFee = nil, false
			continue
		}
		if m := walmartMulti.FindStringSubmatch(upper); m != nil {
			if _, total, ok := splitPrice(line); ok {
				qty, _ := strconv.Atoi(m[1])
				switch {
				case pendingFee:
					p.fees += total
				case pending != nil:
					pending.Quantity = Units(float64(qty))
					pending.Price = total
					p.items = append(p.items, *pending)
				}
			}
			pending, pendingFee = nil, false
			continue
		}
		if m := walmartItem.FindStringSubmatch(line); m != nil {
			pending, pendingFee = nil, false
			code := digitsOnly(m[2])
			if len(code) < 12 {
				continue
			}
			desc := strings.TrimSpace(m[1])
			amount, hasPrice := walmartAmount(m[3])
			item := LineItem{
				Code:        ProductCode(strings.TrimLeft(code, "0")),
				Description: desc,
				Quantity:    Units(1),
				Price:       amount,
			}
			switch {
			case walmartFee.MatchString(desc):
				if hasPrice {
					p.fees += amount
				} else {
					pendingFee = true
				}
			case hasPrice && amount < 0:
				p.adjustments += amount
			case hasPrice:
				p.items = append(p.items, item)
			case strings.Contains(m[3], "$"):
				p.unpriced = append(p.unpriced, len(p.items))
				p.items = append(p.items, item)
			default:
				pending = &item
			}
			continue
		}
		if text, amount, ok := splitPrice(line); ok && amount < 0 {
			if m := walmartGroup.FindStringSubmatch(text); m != nil && len(groups[m[1]]) > 0 {
				spreadDiscount(&p, groups[m[1]], amount.Abs())
			} else {
				p.adjustments += amount
			}
		}
	}
	return p
}

// spreadDiscount shares a deal's discount among the items it covers in
// proportion to their prices.
func spreadDiscount(p *parsed, items []int, discount Money) {
	var total Money
	for _, i := range items {
		total += p.items[i].Price
	}
	remaining := discount
	for n, i := range items {
		share := discount / Money(len(items))
		if total > 0 {
			share = discount * p.items[i].Price / total
		}
		if n == len(items)-1 {
			share = remaining
		}
		remaining -= share
		p.items[i].Discount += share
	}
}

// walmartAmount reads the "$1.47 D" that follows an item's code, tolerating
// OCR that splits the amount ("$1 47"), drops the decimal point, or reads
// digits as letters ("$1.4T").
func walmartAmount(s string) (Money, bool) {
	i := strings.Index(s, "$")
	if i < 0 {
		return 0, false
	}
	negative := strings.Contains(s[i:], "-")
	acc := ""
	for _, tok := range strings.Fields(s[i+1:]) {
		tok = walmartOCRChar.Replace(walmartMinus.ReplaceAllString(tok, ""))
		if walmartFull.MatchString(tok) && acc != "" {
			acc = tok
			break
		}
		if strings.Trim(tok, "0123456789.") != "" {
			break
		}
		acc += tok
		if walmartCents.MatchString(acc) {
			break
		}
	}
	digits := digitsOnly(acc)
	var amount Money
	switch {
	case strings.Contains(acc, "."):
		parts := strings.SplitN(acc, ".", 2)
		cents := digitsOnly(parts[1])
		if len(cents) != 2 {
			return 0, false
		}
		amount, _ = ParseMoney(digitsOnly(parts[0]) + "." + cents)
	case len(digits) >= 3:
		amount, _ = ParseMoney(digits[:len(digits)-2] + "." + digits[len(digits)-2:])
	default:
		return 0, false
	}
	if negative {
		amount = -amount
	}
	return amount, true
}
