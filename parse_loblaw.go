package larder

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	loblawDepartment = regexp.MustCompile(`^\d{2}-([A-Z][A-Z &/-]+)$`)
	loblawItem       = regexp.MustCompile(`^(?:\((\d+)\)\s*)?(\*?)(\d[\d.\-]{3,14}(?:\s[\d.\-]{2,8})?|\d{1,3}(?:\s[A-Z]+\s)?BAGS?)\s+(.+)$`)
	loblawFlagsOnly  = regexp.MustCompile(`^[GHPMRJQKB]{1,6}$`)
	loblawFlags      = regexp.MustCompile(`\s+[GHPMRJQKB]{1,5}$`)
	loblawWeight     = regexp.MustCompile(`^(\d*\.\d+)\s*kg\s*(?:Net\s*)?[@a-z9]\s*\$?(\d+\.\d{2})\s*/\s*kg`)
	loblawGross      = regexp.MustCompile(`(?i)\s*-?\d*\.\d+\s*kg\s*(?:Gross|[lT]are\b.*)`)
	loblawMulti      = regexp.MustCompile(`^(\d+)\s*[@a-z9]\s*`)
	loblawRegular    = regexp.MustCompile(`\$(\d+\.\d{2})\s*ea\b`)
	loblawLimit      = regexp.MustCompile(`(?i)^\$\d+\.\d{2}\s*[l1I]mt\b`)
	loblawFee        = regexp.MustCompile(`(?i)DEPOSIT|RECYCLING|CHARITY|\bBAGS?\b|ENVIRO`)
)

// parseLoblaw reads Real Canadian Superstore and Wholesale Club receipts.
// Items start with a UPC or PLU; when the price depends on quantity or
// weight it appears on following lines such as "2 @ $1.49 ea 2.98" or
// "0.815 kg @ $2.14/kg 1.74". Items prefixed "(n)" share one price line.
func parseLoblaw(lines []string) parsed {
	var p parsed
	department := ""
	var pending []int   // indexes of items still awaiting a price line
	var pendingN []int  // group counts for those items
	var regular Money   // regular unit price announced for the pending group
	pendingFee := false // a fee line whose amount follows on the next line

	flush := func() { pending, pendingN, regular, pendingFee = nil, nil, 0, false }

	for _, line := range lines {
		// Tare-weighed items print gross and tare weights before the net.
		if line = strings.TrimSpace(loblawGross.ReplaceAllString(line, "")); line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "SUBTOTAL") || strings.HasPrefix(upper, "TOTAL") {
			if _, amount, ok := splitPrice(line); ok {
				p.setSubtotal(amount, strings.HasPrefix(upper, "TOTAL"))
			}
			break
		}
		if m := loblawDepartment.FindStringSubmatch(line); m != nil {
			department = titleCase(m[1])
			flush()
			continue
		}
		if loblawLimit.MatchString(line) || strings.Contains(upper, " EA OR ") {
			if m := loblawRegular.FindStringSubmatch(line); m != nil {
				regular, _ = ParseMoney(m[1])
			}
			continue
		}
		if m := loblawWeight.FindStringSubmatch(line); m != nil && len(pending) > 0 {
			_, total, ok := splitPrice(line[len(m[0]):])
			kg, _ := strconv.ParseFloat(m[1], 64)
			if ok {
				item := &p.items[pending[len(pending)-1]]
				item.Quantity = Kilograms(kg)
				item.Price = total
			}
			flush()
			continue
		}
		text, amount, hasPrice := splitPrice(line)
		if !hasPrice && loblawFee.MatchString(text) {
			flush()
			pendingFee = true
			continue
		}

		if m := loblawItem.FindStringSubmatch(text); m != nil && !loblawWeight.MatchString(line) {
			if m[2] == "*" || loblawFee.MatchString(m[0]) {
				flush()
				if hasPrice {
					p.fees += amount
				} else {
					pendingFee = true
				}
				continue
			}
			n := 1
			if m[1] != "" {
				n, _ = strconv.Atoi(m[1])
			}
			desc := loblawFlags.ReplaceAllString(strings.TrimSpace(m[4]), "")
			item := LineItem{
				Code:        ProductCode(strings.TrimLeft(digitsOnly(m[3]), "0")),
				Description: desc,
				Department:  department,
				Quantity:    Units(float64(n)),
			}
			if hasPrice && m[1] == "" {
				item.Price = amount
				p.items = append(p.items, item)
				flush()
				continue
			}
			if len(pending) > 0 && m[1] == "" {
				flush()
			}
			p.items = append(p.items, item)
			pending = append(pending, len(p.items)-1)
			pendingN = append(pendingN, n)
			continue
		}

		if pendingFee && hasPrice {
			p.fees += amount
			flush()
			continue
		}
		if hasPrice && len(pending) == 1 && loblawFlagsOnly.MatchString(text) {
			p.items[pending[0]].Price = amount
			flush()
			continue
		}
		if m := loblawMulti.FindStringSubmatch(line); m != nil && hasPrice && len(pending) > 0 {
			qty, _ := strconv.Atoi(m[1])
			applyGroupPrice(&p, pending, pendingN, qty, amount, regular)
			flush()
			continue
		}
		if hasPrice && loblawFee.MatchString(text) {
			p.fees += amount
			continue
		}
		if hasPrice && amount < 0 && p.last() != nil {
			p.last().Discount += amount.Abs()
		}
	}
	return p
}

// applyGroupPrice spreads a price line across the pending items it covers,
// in proportion to how many of each were bought.
func applyGroupPrice(p *parsed, pending, counts []int, qty int, total, regular Money) {
	sum := 0
	for _, n := range counts {
		sum += n
	}
	if sum <= 1 {
		sum = qty
		counts = []int{qty}
	}
	remaining := total
	for i, idx := range pending {
		share := total * Money(counts[i]) / Money(sum)
		if i == len(pending)-1 {
			share = remaining
		}
		remaining -= share
		item := &p.items[idx]
		item.Quantity = Units(float64(counts[i]))
		item.Price = share
		if regular > 0 {
			shelf := regular * Money(counts[i])
			if shelf > share {
				item.Price, item.Discount = shelf, shelf-share
			}
		}
	}
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
