package larder

import (
	"testing"
	"time"
)

func receiptOn(store StoreID, day int, items ...LineItem) Receipt {
	return Receipt{
		File:  ReceiptFile{Date: time.Date(2025, 1, day, 0, 0, 0, 0, time.UTC), Store: Store{ID: store}},
		Items: items,
	}
}

func TestPurchasesMergeMisreadCodes(t *testing.T) {
	romaine := LineItem{Code: "39036", Description: "ROMAINE", Quantity: Units(1), Price: 899}
	misread := LineItem{Code: "35036", Description: "ROMAlNE", Quantity: Units(1), Price: 899}
	other := LineItem{Code: "39037", Description: "KS PAPER TOWEL", Quantity: Units(1), Price: 2299}
	receipts := []Receipt{
		receiptOn("costco", 1, romaine), receiptOn("costco", 2, romaine), receiptOn("costco", 3, romaine),
		receiptOn("costco", 4, misread, other),
		receiptOn("coop", 5, LineItem{Description: "Bananas", Quantity: Units(1), Price: 150}),
		receiptOn("coop", 6, LineItem{Description: "Free", Quantity: Units(1)}),
	}
	ps := Purchases(receipts)
	if len(ps) != 6 {
		t.Fatalf("got %d purchases", len(ps))
	}
	if ps[3].Key != "costco:39036" {
		t.Errorf("misread code kept: %s", ps[3].Key)
	}
	if ps[4].Key != "costco:39037" {
		t.Errorf("distinct product merged: %s", ps[4].Key)
	}
	if ps[5].Key != "coop:BANANAS" || ps[5].Key.Store() != "coop" {
		t.Errorf("description key = %s", ps[5].Key)
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0}, {"abc", "abc", 0}, {"abc", "abd", 1}, {"abc", "ab", 1}, {"kitten", "sitting", 3},
	}
	for _, tt := range tests {
		if got := editDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("editDistance(%q, %q) = %d", tt.a, tt.b, got)
		}
	}
	if similarDescriptions("", "") != true || similarDescriptions("MILK", "PAPER TOWEL") {
		t.Error("similarDescriptions misjudged")
	}
}
