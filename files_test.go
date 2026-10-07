package larder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseReceiptName(t *testing.T) {
	date, store, ok := ParseReceiptName("20260102-Costco-Receipt-1.pdf")
	if !ok || store.ID != "costco" || !date.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v %v %v", date, store, ok)
	}
	for _, name := range []string{
		"20260102-CostcoGas-Receipt.pdf",
		"20260102-Costco-Statement.pdf",
		"notes.txt",
		"20261399-Costco-Receipt.pdf",
	} {
		if _, _, ok := ParseReceiptName(name); ok {
			t.Errorf("%s accepted", name)
		}
	}
	if _, store, ok := ParseReceiptName("20210818-Coop-receipt.pdf"); !ok || store.ID != "coop" {
		t.Errorf("lower-case receipt rejected")
	}
}

func TestFindReceipts(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"2019/20191231-Walmart-Receipt.pdf",
		"2018/20181231-Walmart-Receipt.pdf",
		"2020/20200105-Superstore-Receipt.pdf",
		"2020/20200101-Costco-Receipt.pdf",
		"2020/20200101-Amazon-Receipt.pdf",
	} {
		path := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := FindReceipts(root, YearRange(2019, 2020))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, filepath.Base(f.Path))
	}
	want := []string{"20191231-Walmart-Receipt.pdf", "20200101-Costco-Receipt.pdf", "20200105-Superstore-Receipt.pdf"}
	if len(names) != len(want) {
		t.Fatalf("got %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("got %v, want %v", names, want)
		}
	}
	if _, err := FindReceipts(filepath.Join(root, "missing"), YearRange(2019, 2020)); err == nil {
		t.Error("missing directory did not fail")
	}
}

func TestStoreLookups(t *testing.T) {
	if s, ok := StoreByID("tnt"); !ok || s.Name != "T&T Supermarket" {
		t.Errorf("StoreByID = %v %v", s, ok)
	}
	if _, ok := StoreByID("nowhere"); ok {
		t.Error("unknown store found")
	}
}
