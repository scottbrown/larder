package larder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Money is an amount of Canadian currency in cents.
type Money int64

var moneyPattern = regexp.MustCompile(`^(-)?\$?(\d*)(?:\.(\d{1,2}))?(-)?$`)

// ParseMoney parses amounts such as "12.99", "$4", ".99", "-1.50" or the
// trailing-minus form "3.00-" that receipts use for discounts.
func ParseMoney(s string) (Money, error) {
	clean := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	m := moneyPattern.FindStringSubmatch(clean)
	if m == nil || (m[2] == "" && m[3] == "") {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	var dollars, cents int64
	if m[2] != "" {
		dollars, _ = strconv.ParseInt(m[2], 10, 64)
	}
	if m[3] != "" {
		frac := m[3]
		if len(frac) == 1 {
			frac += "0"
		}
		cents, _ = strconv.ParseInt(frac, 10, 64)
	}
	amount := Money(dollars*100 + cents)
	if m[1] != "" || m[4] != "" {
		amount = -amount
	}
	return amount, nil
}

// Dollars returns the amount as a floating-point number of dollars.
func (m Money) Dollars() float64 {
	return float64(m) / 100
}

// String formats the amount as dollars, e.g. "$12.99" or "-$3.00".
func (m Money) String() string {
	sign := ""
	if m < 0 {
		sign = "-"
		m = -m
	}
	return fmt.Sprintf("%s$%d.%02d", sign, m/100, m%100)
}

// Abs returns the absolute value of the amount.
func (m Money) Abs() Money {
	if m < 0 {
		return -m
	}
	return m
}
