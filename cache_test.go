package larder

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type countingRecognizer struct {
	calls int
	doc   Document
	err   error
}

func (c *countingRecognizer) Recognize(string) (Document, error) {
	c.calls++
	return c.doc, c.err
}

func TestCachedRecognizerRecognisesOnce(t *testing.T) {
	dir := t.TempDir()
	receipt := filepath.Join(dir, "a.pdf")
	if err := os.WriteFile(receipt, []byte("receipt"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner := &countingRecognizer{doc: docFromRows("39036 ROMAINE 8.99")}
	c := CachedRecognizer{Inner: inner, Dir: filepath.Join(dir, "cache"), Variant: "test"}

	for range 2 {
		doc, err := c.Recognize(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Lines()[0].Text() != "39036 ROMAINE 8.99" {
			t.Errorf("unexpected document %+v", doc)
		}
	}
	if inner.calls != 1 {
		t.Errorf("recognised %d times, want 1", inner.calls)
	}
}

func TestCachedRecognizerErrors(t *testing.T) {
	dir := t.TempDir()
	c := CachedRecognizer{Inner: &countingRecognizer{err: errors.New("boom")}, Dir: dir}
	if _, err := c.Recognize(filepath.Join(dir, "missing.pdf")); err == nil {
		t.Error("missing file did not fail")
	}
	receipt := filepath.Join(dir, "a.pdf")
	_ = os.WriteFile(receipt, []byte("x"), 0o644)
	if _, err := c.Recognize(receipt); err == nil {
		t.Error("recogniser error not returned")
	}
}

func TestLoadReceiptsSkipsDuplicatesAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) ReceiptFile {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		date, store, _ := ParseReceiptName(name)
		return ReceiptFile{Path: path, Date: date, Store: store}
	}
	files := []ReceiptFile{
		write("20260101-Costco-Receipt.pdf", "same"),
		write("20260101-Costco-Receipt-1.pdf", "same"),
	}
	bad := &countingRecognizer{doc: docFromRows("30669 BANANAS 1.99", "SUBTOTAL 5.00")}
	good := &countingRecognizer{doc: docFromRows("30669 BANANAS 1.99", "SUBTOTAL 1.99")}
	receipts, err := LoadReceipts(files, bad, good)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 {
		t.Fatalf("got %d receipts, want 1", len(receipts))
	}
	if receipts[0].Status() != Reconciled {
		t.Errorf("fallback reading not used: %s", receipts[0].Status())
	}

	failing := &countingRecognizer{err: errors.New("boom")}
	if _, err := LoadReceipts(files, failing); err == nil {
		t.Error("recognition error not returned")
	}
}
