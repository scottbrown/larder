package larder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CatalogueEntry names a product and lists the store items that are it.
type CatalogueEntry struct {
	Name     string   `yaml:"name"`
	Category string   `yaml:"category,omitempty"`
	Match    []string `yaml:"match"`
}

// Catalogue is the hand-edited YAML file that gives products friendly names
// and categories, groups the same product across stores, and hides items
// that should not be analysed.
type Catalogue struct {
	Products []CatalogueEntry `yaml:"products"`
	Ignore   []string         `yaml:"ignore,omitempty"`
	// Private lists categories left out of public dashboards. When empty,
	// DefaultPrivateCategories applies.
	Private []string `yaml:"private,omitempty"`

	byKey  map[ProductKey]int
	ignore map[ProductKey]bool
}

// LoadCatalogue reads a catalogue file. A missing file yields an empty
// catalogue.
func LoadCatalogue(path string) (*Catalogue, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NewCatalogue(nil, nil)
	}
	if err != nil {
		return nil, err
	}
	return ParseCatalogue(data)
}

// ParseCatalogue decodes catalogue YAML.
func ParseCatalogue(data []byte) (*Catalogue, error) {
	var c Catalogue
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing catalogue: %w", err)
	}
	cat, err := NewCatalogue(c.Products, c.Ignore)
	if err != nil {
		return nil, err
	}
	cat.Private = c.Private
	return cat, nil
}

// DefaultPrivateCategories are kept out of public dashboards unless the
// catalogue says otherwise: what a household buys for its health, its
// bathroom cabinet and its wine rack is nobody else's business.
var DefaultPrivateCategories = []string{"Health", "Personal care", "Alcohol"}

// PrivateCategory reports whether products in a category are kept out of
// public dashboards.
func (c *Catalogue) PrivateCategory(category string) bool {
	private := c.Private
	if len(private) == 0 {
		private = DefaultPrivateCategories
	}
	for _, p := range private {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(category)) {
			return true
		}
	}
	return false
}

// NewCatalogue builds a catalogue, rejecting a store item claimed by two
// products.
func NewCatalogue(products []CatalogueEntry, ignore []string) (*Catalogue, error) {
	c := &Catalogue{
		Products: products,
		Ignore:   ignore,
		byKey:    map[ProductKey]int{},
		ignore:   map[ProductKey]bool{},
	}
	for i, p := range products {
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("catalogue product %d has no name", i+1)
		}
		for _, ref := range p.Match {
			key := parseRef(ref)
			if prev, dup := c.byKey[key]; dup {
				return nil, fmt.Errorf("%s is matched by both %q and %q", ref, products[prev].Name, p.Name)
			}
			c.byKey[key] = i
		}
	}
	for _, ref := range ignore {
		c.ignore[parseRef(ref)] = true
	}
	return c, nil
}

// parseRef turns "costco:30669" or "coop:Bananas" into a ProductKey.
func parseRef(ref string) ProductKey {
	store, item, _ := strings.Cut(strings.TrimSpace(ref), ":")
	store = strings.ToLower(strings.TrimSpace(store))
	item = strings.TrimSpace(item)
	if digitsOnly(item) == item {
		return ProductKey(store + ":" + strings.TrimLeft(item, "0"))
	}
	return ProductKey(store + ":" + normaliseDescription(item))
}

// Ignored reports whether a store item is excluded from analysis.
func (c *Catalogue) Ignored(key ProductKey) bool {
	return c.ignore[key]
}

// Lookup returns the catalogue entry covering a store item.
func (c *Catalogue) Lookup(key ProductKey) (CatalogueEntry, bool) {
	i, ok := c.byKey[key]
	if !ok {
		return CatalogueEntry{}, false
	}
	return c.Products[i], true
}

// Covers reports whether the catalogue names or ignores a store item.
func (c *Catalogue) Covers(key ProductKey) bool {
	_, named := c.byKey[key]
	return named || c.ignore[key]
}

// RefFor formats a product key as it would be written in the catalogue.
func RefFor(key ProductKey) string {
	return string(key)
}

// MarshalYAML-friendly stub for a product the catalogue does not yet cover.
func StubEntry(name, category string, keys ...ProductKey) CatalogueEntry {
	refs := make([]string, len(keys))
	for i, k := range keys {
		refs[i] = RefFor(k)
	}
	return CatalogueEntry{Name: name, Category: category, Match: refs}
}

// MarshalCatalogue encodes entries as catalogue YAML.
func MarshalCatalogue(entries []CatalogueEntry) ([]byte, error) {
	return yaml.Marshal(struct {
		Products []CatalogueEntry `yaml:"products"`
	}{entries})
}
