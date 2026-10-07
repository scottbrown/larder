package larder

import (
	"regexp"
	"strings"
)

// trailingPrice matches an amount at the end of a receipt line along with any
// tax or status flags printed after it, e.g. "14.97 G", "$3.22 D", "5.00-G",
// "$0.43GD". A leading "$" lets OCR drop the decimal point ("$1 77").
var trailingPrice = regexp.MustCompile(`(?:^|\s)(-)?(\$\s*\d{1,4}\s*[.,]?\s*\.?\s*\d{2}|\d{0,4}\s*[.,]\s*\d{2})\s*(-)?\s*(?:[A-Z*]{0,4}|[a-z]{1,2})\s*$`)

// ocrAmount matches a final token that looks like an amount in which OCR
// has confused digits with letters, e.g. "2.0C-" or "1O.99".
var ocrAmount = regexp.MustCompile(`(^|\s)(\$?)([\dOoCDIlSB]{1,4}[.,/][\dOoCDIlSB]{2})(-?\s?[A-Z]{0,3})$`)

var ocrDigits = strings.NewReplacer("O", "0", "o", "0", "C", "0", "D", "0", "I", "1", "l", "1", "S", "5", "B", "8")

var accents = strings.NewReplacer(
	"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
	"È", "E", "É", "E", "Ê", "E", "Ë", "E", "Ì", "I", "Í", "I", "Î", "I", "Ï", "I",
	"Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O", "Ù", "U", "Ú", "U", "Û", "U", "Ü", "U",
	"Ç", "C", "Ñ", "N", "à", "a", "á", "a", "â", "a", "ä", "a", "è", "e", "é", "e", "ê", "e",
	"ë", "e", "ì", "i", "í", "i", "î", "i", "ï", "i", "ò", "o", "ó", "o", "ô", "o", "ö", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n",
)

// ocrDollar fixes amounts where OCR read "$" as "S", e.g. "S2.44".
var ocrDollar = regexp.MustCompile(`(^|\s)S(\d+\.\d{2})`)

// normaliseLine tidies OCR artefacts that would otherwise hide prices.
func normaliseLine(s string) string {
	s = strings.NewReplacer("₴", "@", "|", " ").Replace(accents.Replace(s))
	s = ocrDollar.ReplaceAllString(s, "$1$$$2")
	s = strings.ReplaceAll(s, "$-", "-$")
	s = strings.Join(strings.Fields(s), " ")
	if m := ocrAmount.FindStringSubmatchIndex(s); m != nil && strings.ContainsAny(s[m[6]:m[7]], "0123456789") {
		amount := strings.Replace(ocrDigits.Replace(s[m[6]:m[7]]), "/", ".", 1)
		s = s[:m[6]] + amount + s[m[7]:]
	}
	return s
}

// splitPrice separates a receipt line into the text before a trailing amount
// and the amount itself. It reports false when the line has no price.
func splitPrice(line string) (string, Money, bool) {
	loc := trailingPrice.FindStringSubmatchIndex(line)
	if loc == nil {
		return line, 0, false
	}
	digits := digitsOnly(line[loc[4]:loc[5]])
	if len(digits) < 3 {
		digits = strings.Repeat("0", 3-len(digits)) + digits
	}
	amount, err := ParseMoney(digits[:len(digits)-2] + "." + digits[len(digits)-2:])
	if err != nil {
		return line, 0, false
	}
	if loc[2] >= 0 || loc[6] >= 0 {
		amount = -amount
	}
	return strings.TrimSpace(line[:loc[0]]), amount, true
}

// normaliseDescription produces a stable key for an item description,
// upper-cased with punctuation and repeated spaces removed.
func normaliseDescription(s string) string {
	s = strings.ToUpper(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ', r == '%', r == '&':
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// isSubtotal reports whether a line is a receipt's subtotal, tolerating OCR
// that splits or mangles the word, e.g. "SUBTO AL" or "SUB TOTAL".
func isSubtotal(line string) bool {
	compact := strings.ReplaceAll(strings.ToUpper(line), " ", "")
	return strings.HasPrefix(compact, "SUBT")
}

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
