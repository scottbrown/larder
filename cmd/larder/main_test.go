package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottbrown/larder/internal/pdftest"
)

// fixture writes a year of small Costco receipts and returns the flags that
// point larder at them with a private cache and catalogue.
func fixture(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	receipts := filepath.Join(root, "receipts", "2025")
	if err := os.MkdirAll(receipts, 0o755); err != nil {
		t.Fatal(err)
	}
	prices := []string{"1.99", "1.99", "2.19", "2.19", "2.29"}
	for i, price := range prices {
		name := fmt.Sprintf("2025%02d15-Costco-Receipt.pdf", 2*i+1)
		err := pdftest.WriteReceipt(filepath.Join(receipts, name),
			pdftest.Row{Text: "COSTCO WHOLESALE"},
			pdftest.Row{Text: "30669 BANANAS", Amount: price},
			pdftest.Row{Text: "457 HOMO MILK", Amount: "6.29"},
			pdftest.Row{Text: "SUBTOTAL", Amount: subtotal(price, "6.29")},
			pdftest.Row{Text: fmt.Sprintf("2025/%02d/15 11:44", 2*i+1)},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	catalogue := filepath.Join(root, "catalogue.yml")
	if err := os.WriteFile(catalogue, []byte("products:\n  - name: Bananas\n    category: Produce\n    match: [costco:30669]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{
		"--receipts", filepath.Join(root, "receipts"),
		"--cache", filepath.Join(root, "cache"),
		"--catalogue", catalogue,
		"--from", "2025", "--to", "2025",
	}
}

func subtotal(a, b string) string {
	var x, y float64
	fmt.Sscanf(a, "%f", &x)
	fmt.Sscanf(b, "%f", &y)
	return fmt.Sprintf("%.2f", x+y)
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("larder %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String()
}

func TestCommandsEndToEnd(t *testing.T) {
	flags := fixture(t)

	if out := run(t, append([]string{"scan"}, flags...)...); !strings.Contains(out, "5 receipts scanned, 0 failed") {
		t.Errorf("scan output:\n%s", out)
	}
	if out := run(t, append([]string{"audit", "-v"}, flags...)...); !strings.Contains(out, "5 (100%)") {
		t.Errorf("audit output:\n%s", out)
	}
	items := run(t, append([]string{"items"}, flags...)...)
	if strings.Count(items, "\n") != 11 || !strings.Contains(items, "costco,30669,BANANAS") {
		t.Errorf("items output:\n%s", items)
	}
	regulars := run(t, append([]string{"regulars"}, flags...)...)
	for _, want := range []string{"Bananas", "Produce", "HOMO MILK", "61d"} {
		if !strings.Contains(regulars, want) {
			t.Errorf("regulars missing %q:\n%s", want, regulars)
		}
	}
	stubs := run(t, append([]string{"catalogue"}, flags...)...)
	if !strings.Contains(stubs, "costco:457") || strings.Contains(stubs, "costco:30669") {
		t.Errorf("catalogue stubs:\n%s", stubs)
	}

	html := filepath.Join(t.TempDir(), "larder.html")
	run(t, append([]string{"dashboard", "-o", html}, flags...)...)
	page, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<title>Larder</title>") || !strings.Contains(string(page), "HOMO MILK") {
		t.Error("dashboard missing content")
	}

	receipt := filepath.Join(strings.TrimSuffix(flags[1], "receipts")+"receipts", "2025", "20250115-Costco-Receipt.pdf")
	if out := run(t, append([]string{"ocr", receipt}, flags...)...); !strings.Contains(out, "30669 BANANAS | 1.99") {
		t.Errorf("ocr output:\n%s", out)
	}
}

func TestCommandsNeedReceipts(t *testing.T) {
	t.Setenv("LARDER_RECEIPTS", "")
	cmd := newRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"audit"})
	if err := cmd.Execute(); err == nil {
		t.Error("audit without receipts succeeded")
	}
}

func TestDefaultPaths(t *testing.T) {
	t.Setenv("LARDER_CATALOGUE", "/tmp/x.yml")
	if got := defaultCataloguePath(); got != "/tmp/x.yml" {
		t.Errorf("catalogue path = %s", got)
	}
	t.Setenv("LARDER_CATALOGUE", "")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	if got := defaultCataloguePath(); got != "/cfg/larder/catalogue.yml" {
		t.Errorf("catalogue path = %s", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := defaultCataloguePath(); !strings.HasSuffix(got, ".config/larder/catalogue.yml") {
		t.Errorf("catalogue path = %s", got)
	}
	if defaultCacheDir() == "" {
		t.Error("no cache directory")
	}
}
