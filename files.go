package larder

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var receiptName = regexp.MustCompile(`^(\d{8})-([^-]+)-[Rr]eceipt(?:-\d+)?\.pdf$`)

// ReceiptFile is a scanned grocery receipt found on disk.
type ReceiptFile struct {
	Path  string
	Date  time.Time
	Store Store
}

// DateRange is an inclusive range of calendar days.
type DateRange struct {
	From time.Time
	To   time.Time
}

// Contains reports whether t falls within the range.
func (r DateRange) Contains(t time.Time) bool {
	return !t.Before(r.From) && !t.After(r.To)
}

// YearRange covers every day from January 1 of from to December 31 of to.
func YearRange(from, to int) DateRange {
	return DateRange{
		From: time.Date(from, time.January, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(to, time.December, 31, 0, 0, 0, 0, time.UTC),
	}
}

// ParseReceiptName extracts the date and store from a filename such as
// "20260102-Costco-Receipt-1.pdf". It reports false for files that are not
// grocery receipts.
func ParseReceiptName(name string) (time.Time, Store, bool) {
	m := receiptName.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, Store{}, false
	}
	date, err := time.Parse("20060102", m[1])
	if err != nil {
		return time.Time{}, Store{}, false
	}
	store, ok := StoreByPrefix(m[2])
	if !ok {
		return time.Time{}, Store{}, false
	}
	return date, store, true
}

// FindReceipts walks root and returns the grocery receipts dated within r,
// ordered by date and then path.
func FindReceipts(root string, r DateRange) ([]ReceiptFile, error) {
	var out []ReceiptFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		date, store, ok := ParseReceiptName(d.Name())
		if ok && r.Contains(date) {
			out = append(out, ReceiptFile{Path: path, Date: date, Store: store})
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}
		return out[i].Path < out[j].Path
	})
	return out, err
}
