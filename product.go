package larder

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// ProductKey identifies one product at one store: "costco:30669" for stores
// that print item codes, or "coop:BANANAS" for stores that print only a
// description.
type ProductKey string

// NewProductKey builds the key for an item bought at a store.
func NewProductKey(store StoreID, item LineItem) ProductKey {
	if item.Code != "" {
		return ProductKey(string(store) + ":" + string(item.Code))
	}
	return ProductKey(string(store) + ":" + normaliseDescription(item.Description))
}

// Store returns the store part of the key.
func (k ProductKey) Store() StoreID {
	store, _, _ := strings.Cut(string(k), ":")
	return StoreID(store)
}

// Purchase is one line item together with when and where it was bought.
type Purchase struct {
	Date    time.Time
	Store   StoreID
	Key     ProductKey
	Item    LineItem
	Receipt string
	Status  Status
}

// nonProduct matches lines that parsers occasionally mistake for items:
// fees, deposits, bags, subtotals, deal descriptions, gift cards and
// tickets. None of them are groceries.
var nonProduct = regexp.MustCompile(`(?i)\bFEE\b|DEPOSIT|\bDEP\b|\bCRF\b|HDPE|[PF]LASTIC.?BAG|SUBT.?TAL|^\d+ FOR \$|GIFT ?CARD|GIFT/CRD|TICKETS?\b|CINEPLEX`)

// Purchases flattens receipts into individual purchases, merging product
// codes and descriptions that differ only by an OCR misreading.
func Purchases(receipts []Receipt) []Purchase {
	var out []Purchase
	for _, r := range receipts {
		status := r.Status()
		for _, item := range r.Items {
			if item.Price <= 0 || nonProduct.MatchString(item.Description) {
				continue
			}
			out = append(out, Purchase{
				Date:    r.File.Date,
				Store:   r.File.Store.ID,
				Key:     NewProductKey(r.File.Store.ID, item),
				Item:    item,
				Receipt: r.File.Path,
				Status:  status,
			})
		}
	}
	remap := misreadKeys(out)
	for i := range out {
		if k, ok := remap[out[i].Key]; ok {
			out[i].Key = k
		}
	}
	return out
}

// misreadKeys finds rarely seen product keys that are one character away
// from a much more common key at the same store with a similar description,
// and maps them onto it. OCR misreads a digit or letter far more often than
// a store sells two near-identical codes.
func misreadKeys(purchases []Purchase) map[ProductKey]ProductKey {
	count := map[ProductKey]int{}
	descs := map[ProductKey]map[string]int{}
	for _, p := range purchases {
		count[p.Key]++
		if descs[p.Key] == nil {
			descs[p.Key] = map[string]int{}
		}
		descs[p.Key][normaliseDescription(p.Item.Description)]++
	}
	byStore := map[StoreID][]ProductKey{}
	for k := range count {
		byStore[k.Store()] = append(byStore[k.Store()], k)
	}

	remap := map[ProductKey]ProductKey{}
	for _, keys := range byStore {
		sort.Slice(keys, func(i, j int) bool {
			if count[keys[i]] != count[keys[j]] {
				return count[keys[i]] > count[keys[j]]
			}
			return keys[i] < keys[j]
		})
		for _, rare := range keys {
			if count[rare] > 2 {
				continue
			}
			for _, common := range keys {
				if count[common] < 3 || count[common] < 3*count[rare] {
					break
				}
				if editDistance(string(rare), string(common)) == 1 &&
					similarDescriptions(topKey(descs[rare]), topKey(descs[common])) {
					remap[rare] = common
					break
				}
			}
		}
	}
	return remap
}

// similarDescriptions reports whether two normalised descriptions are close
// enough to name the same product despite OCR noise.
func similarDescriptions(a, b string) bool {
	if a == b {
		return true
	}
	longest := max(len(a), len(b))
	if longest == 0 {
		return false
	}
	return float64(editDistance(a, b)) <= 0.34*float64(longest)
}

func topKey(m map[string]int) string {
	best, n := "", -1
	for k, v := range m {
		if v > n || v == n && k < best {
			best, n = k, v
		}
	}
	return best
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
