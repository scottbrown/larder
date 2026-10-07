package larder

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	costcoItem     = regexp.MustCompile(`^(?:E\s+)?(\d[\d.,\-]{2,8})(?:[\s,]+(.*))?$`)
	costcoDiscount = regexp.MustCompile(`^\d{3,7}[\s,.]*[TF]PD\s*/\s*(\S*)`)
	costcoMulti    = regexp.MustCompile(`^(\d+)\s*@\s*\$?\d+\.\d{2}$`)
	costcoGarbled  = regexp.MustCompile(`\s[\d.:,]{1,6}$`)
	costcoFee      = regexp.MustCompile(`(?i)FEE|DEPOSIT|\bECO\b|CORE|ENVIRO|LEVY`)
	costcoTotal    = regexp.MustCompile(`^[*\s]*TOTAL\b`)
)

// parseCostco reads Costco warehouse receipts. Items look like
// "39036 ROMAINE 8.99"; instant savings follow as "2014432 TPD/39036 3.00-";
// multiples are announced on a preceding "2 @ 1.50" line, and a "VOID" line
// cancels the line after it.
func parseCostco(lines []string) parsed {
	var p parsed
	pendingQty := 0
	voidNext := false
	var tax Money
	taxSeen := false
	for _, line := range lines {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "COSTCO.CA") {
			return parsed{}
		}
		if isSubtotal(line) {
			if _, amount, ok := splitPrice(line); ok {
				p.setSubtotal(amount, false)
			}
			break
		}
		// Receipts without a subtotal print only the tax and total.
		if strings.HasPrefix(upper, "TAX") {
			tax, taxSeen = 0, true
			if _, amount, ok := splitPrice(line); ok {
				tax = amount
			}
			continue
		}
		if costcoTotal.MatchString(upper) && taxSeen {
			if _, amount, ok := splitPrice(line); ok {
				p.setSubtotal(amount-tax, false)
			}
			break
		}
		if taxSeen {
			continue
		}
		if upper == "VOID" {
			voidNext = true
			continue
		}
		if m := costcoMulti.FindStringSubmatch(line); m != nil {
			pendingQty, _ = strconv.Atoi(m[1])
			continue
		}
		text, amount, ok := splitPrice(line)
		if !ok {
			if m := costcoItem.FindStringSubmatch(line); m != nil && len(p.items) > 0 && costcoGarbled.MatchString(line) {
				desc := costcoGarbled.ReplaceAllString(m[2], "")
				p.unpriced = append(p.unpriced, len(p.items))
				p.items = append(p.items, LineItem{Code: costcoCode(m[1]), Description: desc, Quantity: Units(1)})
			}
			continue
		}
		wasVoid := voidNext
		voidNext = false
		if m := costcoDiscount.FindStringSubmatch(text); m != nil {
			if wasVoid {
				continue
			}
			if item := costcoDiscountTarget(&p, m[1]); item != nil {
				item.Discount += amount.Abs()
			}
			pendingQty = 0
			continue
		}
		m := costcoItem.FindStringSubmatch(text)
		if m == nil {
			// Unnumbered lines such as deposits, eco fees and battery cores.
			if len(p.items) > 0 && costcoFee.MatchString(text) {
				p.fees += amount
			}
			continue
		}
		code := costcoCode(m[1])
		if amount < 0 {
			if wasVoid {
				p.remove(code)
			} else if item := p.last(); item != nil {
				item.Discount += amount.Abs()
			}
			continue
		}
		qty := 1
		if pendingQty > 0 {
			qty, pendingQty = pendingQty, 0
		}
		p.items = append(p.items, LineItem{
			Code:        code,
			Description: strings.TrimSpace(strings.Trim(m[2], "|]")),
			Quantity:    Units(float64(qty)),
			Price:       amount,
		})
	}
	return p
}

func costcoCode(s string) ProductCode {
	return ProductCode(digitsOnly(s))
}

// costcoDiscountTarget finds the item an instant saving refers to: the most
// recent item with the referenced number, or else the previous item.
func costcoDiscountTarget(p *parsed, ref string) *LineItem {
	for i := len(p.items) - 1; i >= 0 && i >= len(p.items)-4; i-- {
		if string(p.items[i].Code) == ref && p.items[i].Discount == 0 {
			return &p.items[i]
		}
	}
	return p.last()
}
