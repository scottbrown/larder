//go:build darwin

package vision

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottbrown/larder/internal/pdftest"
)

func TestRecognizeReadsReceiptText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.pdf")
	if err := pdftest.WriteReceipt(path,
		pdftest.Row{Text: "COSTCO WHOLESALE"},
		pdftest.Row{Text: "30669 BANANAS", Amount: "1.99"},
		pdftest.Row{Text: "SUBTOTAL", Amount: "1.99"},
	); err != nil {
		t.Fatal(err)
	}
	for _, dpi := range []int{0, 300} {
		doc, err := Recognizer{DPI: dpi}.Recognize(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(doc) != 1 || doc[0].Width == 0 || doc[0].Height == 0 {
			t.Fatalf("unexpected pages: %+v", doc)
		}
		var texts []string
		for _, l := range doc.Lines() {
			texts = append(texts, l.Text())
		}
		got := strings.Join(texts, "\n")
		for _, want := range []string{"30669 BANANAS 1.99", "SUBTOTAL 1.99"} {
			if !strings.Contains(got, want) {
				t.Errorf("dpi %d: missing %q in\n%s", dpi, want, got)
			}
		}
	}
}

func TestRecognizeRejectsMissingFile(t *testing.T) {
	if _, err := (Recognizer{}).Recognize(filepath.Join(t.TempDir(), "none.pdf")); err == nil {
		t.Error("missing file did not fail")
	}
}
