package larder

// StoreID identifies a grocery retailer.
type StoreID string

// Store describes a retailer and the filename prefixes its receipts use.
type Store struct {
	ID       StoreID
	Name     string
	Prefixes []string
}

// Stores lists every retailer whose receipts are analysed.
var Stores = []Store{
	{ID: "costco", Name: "Costco", Prefixes: []string{"Costco"}},
	{ID: "superstore", Name: "Real Canadian Superstore", Prefixes: []string{"Superstore", "SuperStore", "RealCanadianSuperstore"}},
	{ID: "wholesaleclub", Name: "Wholesale Club", Prefixes: []string{"WholesaleClub"}},
	{ID: "walmart", Name: "Walmart", Prefixes: []string{"Walmart"}},
	{ID: "coop", Name: "Calgary Co-op", Prefixes: []string{"Coop", "CoOp", "CalgaryCoop"}},
	{ID: "sobeys", Name: "Sobeys", Prefixes: []string{"Sobeys"}},
	{ID: "safeway", Name: "Safeway", Prefixes: []string{"Safeway"}},
	{ID: "tnt", Name: "T&T Supermarket", Prefixes: []string{"T&TSupermarket", "T&T"}},
	{ID: "saveonfoods", Name: "Save-On-Foods", Prefixes: []string{"SaveOnFoods"}},
	{ID: "bulkbarn", Name: "Bulk Barn", Prefixes: []string{"BulkBarn"}},
	{ID: "freshco", Name: "FreshCo", Prefixes: []string{"Freshco", "FreshCo"}},
	{ID: "iga", Name: "IGA", Prefixes: []string{"IGA"}},
}

// StoreByPrefix returns the store whose receipts are filed under prefix.
func StoreByPrefix(prefix string) (Store, bool) {
	for _, s := range Stores {
		for _, p := range s.Prefixes {
			if p == prefix {
				return s, true
			}
		}
	}
	return Store{}, false
}

// StoreByID returns the store with the given identifier.
func StoreByID(id StoreID) (Store, bool) {
	for _, s := range Stores {
		if s.ID == id {
			return s, true
		}
	}
	return Store{}, false
}
