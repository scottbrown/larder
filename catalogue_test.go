package larder

import (
	"path/filepath"
	"strings"
	"testing"
)

const sampleCatalogue = `
products:
  - name: Bananas
    category: Produce
    match:
      - costco:30669
      - superstore:0004011
      - coop:Bananas
ignore:
  - costco:23983
`

func TestParseCatalogue(t *testing.T) {
	c, err := ParseCatalogue([]byte(sampleCatalogue))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []ProductKey{"costco:30669", "superstore:4011", "coop:BANANAS"} {
		e, ok := c.Lookup(key)
		if !ok || e.Name != "Bananas" || e.Category != "Produce" {
			t.Errorf("Lookup(%s) = %+v, %v", key, e, ok)
		}
		if !c.Covers(key) {
			t.Errorf("Covers(%s) = false", key)
		}
	}
	if !c.Ignored("costco:23983") || !c.Covers("costco:23983") {
		t.Error("ignored item not ignored")
	}
	if _, ok := c.Lookup("costco:1"); ok {
		t.Error("unknown item found")
	}
}

func TestCatalogueRejectsDuplicatesAndBadYAML(t *testing.T) {
	dup := "products:\n  - name: A\n    match: [costco:1]\n  - name: B\n    match: [costco:1]\n"
	if _, err := ParseCatalogue([]byte(dup)); err == nil || !strings.Contains(err.Error(), "costco:1") {
		t.Errorf("duplicate not rejected: %v", err)
	}
	if _, err := ParseCatalogue([]byte("products:\n  - match: [costco:1]\n")); err == nil {
		t.Error("nameless product accepted")
	}
	if _, err := ParseCatalogue([]byte("products: [")); err == nil {
		t.Error("bad YAML accepted")
	}
}

func TestLoadCatalogueMissingFileIsEmpty(t *testing.T) {
	c, err := LoadCatalogue(filepath.Join(t.TempDir(), "none.yml"))
	if err != nil || len(c.Products) != 0 {
		t.Errorf("got %+v, %v", c, err)
	}
}

func TestMarshalCatalogueRoundTrips(t *testing.T) {
	data, err := MarshalCatalogue([]CatalogueEntry{StubEntry("Milk", "Dairy", "costco:457", "coop:HOMO MILK")})
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCatalogue(data)
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := c.Lookup("coop:HOMO MILK"); !ok || e.Name != "Milk" {
		t.Errorf("round trip lost entry: %s", data)
	}
}
