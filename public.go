package larder

import (
	"regexp"
	"time"
)

// sensitiveName catches health, personal-care, alcohol and child-related
// products that are not catalogued yet, so a public dashboard does not
// depend on the catalogue being complete.
var sensitiveName = regexp.MustCompile(`(?i)PROBIO|VITA|VITES|\bVIT\b|ADVIL|TYLENOL|MOTRIN|REACTINE|CLARITIN|ALLER|HALLS|BENYLIN|BUCKLEY|COLD|COUGH|\bRX\b|PHARM|PRESCR|MEDIC|RENU|CONTACT|\bLENS|GLAXAL|CREAM BASE|PRONAMEL|SENSODYNE|TOOTH|DENT|FLOSS|GLIDE|MOUTHW|LISTERINE|DOVE|SHAMPOO|CONDIT|SOAP|DEODOR|ANTIPERS|RAZOR|TAMPON|\bPADS?\b|LINER|DEPEND|CONDOM|PREGN|DIAPER|PULL.?UPS|BABY|WIPES?\b|NICORETTE|MELATONIN|IBUPROF|ACETAMIN|LAXAT|FIBRE ?SUPP|PSYLLI|ADULT|CHILDREN|KIDS?\b|SYRAH|MERLOT|CABERNET|PINOT|CHARDON|SAUVIGNON|\bWINE|BEER|PORTER|LAGER|\bIPA\b|\bALE\b|CIDER|VODKA|\bRUM\b|WHISK|\bGIN\b|TEQUILA|LIQUOR|SPIRITS|SELTZER`)

// PublicStats keeps only products that are safe to share: nothing in a
// private category, nothing whose name suggests health, personal care,
// alcohol or children, and no store item codes.
func PublicStats(stats []ProductStats, cat *Catalogue) []ProductStats {
	var out []ProductStats
	for _, s := range stats {
		if cat.PrivateCategory(s.Category) || sensitiveName.MatchString(s.Name) {
			continue
		}
		s.Keys = nil
		out = append(out, s)
	}
	return out
}

// weekOf returns the Monday that starts t's week, so shared dates show when
// something was bought without revealing the household's daily routine.
func weekOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, t.Location())
}
